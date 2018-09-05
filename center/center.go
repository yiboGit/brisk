package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"brisk"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/rs/xid"
)

var (
	// Etcd 通过配置文件获取Etcd的地址
	Etcd         = os.Getenv("Etcd") // Etcd = "t.epeijing.cn:2379"
	cli, etcdErr = clientv3.New(clientv3.Config{
		Endpoints:   []string{Etcd},
		DialTimeout: 5 * time.Second,
	})
	scheduler = NewScheduler()
)

type ServiceImageEvent struct {
	ServiceName string    `json:"service_name"`
	CommitHash  string    `json:"commit_hash"`
	CreateTime  time.Time `json:"create_time"`
}

type Scheduler struct {
	// 所有服务的配置信息
	ServiceMetas brisk.AllServConfigs
	// 所有服务器节点的配置信息
	NodeMetas        brisk.NodeConfigs
	HTTPServer       *echo.Echo
	ImageEventChan   chan (ServiceImageEvent)
	RollingServices  map[string]bool
	RollingServQueue map[string][]ServiceImageEvent
	// 添加 收件人
	MailAddressees brisk.MailAddressees
	// 整段信息 保存在 s中 按照服务名 分类保存
	MailMessage brisk.MailMessage
}

// NewScheduler new scheduler
func NewScheduler() *Scheduler {
	var (
		serviceMetas   brisk.AllServConfigs
		nodeMetas      brisk.NodeConfigs
		mailAddressees brisk.MailAddressees
	)
	//  /Users/liamy/go/src/eglass.com/brisk/center 本地路径
	//  /etc/center-yaml node2 服务器了路径
	brisk.ReadYamlFile("/etc/center-yaml/ServConfigs.yaml", &serviceMetas)
	brisk.ReadYamlFile("/etc/center-yaml/NodeConfigs.yaml", &nodeMetas)
	brisk.ReadYamlFile("/etc/center-yaml/MailAddressee.yaml", &mailAddressees)

	return &Scheduler{
		ServiceMetas:     serviceMetas,
		NodeMetas:        nodeMetas,
		MailAddressees:   mailAddressees,
		MailMessage:      make(map[string]string),
		ImageEventChan:   make(chan (ServiceImageEvent), 100),
		RollingServices:  make(map[string]bool),
		RollingServQueue: make(map[string][]ServiceImageEvent),
		HTTPServer: func() *echo.Echo {
			e := echo.New()
			e.Use(middleware.Logger())
			e.Use(middleware.Recover())
			return e
		}(),
	}
}

// Init 初始化
func (s *Scheduler) Init() {
	s.HTTPServer.POST("/api/brisk", func(c echo.Context) error {
		var event ServiceImageEvent
		c.Bind(&event)
		s.ImageEventChan <- event
		c.String(200, "accepted")
		return nil
	})
}

// Run 启动
func (s *Scheduler) Run() {
	log.Printf("Info: Scheduler Run \n")
	go func() {
		s.HTTPServer.Logger.Fatal(s.HTTPServer.Start(":20000"))
	}()
	// 周期性 检查服务器节点 启动，运行服务的情况
	checkTicker := time.NewTicker(2 * time.Minute)
	// 周期性 检查服务注册情况
	checkRegisterTicker := time.NewTicker(3 * time.Minute)
	for {
		select {
		case e := <-s.ImageEventChan:
			s.handleNewServiceImage(e)
		case <-checkTicker.C:
			s.CheckAllServiceDeployed()
		case <-checkRegisterTicker.C:
			s.CheckServiceRegister()
		}
	}
}

// 放入以服务名分类的缓存队列
func (s *Scheduler) imageEventQueue(e ServiceImageEvent) {
	log.Printf("Info: add serviceImageEvent to the queue , serviceImageEvent: %v \n", e)
	serviceName := e.ServiceName
	if _, ok := s.RollingServQueue[serviceName]; ok {
		s.RollingServQueue[serviceName] = append(s.RollingServQueue[serviceName], e)
	} else {
		s.RollingServQueue[serviceName] = []ServiceImageEvent{e}
	}
	log.Printf("Info: service name: %s , queue: %v \n", serviceName, s.RollingServQueue[serviceName])
}

