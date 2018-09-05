package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/xid"

	"github.com/coreos/etcd/mvcc/mvccpb"

	"brisk"

	"brisk/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/labstack/echo"
)

const (
	// GET 请求类型为GET
	GET = "GET"
	// POST 请求类型为POST
	POST = "POST"
	// Prefix 固定的url请求前缀
	Prefix = "/api/v3/"
)

var (
	Etcd = os.Getenv("Etcd") // 部署使用
	// Etcd         = "t.epeijing.cn:2379" //本地使用
	cli, etcdErr = clientv3.New(clientv3.Config{
		Endpoints:   []string{Etcd},
		DialTimeout: 5 * time.Second,
	})
	remoteRequest  = utils.InitRequest()
	errorNoService = errors.New("no service")
	services       = NewServices()
)

// Services 服务实例操作管理实体
type Services struct {
	lock        sync.RWMutex
	idNameMap   map[string]string
	servicesMap map[string][]brisk.ServerInfo
}

// Init 对Services进行初始化赋值
func (s *Services) Init(cli *clientv3.Client) error {
	log.Println("gateway init")
	resp, err := cli.Get(context.Background(), "service-", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range resp.Kvs {
		var serverInfo brisk.ServerInfo
		err := json.Unmarshal(kv.Value, &serverInfo)
		if err != nil {
			log.Printf("etcd registered format error %v \n", err)
			continue
		}
		s.addServiceInstance(serverInfo)
	}
	log.Println("services init finished")
	return nil
}

// 为服务列表添加一个服务实例
func (s *Services) addServiceInstance(info brisk.ServerInfo) {
	serviceName := info.ServiceName
	s.lock.Lock()
	if s.servicesMap[serviceName] == nil {
		s.servicesMap[serviceName] = []brisk.ServerInfo{info}
	} else {
		s.servicesMap[serviceName] = append(s.servicesMap[serviceName], info)
	}
	s.idNameMap[info.ID] = serviceName
	s.lock.Unlock()
}

// 在服务列表中删除一个服务实例
func (s *Services) removeServiceInstance(serviceName, ID string) {
	instances := s.servicesMap[serviceName]
	for index, instance := range instances {
		if instance.ID == ID {
			s.lock.Lock()
			log.Println("REMOVE: remove serviceInstance from servicesMap")
			delete(s.idNameMap, ID)
			s.servicesMap[serviceName] = append(instances[:index], instances[index+1:]...)
			log.Println("REMOVE: serviceName:", serviceName, ", the latest serviceInstances:", s.servicesMap[serviceName])
			s.lock.Unlock()
			break
		}
	}
}

// NewServices new a Services
func NewServices() *Services {
	return &Services{
		lock:        sync.RWMutex{},
		idNameMap:   make(map[string]string),
		servicesMap: make(map[string][]brisk.ServerInfo),
	}
}

func main() {

	if etcdErr != nil {
		log.Panicf("connect to etcd error: %v", etcdErr)
	}
	//开局获得所有服务实例---存放于map结构
	serviceErr := services.Init(cli)
	if serviceErr != nil {
		log.Panicf("get all servicesInstace from etcd error: %v", etcdErr)
	}
	go func() {
		e := echo.New()
		e.Any(Prefix+":service/*", handleAll)
		e.Logger.Fatal(e.Start(":3030"))
	}()

	watcher := clientv3.NewWatcher(cli)
	w := watcher.Watch(context.Background(), "service-", clientv3.WithPrefix())
	for {
		select {
		case watchResponse := <-w:
			for _, event := range watchResponse.Events {
				// 接受心跳时，etcd注册中心是PUT操作，添加服务实例，且防止重复
				if event.Type == mvccpb.PUT {
					var server brisk.ServerInfo
					err := json.Unmarshal(event.Kv.Value, &server)
					if err != nil {
						log.Println("etcd registered format error", err)
					}
					if _, exist := services.idNameMap[server.ID]; !exist {
						//添加服务实例
						services.addServiceInstance(server)
						log.Println("ADD: add serviceInstance to servicesMap, service-info:", server)
					}
					continue
				}
				// 接受心跳时，etcd注册中心操作为DELETE，意为解除/取消注册
				if event.Type == mvccpb.DELETE {
					key := string(event.Kv.Key)
					nameID := strings.Split(key[len("service-"):], "-")
					log.Println("REMOVE: serviceName:", nameID[0], ",ID:", nameID[1], "should be removed")
					//删除服务实例
					services.removeServiceInstance(nameID[0], nameID[1])
				}
			}
		}
	}
}

func handleAll(c echo.Context) error {
	//解析服务
	url := c.Request().RequestURI
	serviceName := c.Param("service")
	serviceEndpoint, err := getService(serviceName)
	if err != nil {
		c.String(503, err.Error())
		return err
	}
	skip := len(Prefix) + len(serviceName)
	serviceFullURL := serviceEndpoint + url[skip:]
	log.Printf("serviceURL: %s \n", serviceFullURL)
	// if c.QueryString() != "" {
	// serviceFullURL += c.QueryString()
	// log.Printf("URL-Query！- %s", c.QueryString())
	// }
	// TODO
	if c.QueryParam("requestId") == "" {
		requestId := fmt.Sprintf("?requestId=%v", xid.New())
		serviceFullURL += requestId
	}

	req := c.Request()
	var proxyReq *http.Request
	var reqError error
	if req.Method == "GET" {
		if c.QueryParam("requestId") == "" {
			requestId := fmt.Sprintf("?requestId=%v", xid.New())
			serviceFullURL += requestId
		}
		proxyReq, reqError = http.NewRequest(req.Method, serviceFullURL, nil)
		if reqError != nil {
			log.Printf("can not make [GET] request, %v", reqError)
			return reqError
		}
	} else {
		defer req.Body.Close()
		body, error := ioutil.ReadAll(req.Body)
		if reqError != nil {
			return error
		}
		// 读取response里的requestId
		var mapBody map[string]interface{}
		json.Unmarshal(body, &mapBody)
		requestId := mapBody["requestId"]
		if requestId == "" {
			requestId = fmt.Sprintf("%v", xid.New())
			mapBody["requestId"] = requestId
		}
		body, _ = json.Marshal(mapBody)
		reqBody := bytes.NewReader(body)

		proxyReq, reqError = http.NewRequest(req.Method, serviceFullURL, reqBody)
		if reqError != nil {
			log.Printf("can not make request with body, %v", err)
			return reqError
		}
	}
	proxyReq.Header = req.Header
	log.Printf("proxying request to url: %s", serviceFullURL)
	resp, error := remoteRequest.GetClient().Do(proxyReq)
	if error != nil {
		return error
	}
	defer resp.Body.Close()
	_, err = io.Copy(c.Response(), resp.Body)
	return error
}

func getService(serviceName string) (string, error) {
	servicesInfos := services.servicesMap[serviceName]
	services.lock.RLock()
	total := len(servicesInfos)
	services.lock.RUnlock()
	if total == 0 {
		return "", errorNoService
	}
	if total == 1 {
		return "http://" + servicesInfos[0].Address, nil
	}
	log.Printf(" %d available ", total)
	rand.Seed(time.Now().UTC().UnixNano())
	randIndex := rand.Intn(total)
	log.Printf("use index: %d", randIndex)
	return "http://" + servicesInfos[randIndex].Address, nil
}
