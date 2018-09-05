package brisk

import (
	"time"
)

// DockerImage docker镜像信息
type DockerImage struct {
	ID         string            `json:"id"`       // 镜像ID
	FullName   string            `json:name`       // 镜像完整名称FullName
	Env        map[string]string `json:env`        // 环境变量
	Node       string            `json:name`       // 指定节点Node
	CreateTime time.Time         `json:createTime` //镜像创建时间
}
