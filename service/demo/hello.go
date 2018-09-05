package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"eglass.com/brisk"
)

var (
	// Port 端口
	Port = os.Getenv("Port")
	// Host 域名
	Host = os.Getenv("Host")
	// ContainerPort os获取配置文件上容器端口
	ContainerPort = os.Getenv("ContainerPort")
)

func main() {
	server := ServerInstance{}
	brisk.HandleServiceLifeCycle(server)
}

// ServerInstance 实现接口
type ServerInstance struct {
	err error
}

// Server 实现接口
func (s ServerInstance) Service() error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s:%s-%s", Host, Port, "Hello update")
		fmt.Fprintf(w, "Hello update")
	})
	return http.ListenAndServe(fmt.Sprintf(":%s", ContainerPort), nil)
}
