package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"eglass.com/brisk"
	"github.com/coreos/etcd/clientv3"
	"github.com/gorilla/websocket"
)

const (
	urlPrefix = "/api/register/"
	// 操作类型是put
	PUT = "put"
	// 操作类型是delete
	DEL = "delete"
)

var (
	Etcd = os.Getenv("Etcd")
	// Etcd         = "t.epeijing.cn:2379"
	cli, etcdErr = clientv3.New(clientv3.Config{
		Endpoints:   []string{Etcd},
		DialTimeout: 5 * time.Second,
	})
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	if etcdErr != nil {
		log.Fatal("Error(node_access): cannot connect to etcd ", etcdErr)
	}
	log.Println("Successful(node_access): connect to etcd")

	http.HandleFunc(urlPrefix+"delete/", handleDEL)
	http.HandleFunc(urlPrefix+"put/", socketPUT)

	err := http.ListenAndServe(":9999", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func socketPUT(w http.ResponseWriter, r *http.Request) {
	// 解析服务 PUT 操作使用websocket
	log.Println("stock start")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	url := html.EscapeString(r.URL.Path)
	log.Println("stock url")
	key := url[len(urlPrefix+"put/"):]
	log.Printf("Info(node_access): action: put, key: %s \n", key)
	dealPutMsg(conn, key)

}

func dealPutMsg(conn *websocket.Conn, key string) {
	defer conn.Close()
	for {
		var serverInfo brisk.ServerInfo
		err := conn.ReadJSON(&serverInfo)
		if err != nil {
			log.Printf("Error(node_access): Register PUT message err , Error: %v \n", err)
			return
		}
		log.Printf("Info(node_access): value %v \n", serverInfo)
		value, err := json.Marshal(serverInfo)
		if err != nil {
			log.Printf("Error(node_access): Service Info has Error, %v \n", err)
			return
		}
		err = serverRegister(cli, key, value)
		if err != nil {
			log.Printf("%v \n", err)
			return
		}
	}
}

func handleDEL(w http.ResponseWriter, r *http.Request) {
	// 解析服务 DEL 操作仍然使用http 请求
	log.Println("delete start")
	url := html.EscapeString(r.URL.Path)
	key := url[len(urlPrefix+"delete/"):]
	log.Printf("Info(node_access): action: delete, key: %s \n", key)
	var rmMsg map[string]string
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errorMsg := fmt.Sprintf("RM-Error(node_access):request read body error,  %v \n", err)
		log.Printf(errorMsg)
		http.Error(w, errorMsg, 500)
		return
	}
	log.Printf("rmMsg: %v \n", data)
	json.Unmarshal(data, &rmMsg)
	log.Printf("delete message : %v \n", rmMsg)
	value, err := json.Marshal(rmMsg)
	if err != nil {
		errorMsg := fmt.Sprintf("RM-Error(node_access):RM value Marshal error %v \n", err)
		http.Error(w, errorMsg, 500)
		return
	}
	log.Printf("delete put value \n")
	_, err = cli.Put(context.Background(), key, string(value))
	if err != nil {
		errorMsg := fmt.Sprintf("RM-Error(node_access): put remove info error, %v", err)
		http.Error(w, errorMsg, 500)
		return
	}
	log.Printf("RM-info(node_access): put remove info successfully \n")
	array := strings.Split(key, "-")
	if len(array) == 0 {
		err := errors.New("key length error")
		errorMsg := fmt.Sprintf("RM-Error(node_access): RM-key message error, %v", err)
		http.Error(w, errorMsg, 500)
		return
	}

	serviceKey := fmt.Sprintf("service-%s-%s", rmMsg["serviceName"], array[2])
	cli.Delete(context.Background(), serviceKey)
	log.Println("service unregister finish (node_access)")

}

func serverRegister(cli *clientv3.Client, key string, value []byte) error {
	lease, err := cli.Grant(context.Background(), 10)
	if err != nil {
		msg := fmt.Sprintf("Error(node_access): etcd create lease has error, %v ", err)
		return errors.New(msg)
	}
	_, err = cli.Put(context.Background(), key, string(value), clientv3.WithLease(lease.ID))
	if err != nil {
		msg := fmt.Sprintf("Error(node_access): service info register error, %v", err)
		return errors.New(msg)
	}
	log.Println("put ok")
	return nil
}
