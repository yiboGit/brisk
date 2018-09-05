package msa_rpc

import (
	"fmt"
	"os"
	"reflect"
)

// BuildClient 按照指定interface生成对应 client,方便服务间调用；
// serviceInterface interface{}指定接口，clientVar client端变量名，serviceName服务对外暴露时使用的服务名，filePath string 生成文件所在路径：eg：. 表示当前文件夹路径
// * PS：serviceInterface 的参数格式，例如服务接口为UserServer 那么给定的参数格式应该为：(*UserServer)(nil)
func BuildClient(serviceInterface interface{}, clientVar, serviceName, filePath string) {
	t := reflect.TypeOf(serviceInterface).Elem()
	clientType := fmt.Sprintf("%s%s", t.Name(), "Client")
	var methods string
	for i := 0; i < t.NumMethod(); i++ {
		methodName := t.Method(i).Name
		respType := t.Method(i).Type.Out(0).Name()
		var paramStr string
		if t.Method(i).Type.NumIn() == 0 {
			paramStr = `
			func (%s *%s)%s() %s {
				var p msa.EmptyParam
			`
			paramStr = fmt.Sprintf(paramStr, clientVar, clientType, methodName, respType)
		} else {
			paramType := t.Method(i).Type.In(0).Name()
			paramStr = `
			func (%s *%s)%s(p %s) %s {
			`
			paramStr = fmt.Sprintf(paramStr, clientVar, clientType, methodName, paramType, respType)
		}
		//err := %s.request(p, "%s", &result)
		methodStr := `%s
			var result %s

			if p.requestId == "" {
				p.requestId = fmt.Sprintf("%v", xid.New())
			}

			err := %s.request(p, "%s", &result) 
			if err != nil {
				result.Error = err.Error()
				return result
			}
			return result
		}
		`
		methods = methods + fmt.Sprintf(methodStr, paramStr, respType, clientVar, methodName, respType)
	}
	//	requestMethod := `func (%s *%s)request(param interface{}, method string, respResult interface{}) (error){
	requestMethod := `func (%s *%s)request(param interface{}, method string, respResult interface{}) (error){
		n := len(%s.serviceInstances)
		if n == 0 {
			return errors.New("service instance is empty")
		}
		var b bytes.Buffer
		u, _ := json.Marshal(param)
		b.Write(u)
		//TODO  这里做选择 例如负载均衡 暂时随机数
		r := rand.New(rand.NewSource(time.Now().Unix()))
		i := r.Intn(n)
		address := %s.serviceInstances[i].Address
		url := "http://" + address + "/" + "%s" + "/" + method
		resp, err := http.Post(url , "application/json", &b)
		if err != nil {
			return err
		}
		b.ReadFrom(resp.Body)
		json.Unmarshal(b.Bytes(), respResult)
		return nil
	}
		`
	// url := "http://" + address + "/" + "%s" + "/" + method
	requestMethod = fmt.Sprintf(requestMethod, clientVar, clientType, clientVar, clientVar, t.Name())

	watchSerIns := `go func(){
		// 监听etcd中服务实例变化
		cli, etcdErr := clientv3.New(clientv3.Config{
			Endpoints:   []string{"t.epeijing.cn:2379"},
			DialTimeout: 5 * time.Second,
		})
		if etcdErr != nil {
			%s
		}
		watcher := clientv3.NewWatcher(cli)
		w := watcher.Watch(context.Background(), "service-%s-", clientv3.WithPrefix())
		for {
			select {
			case watchResp := <-w:
				for _, event := range watchResp.Events {
					if event.Type == mvccpb.PUT {
						var server brisk.ServerInfo
						err := json.Unmarshal(event.Kv.Value, &server)
						if err != nil {
							log.Println("etcd registered format error", err)
						}
						%s.lock.Lock()
						exist, _  := %s.isExist(server.ID)
						if !exist {
							%s.serviceInstances = append(%s.serviceInstances, server)
						}
						%s.lock.Unlock()
					} else if event.Type == mvccpb.DELETE {
						key := string(event.Kv.Key)
						nameID := strings.Split(key[len("service-"):], "-")
						%s.lock.Lock()
						exist, index := %s.isExist(nameID[1])
						if exist {
							%s.serviceInstances = append(%s.serviceInstances[:index], %s.serviceInstances[index+1:]...)
						}
						%s.lock.Unlock()
					}
				}
			}
		}
		}()
	`
	watchSerIns = fmt.Sprintf(watchSerIns, `log.Panicf("connect to etcd error: %v", etcdErr)`, serviceName, clientVar, clientVar, clientVar, clientVar, clientVar, clientVar, clientVar, clientVar, clientVar, clientVar, clientVar)

	clientInit := `
	func (%s *%s)Init() error{
	  serviceInstances, err := brisk.GetServiceAddress("%s")
	  if err != nil {
		  return err
	  }
	  %s.serviceInstances = serviceInstances
	  %s
	  return nil
	}
	`
	clientInit = fmt.Sprintf(clientInit, clientVar, clientType, serviceName, clientVar, watchSerIns)

	removeSerIns := `func (%s *%s) isExist(ID string) (bool,int) {
		exist := false
		var i int
		for index, item := range %s.serviceInstances {
			if item.ID == ID {
				exist = true
				i = index
				break
			}
		}
		return exist, i
	}
	`
	removeSerIns = fmt.Sprintf(removeSerIns, clientVar, clientType, clientVar)

	title := `package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	brisk "eglass.com/brisk"
	msa "eglass.com/brisk/msa_rpc"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type %s struct {
	lock             sync.RWMutex
	serviceInstances []brisk.ServerInfo
}

var (
	%s = %s{
		lock:             sync.RWMutex{},
		serviceInstances: []brisk.ServerInfo{},
	}
)

%s
%s
%s
%s
	`
	flieName := fmt.Sprintf("%s/%s_client.go", filePath, t.Name())
	f, _ := os.Create(flieName)
	defer f.Close()
	info := fmt.Sprintf(title, clientType, clientVar, clientType, clientInit, removeSerIns, methods, requestMethod)
	f.Write([]byte(info))
}

// 使用方法：demo
// 需要main方法 在方法中给出参数 执行生成
// eg:
// package main
// import brisk "eglass.com/brisk/msa_rpc"
// func main() {
// 	brisk.BuildService((*UserServer)(nil), "instance", "UserInstace", ".")
// 	brisk.BuildClient((*UserServer)(nil), ".")
// }