// 构建新的镜像信息，来 rolling-update
func (s *Scheduler) handleNewServiceImage(e ServiceImageEvent) {
	// 保证每个服务下只有一个正在升级
	if s.RollingServices[e.ServiceName] {
		s.imageEventQueue(e)
		return
	}
	go func() {
		log.Printf("Info : serviceName : %s, rolling-update now : %v \n", e.ServiceName, s.RollingServices[e.ServiceName])
		//设置此服务在滚动升级
		serviceName := e.ServiceName
		commitHash := e.CommitHash
		createTime := e.CreateTime
		log.Printf("Info: rolling-update start; service info : serviceName: [%s], commitHash: %s, createTime: %v \n", serviceName, commitHash, createTime)
		s.RollingServices[serviceName] = true
		// design -- images
		dockerImages := s.designImage(serviceName, commitHash, createTime)
		log.Printf("Info: rolling-update : dockerImages : %v \n", dockerImages)
		if len(dockerImages) == 0 {
			log.Println("Error : design Image error, dockerImages is empty")
			s.RollingServices[serviceName] = false
			return
		}
		// dChan DockerImage-chan
		dChan := make(chan brisk.DockerImage, 1)
		// 向通道中添加第一个镜像,index从0开始
		index := 0
		dChan <- dockerImages[index]
		// watcher
		watcher := clientv3.NewWatcher(cli)
		// 监听keeper 对于镜像启动情况的反馈
		w := watcher.Watch(context.Background(), "rolling-update-"+serviceName)
		for {
			select {
			case watchResponse := <-w:
				// 得到镜像 启动运行信息
				for _, event := range watchResponse.Events {
					if event.Type == mvccpb.PUT {
						execResult, err := strconv.ParseBool(string(event.Kv.Value))
						if err != nil {
							msg := fmt.Sprintf("Rolling-Error : rolling-update-%s result, string ==> bool error, err : %v \n", serviceName, err)
							s.writeMail(serviceName, msg)
							log.Printf(msg) //completed
							goto COMPLETED
						}
						msg := fmt.Sprintf("Rolling-Info : service : %s ,commitHash: %s, rolling-update feedback result : %v \n", serviceName, commitHash, execResult)
						s.writeMail(serviceName, msg)
						log.Printf(msg)
						if execResult {
							if len(dockerImages) == index {
								msg := fmt.Sprintf("Rolling-Serv-Successful: service: %s, commitHash: %s, replicas run successfully, node: %s \n", serviceName, commitHash, dockerImages[index-1].Node)
								log.Printf(msg)
								s.writeMail(serviceName, msg)
								msg = fmt.Sprintf("Rolling-AllServ-Successful: service: %s, all service replicas run successfully \n", serviceName)
								s.writeMail(serviceName, msg)
								log.Printf(msg)
								goto COMPLETED
							}
							msg := fmt.Sprintf("Rolling-Serv-Successful: service: %s, commitHash: %s, replicas run successfully, node: %s \n", serviceName, commitHash, dockerImages[index-1].Node)
							log.Printf(msg)
							s.writeMail(serviceName, msg)
							dChan <- dockerImages[index]
						} else {
							msg := fmt.Sprintf("Rolling-Serv-Fail: service: %s , commitHash: %s, replicas run failed, node: %s \n", serviceName, commitHash, dockerImages[index-1].Node)
							log.Printf(msg)
							s.writeMail(serviceName, msg)
							goto COMPLETED
						}
					}
				}
			case d := <-dChan:
				msg := fmt.Sprintf("Rolling-Info: put dockerImage to etcd, dockerImage-Info: %v \n", d)
				log.Printf(msg)
				s.writeMail(serviceName, msg)
				err := putDockerImage(d)
				if err != nil {
					msg := fmt.Sprintf("Rolling-Error : Put dockerImage error , dockerImage-fullName: %s, err : %v \n", d.FullName, err)
					log.Printf(msg)
					s.writeMail(serviceName, msg)
					goto COMPLETED
				}
				//index自增
				index++
			case <-time.After(2 * time.Minute):
				// 超时处理
				msg := fmt.Sprintf("Rolling-TimeOut : service: %s, rolling update timeout \n", serviceName)
				log.Printf(msg)
				s.writeMail(serviceName, msg)
				goto COMPLETED
			}
		}
	COMPLETED:
		// 完成升级之后删除 升级信息
		msg := fmt.Sprintf("Rolling-Completed: service %s : rolling-update Completed \n", serviceName)
		log.Printf(msg)
		s.writeMail(serviceName, msg)
		// 发送邮件
		// subject 邮件主题
		subject := fmt.Sprintf("%s,service-name: %s", "Rolling-Update-Info", serviceName)
		s.sendEMail(serviceName, subject)
		// 清空已发送邮件的历史信息
		s.writeMail(serviceName, "")
		// 删除镜像启动的反馈信息
		cli.Delete(context.Background(), "rolling-update-"+serviceName)
		// 从缓存中取得 当前服务名下的镜像队列
		queue, exist := s.RollingServQueue[serviceName]
		// 说明rolling-update 结束
		s.RollingServices[serviceName] = false
		// 关闭 channel
		close(dChan)
		if exist && len(queue) != 0 {
			servImageEvent := queue[0]
			s.RollingServQueue[serviceName] = queue[1:]
			log.Printf("Rolling-Completed: service %s : rolling-update next serviceImageEvent \n", serviceName)
			s.ImageEventChan <- servImageEvent
		}
	}()
}

