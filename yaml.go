package brisk

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

// ServConfigs 服务配置
type ServConfigs struct {
	ServiceName string `yaml: servicename`
	Replica     int    `yaml: replica` //服务节点个数，目前最多2个
	Meta        Meta   `yaml: meta`
}

type Meta struct {
	Port          string `yaml: port`
	ContainerPort string `yaml: containerport`
	NeedNetPublic bool   `yaml: needNetpublic`
	ImagePrefix   string `yaml: imageprefix`
	Etcd          string `yaml: etcd`
}

// AllServConfigs 所有的服务配置，key 为服务名
type AllServConfigs map[string]ServConfigs

// NodeConfig Node配置
type NodeConfig struct {
	HostName      string `yaml: hostname`      //节点名称(hostname)
	HasPublic     bool   `yaml: haspublic`     //是否有公有IP
	PrivateIP     string `yaml: privateip`     // 私有IP
	PublicIP      string `yaml: publicip`      // 公有IP
	MaxContainers int    `yaml: maxcontainers` //节点最大容器数量
}

// NodeConfigs 所有Node配置，key 节点名称
type NodeConfigs map[string]NodeConfig

// ReadYamlFile 从yaml文件中获取数据 （通用）
func ReadYamlFile(filePath string, out interface{}) {
	// 传入地址
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error : read yaml file error, Err: %v \n", err)
	}
	log.Println("Successful : read yaml file successful")
	err = yaml.Unmarshal(yamlFile, out)
	if err != nil {
		log.Fatalf("Error :yamlFile data Unmarshal error, Err: %v \n", err)
	}
	log.Printf("Successful: yamlFile data Unmarshal successful, Data :%v", out)
}

// MailAddressees 邮件收信人  key: 收件人姓名， value: 收件人地址
type MailAddressees map[string]string

// MailMessage 邮件信息  key: 信息针对服务 value: 邮局具体信息
type MailMessage map[string]string
