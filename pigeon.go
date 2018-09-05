package brisk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/coreos/etcd/clientv3"
)

func GetServiceAddress(serviceName string) ([]ServerInfo, error) {
	cli, etcdErr := clientv3.New(clientv3.Config{
		Endpoints:   []string{Etcd},
		DialTimeout: 5 * time.Second,
	})
	defer cli.Close()
	if etcdErr != nil {
		log.Fatal("Error: cannot connect to etcd ", etcdErr)
	}
	serPrefix := fmt.Sprintf("%s-%s-", "service", serviceName)
	resp, err := cli.Get(context.Background(), serPrefix, clientv3.WithPrefix())
	if err != nil {
		return []ServerInfo{}, err
	}
	var services []ServerInfo
	for _, kv := range resp.Kvs {
		var serverInfo ServerInfo
		err := json.Unmarshal(kv.Value, &serverInfo)
		if err != nil {
			log.Println("etcd registered format error", err)
			continue
		}
		services = append(services, serverInfo)
	}
	if len(services) == 0 {
		return []ServerInfo{}, errors.New("No service instance")
	}
	return services, nil
}