// 将整理完成的 dockerImage PUT到etcd中心，以供keeper启动镜像使用
func putDockerImage(dockerImage brisk.DockerImage) error {
	//"docker-image"
	log.Println("Info : Put dockerImage to etcd")
	key := fmt.Sprintf("%s-%s", "docker-image", fmt.Sprintf("%s", xid.New()))
	value, err := json.Marshal(dockerImage)
	if err != nil {
		log.Printf("Error : dockerImage marshal error , err : %v \n", err)
		return err
	}
	_, err = cli.Put(context.Background(), key, string(value))
	if err != nil {
		log.Printf("Error : Put dockerImage to etcd  error , err : %v \n", err)
		return err
	}
	return nil
}

// ServConvImage 服务配置信息 转 docker镜像信息
func (s *Scheduler) designImage(serviceName string, commitHash string, createTime time.Time) []brisk.DockerImage {
	log.Printf("Info: start design image, serviceName：%s, commitHash: %s, createTime: %v \n", serviceName, commitHash, createTime)
	// 镜像列表
	var dockerImages []brisk.DockerImage
	// 取得服务配置
	servConfig := s.ServiceMetas[serviceName]
	log.Printf("Info: 获取服务器列表：%v \n", servConfig)
	// 获取服务器列表
	nodeConfigs := s.getServerNode(servConfig.Meta.NeedNetPublic)
	log.Printf("Info: 获取服务器列表：%v \n", nodeConfigs)
	// 取得镜像全名
	fullName := fmt.Sprintf("%s:%s", servConfig.Meta.ImagePrefix, commitHash)
	log.Printf("Info: fullName %s \n", fullName)
	for i := 0; i < servConfig.Replica; i++ {
		// 目前只能是把有公网的服务器 放在第一位  逻辑存在一些问题
		// qualified := !servConfig.Meta.NeedNetPublic || nodeConfigs[i].HasPublic
		// if !qualified {
		// 	continue
		// }
		dockerImage := brisk.DockerImage{
			ID:       fmt.Sprintf("%s", xid.New()),
			FullName: fullName,
			Env: map[string]string{
				"IP":            nodeConfigs[i].PrivateIP,
				"Port":          servConfig.Meta.Port,
				"ContainerPort": servConfig.Meta.ContainerPort,
				"Host":          nodeConfigs[i].HostName,
				"Etcd":          servConfig.Meta.Etcd,
				"ServiceName":   servConfig.ServiceName,
			},
			Node:       nodeConfigs[i].HostName,
			CreateTime: createTime,
		}
		dockerImages = append(dockerImages, dockerImage)
	}
	log.Printf("Info: image design finished, serviceName：%s \n", serviceName)
	log.Printf("Info: images：%v \n", dockerImages)
	return dockerImages
}

// 拓展，获取服务器node列表
func (s *Scheduler) getServerNode(hasPubNet bool) map[int]brisk.NodeConfig {
	// 目前这段逻辑 基于的是 服务器数量与副本数量 一定是对等的情况
	nodeMetas := make(map[int]brisk.NodeConfig)
	i := 0
	for _, value := range s.NodeMetas {
		if hasPubNet {
			if value.HasPublic {
				nodeMetas[i] = value
				i++
			}
		} else {
			nodeMetas[i] = value
			i++
		}
	}
	return nodeMetas
}

// CheckAllServiceDeployed 检查所有的服务副本 启动情况
func (s *Scheduler) CheckAllServiceDeployed() {
	//先获取目前所有节点上运行的keeper
	keeperHost, err := getAllNolKeeper()
	if err != nil {
		log.Printf("Check-Error: %v \n", err)
	} else {
		log.Printf("Check-Info: keeper running now, keeper : %v \n", keeperHost)
	}
	// 获取每个节点上成功运行的服务
	successfulImages := getAllSuccService(keeperHost, "Check")
	// 目前启动的服务与服务列表中的服务进行对比,检查
	analysisServReplica(successfulImages)
}

