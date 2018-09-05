### brisk部分常用数据结构：

#### 服务详细信息：
        ServerInfo: 
            ID            string        服务ID
            IP            string        对外连接服务的 IP
            Port          string        宿主机(服务器)，对外服务端口，本机或者端口映射后得到的
            ContainerPort string        容器暴露端口对外服务端口，本机或者端口映射后得到的
            Host          string        服务副本部署在的服务器HostName
            Address       string        服务地址 host:端口
            ServiceName   string        服务名称

#### docker镜像信息：
        DockerImage:
            ID         string               镜像唯一性ID
            FullName   string               镜像完整名称，包含前缀以及版本号
            Env        map[string]string    镜像启动所使用环境变量
            Node       string               指定所在服务器节点的HostName
            CreateTime time.Time            镜像创建时间

#### 节点上运行的镜像信息：
        NodeImage:
            ImageInfoKey string             镜像详细记录，保存在etcd中的key值，保存在此处，作为未来更新ImageInfo记录使用
            FullName     string             镜像全名
            Env          map[string]string  使用的环境变量，例如：IP,Port,ContainerPort,Host,Etcd,ServiceName
            Node         string             指定服务器节点HostName
            ContainerID  string             容器ID

#### 服务配置文件信息：
        ServConfigs:
            ServiceName string  服务名
            Replica     int     服务副本个数，依据具体情况而定
            Meta        Meta    服务详细数据信息
##### 服务详细数据信息：
            Meta:
                Port          string    服务器端口
                ContainerPort string    容器端口
                NeedNetPublic bool      服务是否需要公网
                ImagePrefix   string    服务镜像前缀
                Etcd          string    Etcd注册中心的地址

#### Node服务器节点配置信息：
        NodeConfig: 
            HostName      string    服务器节点的HostName
            HasPublic     bool      服务器是否有公有IP
            PrivateIP     string    服务器节点私有IP
            PublicIP      string    服务器节点公有IP
            MaxContainers int       服务器节点最大容器数量
    

### Etcd保存信息，前缀规则：

#### service注册与发现部分：
        服务注册：
            service-"ServiceName"-"xID"
                ServiceName: 当前服务名称
                xID: 标志唯一性的ID
        
        服务销毁，停止，删除信号：
            RM-"Host"-"xID"
                Host: 当前服务器节点的HostName
                xID: 标志唯一性的ID
                PS: 词条信息使用完之后，Etcd会立即删除

#### keeper部分:    
        Keeper正常运行信息:
            running-keeper-"HostName"
                HostName: 当前服务器节点的HostName

        keeper所在服务器当前启动/管理的正常运行的服务:
            keeper-"HostName"-image
                HostName: 当前服务器节点的HostName

        keeper关于当前服务副本启动的反馈信息，滚动升级专用:
            rolling-update-"ServiceName"
                ServiceName: 服务的名称
    
#### center部分:
        center保存，整理好的镜像信息：
            docker-image-"xID"
                xID: 标志唯一性的ID


### msa-rpc的使用介绍：
对外提供两个方法：    
 *BuildService()* 自动生成服务端所必须的路由规则，封装入bind方法中，使用时调用bind方法，进行绑定.  
* 所需要的参数：    
- [x] serviceInstance interface{} 指定接口,参数格式：服务接口为UserServer 那么给定的参数格式应该为：(*UserServer)(nil)   
- [x] structName实现接口结构体名称  
- [x] structVar实现接口结构体的变量名  
- [x] 生成文件所在路径：eg: . 表示当前文件夹路径  
 
*BuildClient()* 自动生成服务间通信所需要的被调用服务的client端，生成之后，只需要调用client端内的方法，即可完成服务间通信。
* 所需要的参数：
- [x] serviceInterface interface{}指定接口，参数格式：服务接口为UserServer 那么给定的参数格式应该为：(*UserServer)(nil)
- [x] clientVar client端结构体变量名
- [x] serviceName服务对外暴露时使用的服务名
- [x] filePath string 生成文件所在路径：eg：. 表示当前文件夹路径


        

        
    

