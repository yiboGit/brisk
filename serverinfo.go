package brisk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/rs/xid"
)

// ServerInfo os取到的当前信息
var (
	// IP            = os.Getenv("IP")
	// Port          = os.Getenv("Port")          // Port os获取配置文件上宿主机端口
	// ContainerPort = os.Getenv("ContainerPort") // ContainerPort os获取配置文件上容器端口
	// Host          = os.Getenv("Host")
	// Etcd          = os.Getenv("Etcd") //"t.epeijing.cn:2379"
	// ServiceName   = os.Getenv("ServiceName")
	// 测试使用
	IP            = "127.0.0.1"
	Port          = "8999" // Port os获取配置文件上宿主机端口
	ContainerPort = "8999" // ContainerPort os获取配置文件上容器端口
	Host          = "localhost"
	Etcd          = "t.epeijing.cn:2379"
	ServiceName   = "panel_service"
	HostName      = GetHostname() // docker运行镜像时，这里取得的HostName，为容器的ID
)

// ServerInfo 服务注册信息
type ServerInfo struct {
	ID            string `json:"id"`            // 服务ID
	IP            string `json:"ip"`            // 对外连接服务的 IP
	Port          string `json:"port"`          // 宿主机，对外服务端口，本机或者端口映射后得到的
	ContainerPort string `json:"ContainerPort"` // 容器暴露端口对外服务端口，本机或者端口映射后得到的
	Host          string `json:"host"`
	Address       string `json:"address"`
	ServiceName   string `json:"serviceName"`
}

type Server interface {
	Service() error
}

func HandleServiceLifeCycle(server Server) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		log.Fatal(server.Service())
	}()

	go func() {
		defer wg.Done()
		time.Sleep(time.Second)
		cli, etcdErr := clientv3.New(clientv3.Config{
			Endpoints:   []string{Etcd},
			DialTimeout: 5 * time.Second,
		})
		if etcdErr != nil {
			log.Fatal("Error: cannot connect to etcd ", etcdErr)
		}
		log.Println("Successful: connect to etcd")
		guid := xid.New()
		xID := fmt.Sprintf("%s", guid)
		key := fmt.Sprintf("%s-%s-%s", "service", ServiceName, xID)
		address := Host + ":" + Port
		serviceInfo := ServerInfo{
			ID:            xID,
			IP:            IP,
			Port:          Port,
			ContainerPort: ContainerPort,
			Host:          Host,
			Address:       address,
			ServiceName:   ServiceName,
		}
		log.Printf("key : %s ; value : %v", key, serviceInfo)
		value, err := json.Marshal(serviceInfo)
		if err != nil {
			log.Fatal("Error: Service Info has Error", err)
		}
		//首次注册服务
		serverRegister(cli, key, value)
		log.Println("Successful:service register ")
		//设置心跳时间，准备注册服务
		ticker := time.NewTicker(10 * time.Second)
		c := make(chan os.Signal, 1)
		//监测os的三种关于退出，销毁的信号
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
		for {
			select {
			case <-ticker.C:
				//定时注册,创建10s的租期，意为每10s发送一次心跳
				serverRegister(cli, key, value)
			case <-c:
				//容器停止记录，删除记录 镜像的名称也要有统一的规范
				rmKey := fmt.Sprintf("%s-%s-%s", "RM", Host, fmt.Sprintf("%s", xid.New()))
				valueMap := map[string]string{
					"serviceName": ServiceName,
					"containerId": HostName,
				}
				value, err := json.Marshal(valueMap)
				if err != nil {
					log.Printf("RM-Error: RM-%s value Marshal error %v \n", Host, err)
				}
				_, err = cli.Put(context.Background(), rmKey, string(value))
				if err != nil {
					log.Printf("RM-Error: put remove info error, %v", err)
				}
				log.Printf("RM-info: put remove info successfully\n")
				//退出时（销毁时），自动解除注册
				cli.Delete(context.Background(), key)
				log.Println("service unregister finish")
				return
			}
		}
	}()
	wg.Wait()
}

func serverRegister(cli *clientv3.Client, key string, value []byte) {
	lease, err := cli.Grant(context.Background(), 10)
	if err != nil {
		log.Fatal("Error: etcd create lease has error", err)
	}
	_, err = cli.Put(context.Background(), key, string(value), clientv3.WithLease(lease.ID))
	if err != nil {
		log.Fatal("Error: service info register error", err)
	}
}