func getAllNolKeeper() ([]string, error) {
	//"running-keeper-"--前缀; 先获取目前所有节点上运行的keeper
	resp, err := cli.Get(context.Background(), "running-keeper-", clientv3.WithPrefix())
	if err != nil {
		e := fmt.Sprintf("center get all keeper error, err : %v", err)
		err = errors.New(e)
		return nil, err
	}
	var keeperHost []string
	for _, kv := range resp.Kvs {
		keeperHost = append(keeperHost, string(kv.Value))
	}
	if len(keeperHost) == 0 {
		err := errors.New("no keeper service on all nodes")
		return nil, err
	}
	return keeperHost, nil
}

func getAllSuccService(keeperHost []string, logPrefix string) map[string][]brisk.NodeImage {
	imageResult := make(map[string][]brisk.NodeImage)
	for _, hostName := range keeperHost {
		log.Printf("%s-Info: center get all the services that run successfully on the node:%s \n", logPrefix, hostName)
		resp, err := cli.Get(context.Background(), "keeper-"+hostName+"-image")
		if err != nil {
			log.Printf("%s-Error: center get all service error, err : %v \n", logPrefix, err)
			continue
		}
		var succImages brisk.NodeImages
		if len(resp.Kvs) <= 0 || resp.Kvs[0] == nil || string(resp.Kvs[0].Value) == "{}" {
			err = errors.New("Etcd resp.Kvs value is not exist")
			log.Printf("%s-Error : get successNodeImages error , err : %v \n", logPrefix, err)
			continue
		}
		err = json.Unmarshal(resp.Kvs[0].Value, &succImages)
		if err != nil {
			log.Printf("%s-Error : etcd registered format error %v \n", logPrefix, err)
			continue
		}
		log.Printf("%s-Info: Node HostName: %s; all the services that run successfully on the node, service imageInfo : %v \n", logPrefix, hostName, succImages)
		//整理 出最终结果
		for name, nodeimage := range succImages {
			if _, ok := imageResult[name]; ok {
				imageResult[name] = append(imageResult[name], nodeimage)
			} else {
				imageResult[name] = []brisk.NodeImage{nodeimage}
			}
		}
	}
	log.Printf("%s-Info: all node, all successful service: %v \n", logPrefix, imageResult)
	return imageResult
}

// analysisServReplica 服务列表中的服务 对比 目前已查到的启动的服务 --> 分析结果
func analysisServReplica(succImages map[string][]brisk.NodeImage) {
	allSuccessful := true
	var succServName []string
	var failServName []string
	for name, servConfig := range scheduler.ServiceMetas {
		if value, ok := succImages[name]; ok {
			if servConfig.Replica == len(value) {
				log.Printf("Check-Info-ServiceOK: serviceName: %s ; service starts normally，the number of service replicas is correct, ,expect : %v, actual : %v \n", name, servConfig.Replica, len(value))
				succServName = append(succServName, name)
			} else {
				log.Printf("Check-Info-ServiceFAIL: serviceName: %s : wrong number of service replicas ,expect : %v, actual : %v \n", name, servConfig.Replica, len(value))
				allSuccessful = false
				failServName = append(failServName, name)
			}
		} else {
			log.Printf("Check-Info-ServiceFAIL: %s : wrong number of service replicas ,expect : %v, actual : %v \n", name, servConfig.Replica, len(value))
			allSuccessful = false
			failServName = append(failServName, name)
		}
	}
	if allSuccessful {
		log.Printf("Check-Info: All Service-replicas Correct: All service replicas started successfully，service-name: %v \n", succServName)
	} else {
		log.Printf("Check-Info: Some Service-replicas Wrong: Some services replicas failed to start, service-name: --success: %v --fail: %v \n", succServName, failServName)
	}
}

func main() {
	sch := NewScheduler()
	sch.Init()
	sch.Run()
}

// writeMail 编写邮件信息，传入msg为""时，标明删除缓存中服务名对应的邮件信息
func (s *Scheduler) writeMail(serviceName, msg string) {
	if msg == "" {
		log.Println("SendMail: MailMessage delete old message")
		delete(s.MailMessage, serviceName)
	} else {
		msg = fmt.Sprintf(time.Now().Format("2006-01-02 15:04:05") + "：" + msg)
		s.MailMessage[serviceName] = s.MailMessage[serviceName] + msg
	}
}

