package brisk

import (
	"errors"
	"log"
	"os/exec"
	"strings"
	"time"
)

// ImageInfo 镜像整理之后的详细信息，作为详细记录使用
type ImageInfo struct {
	Name        string            `json:"name"`      // 镜像名
	FullName    string            `json:fullName`    // 镜像全名
	Env         map[string]string `json:env`         // 环境变量
	Version     string            `json:version`     // 版本号
	Node        string            `json:name`        // 指定节点Node
	ContainerID string            `json:containerID` // 容器ID
	CreateTime  time.Time         `json:createTime`  // 启动时间
}

// NodeImage 节点上运行的镜像信息， 以供正常使用
type NodeImage struct {
	ImageInfoKey string            `json:key`         // 镜像详细记录，保存在etcd中的key值，保存在此处，作为未来更新ImageInfo记录使用
	FullName     string            `json:fullName`    // 镜像全名
	Env          map[string]string `json:env`         // 环境变量
	Node         string            `json:name`        // 指定节点Node
	ContainerID  string            `json:containerID` // 容器ID
}

// NodeImages keeper启动成功/失败的镜像（复数）信息
type NodeImages map[string]NodeImage

// NewNodeImages New
func NewNodeImages() NodeImages {
	return make(map[string]NodeImage)
}

func SplitFullName(fullName string) (string, string, error) {
	str := strings.Split(fullName, "/")
	if len(str) < 2 {
		log.Println("dockerImage name has error")
		return "", "", errors.New("dockerImage name has error")
	}
	infos := strings.Split(str[1], ":")
	if len(infos) < 2 {
		log.Println("dockerImage name has error")
		return "", "", errors.New("dockerImage name has error")
	}
	return infos[0], infos[1], nil
}

func GetHostname() string {
	hostnameCmd := exec.Command("sh", "-c", "hostname")
	stdout, err := hostnameCmd.CombinedOutput()
	strStdout := string(stdout)
	if err != nil {
		log.Printf("get node hostname has error, %v, info: %s", err, strStdout)
	}
	return strings.TrimSpace(strStdout)
}

// Converter DockerImage ==> imageInfo
func (i *ImageInfo) Converter(d DockerImage) (err error) {
	i.Node = d.Node
	i.FullName = d.FullName
	i.Env = d.Env
	i.Name, i.Version, err = SplitFullName(i.FullName)
	if err != nil {
		return err
	}
	return nil
}