// sendEMail 发送信息，serviceName为对应的服务名称, subject 为邮件主题
func (s *Scheduler) sendEMail(serviceName string, subject string) {
	log.Println("SendMail: center want to send mail")
	mailMessage := s.MailMessage[serviceName]
	if len(s.MailAddressees) == 0 {
		log.Printf("SendMail-Error: No MailAddressees, Do not send mail \n")
		return
	}
	var addressees []string
	for _, value := range s.MailAddressees {
		addressees = append(addressees, value)
	}

	log.Printf("SendMail: center send mail, %s \n", subject)
	err := brisk.SendMailTLS(addressees, subject, mailMessage)
	if err != nil {
		log.Printf("SendMail-Error: center send e-mail fail, error: %v \n", err)
	} else {
		log.Println("SendMail: center send e-mail successful")
	}
	log.Println("SendMail: center send e-mail finished")
}

// CheckServiceRegister 根据keeper节点上行成功运行的服务，监测服务是否正常注册
func (s *Scheduler) CheckServiceRegister() {
	// 获取目前节点服务器上 运行的keeper host
	log.Println("Check-ServiceRegister-Info: started")
	keeperHost, err := getAllNolKeeper()
	if err != nil {
		log.Printf("Check-ServiceRegister-Error: %v \n", err)
	} else {
		log.Printf("Check-ServiceRegister-Info: keeper running now, keeper : %v \n", keeperHost)
	}
	// 根据keeper host 获取节点上成功启动运行的服务    注册使用key:fmt.Sprintf("%s-%s-%s", "service", ServiceName, xID)
	succServices := getAllSuccService(keeperHost, "Check-ServiceRegister")

	// 获取所有成功注册的 服务信息
	serverInfoMap, err := getAllRegisterServ()
	if err != nil {
		logMsg := fmt.Sprintf("Check-ServiceRegister-Error: %v \n", err)
		s.writeMail("serviceRegister", logMsg)
		log.Printf(logMsg)
		goto FINISH
	}

	// 对比检查 keeper上运行服务 与 注册服务信息 如发现错误:生成日志，邮件并发送
	for servName, nodeImages := range succServices {
		if values, ok := serverInfoMap[servName]; ok {
			if len(nodeImages) != len(values) {
				valuesMap := serverInfosToMap(values)
				for _, nodeImage := range nodeImages {
					if value, ok := valuesMap[nodeImage.Node]; !ok {
						logMsg := fmt.Sprintf("ServiceName : %s，Node: %s，Status: running; Register: fail \n", servName, value.Host)
						s.writeMail("serviceRegister", logMsg)
						log.Printf("Check-ServiceRegister-Error: %s", logMsg)
					}
				}
			}
		} else {
			for _, nodeImage := range nodeImages {
				logMsg := fmt.Sprintf("ServiceName : %s，Node: %s，Status: running; Register: fail \n", servName, nodeImage.Node)
				s.writeMail("serviceRegister", logMsg)
				log.Printf("Check-ServiceRegister-Error: %s", logMsg)
			}
		}
	}

FINISH:
	// subject 邮件主题
	if message, ok := s.MailMessage["serviceRegister"]; ok {
		log.Printf("mail message: %s \n", message)
		s.sendEMail("serviceRegister", "Check-Service-Register")
		s.writeMail("serviceRegister", "")
	} else {
		log.Println("Check-ServiceRegister-Info: finished, all services are normal")
	}
}

// getAllRegisterServ获取所有成功注册的 服务信息
func getAllRegisterServ() (map[string][]brisk.ServerInfo, error) {
	serverInfoMap := make(map[string][]brisk.ServerInfo)
	resp, err := cli.Get(context.Background(), "service-", clientv3.WithPrefix())
	if err != nil {
		msg := fmt.Sprintf("get service register info error, error: %v", err)
		err = errors.New(msg)
		return serverInfoMap, err
	}
	for _, kv := range resp.Kvs {
		var serverInfo brisk.ServerInfo
		err := json.Unmarshal(kv.Value, &serverInfo)
		if err != nil {
			log.Println("etcd registered format error", err)
			continue
		}
		serviceName := serverInfo.ServiceName
		if _, ok := serverInfoMap[serviceName]; ok {
			serverInfoMap[serviceName] = append(serverInfoMap[serviceName], serverInfo)
		} else {
			serverInfoMap[serviceName] = []brisk.ServerInfo{serverInfo}
		}
	}
	return serverInfoMap, nil
}

func serverInfosToMap(source []brisk.ServerInfo) map[string]brisk.ServerInfo {
	target := make(map[string]brisk.ServerInfo)
	for _, server := range source {
		target[server.Host] = server
	}
	return target
}
