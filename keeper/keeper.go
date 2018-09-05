package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"eglass.com/brisk"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/rs/xid"
)

// 镜像命名规则： 私服的HostName:5000/服务名:版本号
// 命名中带有服务名 有利于针对ServiceInfo 与 keeper 的联动操作

var (
	Etcd         = os.Getenv("Etcd")        // Etcd "t.epeijing.cn:2379"
	RestartTime  = os.Getenv("RestartTime") // 1 min
	cli, etcdErr = clientv3.New(clientv3.Config{
		Endpoints:   []string{Etcd},
		DialTimeout: 5 * time.Second,
	})
	keeper = newKeeper()
)

type Keeper struct {
	// successNodeImages 启动成功的 image 存储在本机的缓存，同时也要同步在etcd上,key值为当前节点的 hostname-images
	successNodeImages brisk.NodeImages
	// failNodeImages 启动失败的 image 存储在本机的缓存
	failNodeImages brisk.NodeImages
}

func newKeeper() *Keeper {
	return &Keeper{
		successNodeImages: brisk.NewNodeImages(),
		failNodeImages:    brisk.NewNodeImages(),
	}
}

// init NodeImageCache初始化方法
func (k *Keeper) init() error {
	log.Println("keeper get Hostname")
	hostName := brisk.GetHostname()
	log.Printf("keeper hostname: %s \n", hostName)
	log.Println("keeper start get keeper-hostName-image")
	resp, err := cli.Get(context.Background(), "keeper-"+hostName+"-image")
	if err != nil {
		log.Printf("Error : Etcd get data, error: %s \n", err)
		return err
	}
	log.Println("keeper start get keeper-hostName+image Finished")
	if len(resp.Kvs) <= 0 || resp.Kvs[0] == nil || string(resp.Kvs[0].Value) == "{}" {
		err = errors.New("Etcd resp.Kvs value is not exist")
		log.Printf("Error : %v \n", err)
		return err
	}
	// var nodeImageCache NodeImageCache
	err = json.Unmarshal(resp.Kvs[0].Value, k.successNodeImages)
	if err != nil {
		log.Fatalf("Error : etcd registered format error %v \n", err)
	}
	if len(k.successNodeImages) == 0 {
		return nil
	}
	log.Printf("InitInfo : keeper start service-images  \n")
	k.startImages()
	return nil
}

// startImages 初始化时启动操作
func (k *Keeper) startImages() {
	k.successNodeImages, k.failNodeImages = k.startOperation(false)
	log.Printf("KeeperInfo : images started successfully , successsuccessNodeImages : %v \n", k.successNodeImages)
	log.Printf("KeeperInfo : images failed to start , failNodeImages : %v \n", k.failNodeImages)
	k.syncNodeImage()
}

// restartFailImage 失败服务列表重启
func (k *Keeper) restartFailImage() {
	reSuccessImages, _ := k.startOperation(true)

	for key, value := range reSuccessImages {
		if _, ok := k.failNodeImages[key]; ok {
			delete(k.failNodeImages, key)
		}
		k.successNodeImages[key] = value
	}
	log.Printf("RestartInfo : restart service images , Successful images: %v \n", k.successNodeImages)
	log.Printf("RestartInfo : restart service images , failed images : %v \n", k.failNodeImages)
	k.syncNodeImage()
}

// startOperation 启动操作，返回两个map：NO.1为启动成功map， NO.2为启动失败map
// restart bool参数  表示是失败重启任务，如果是 取失败重启列表；如果否，取成功列表
func (k *Keeper) startOperation(restart bool) (map[string]brisk.NodeImage, map[string]brisk.NodeImage) {
	successImageMap := make(map[string]brisk.NodeImage)
	failNodeImageMap := make(map[string]brisk.NodeImage)
	nodeImages := k.successNodeImages
	if restart {
		nodeImages = k.failNodeImages
	}
	for _, value := range nodeImages {
		name, version, err := brisk.SplitFullName(value.FullName)
		if err != nil {
			log.Printf("Error : dockerImage name has error, Image fullName %s \n", value.FullName)
			continue
		}
		//拉取
		err = pullImage(value.FullName)
		if err != nil {
			log.Printf("Error : pull image error, %v \n", err)
			failNodeImageMap[name] = value
			continue
		}
		//停止
		err = stopImage(value.ContainerID)
		if err != nil {
			log.Printf("Warning : stop image error, %v \n", err)
		}

		var imageInfo brisk.ImageInfo
		imageInfo.FullName = value.FullName
		imageInfo.Env = value.Env
		imageInfo.Node = value.Node
		imageInfo.Name = name
		imageInfo.Version = version
		// 运行
		cid, err := runImage(imageInfo.FullName, imageInfo.Env)
		if err != nil {
			log.Printf("Error : run image error, %v \n", err)
			failNodeImageMap[name] = value
			continue
		}
		imageInfo.ContainerID, value.ContainerID = cid, cid
		imageInfo.CreateTime = time.Now()
		value.ImageInfoKey = putImageInfo(value.ImageInfoKey, imageInfo)
		successImageMap[imageInfo.Name] = value
	}
	return successImageMap, failNodeImageMap
}

func (k *Keeper) syncNodeImage() {
	hostName := brisk.GetHostname()
	cacheValue, err := json.Marshal(k.successNodeImages)
	if err != nil {
		log.Printf("Error: successNodeImages Marshal error %v \n", err)
	}
	_, err = cli.Put(context.Background(), "keeper-"+hostName+"-image", string(cacheValue))
	if err != nil {
		log.Printf("Error: successNodeImages put error %v \n", err)
	}
	log.Printf("Success: successNodeImages sync, %s \n", string(cacheValue))
}

// 镜像 名称解析规则 host:5000/name:version
func main() {
	if etcdErr != nil {
		log.Fatal("Error: cannot connect to etcd ", etcdErr)
	}
	log.Println("Successful: connect to etcd")
	//TODO
	log.Println("Etcd : " + Etcd + ",time : " + RestartTime)
	// init初始化 使用场景：重启 恢复etcd里的服务  将之前保存在etcd中的全部启动后 同步到etcd上
	log.Println("keeper init start")
	err := keeper.init()
	if err != nil {
		log.Printf("Error: Keeper successNodeImages init failed , err: %v \n", etcdErr)
	}
	log.Println("Successful: Keeper successNodeImages init successful")
	hostname := brisk.GetHostname()

	watcher := clientv3.NewWatcher(cli)
	// w 监控镜像的上传，进行启动
	w := watcher.Watch(context.Background(), "docker-image", clientv3.WithPrefix())
	// rm 监控服务的销毁删除，删除本地缓存上的以及etcd上的正在运行的镜像记录
	rm := watcher.Watch(context.Background(), fmt.Sprintf("%s-%s-", "RM", hostname), clientv3.WithPrefix())
	// restartMin 失败服务重启时间，推荐1分钟左右,具体时间根据配置文件而定
	restartMin, err := strconv.Atoi(RestartTime)
	// keeperStarted 向etcd put运行成功的keeper
	keeperStarted(hostname)
	// 监控服务 挂掉
	c := make(chan os.Signal, 1)
	//监测os的三种关于退出，销毁的信号
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	if err != nil {
		// 若取配置的重启时间失败，则默认使用一分钟的重启时间
		log.Printf("Restart : restart time convert err, Error : %v \n", err)
		restartMin = 1
		log.Printf("Restart : restart time use default time 1 min \n")
	}
	restartTicker := time.NewTicker(time.Duration(restartMin) * time.Minute)
	for {
		select {
		// watchResponse 监控镜像信息
		case watchResponse := <-w:
			for _, event := range watchResponse.Events {
				var dockerImage brisk.DockerImage
				// 监听	etcd 关于镜像信息 的操作
				if event.Type == mvccpb.PUT {
					log.Println("Keeper: keeper has a new task")
					err := json.Unmarshal(event.Kv.Value, &dockerImage)
					log.Printf("Keeper: dockerImage : %v \n", dockerImage)
					if hostname != dockerImage.Node {
						log.Printf("Keeper: hostName does not match，this HostName: %s, image Node : %s \n", hostname, dockerImage.Node)
						continue
					}
					if err != nil {
						log.Printf("Error : etcd registered format error ,err : %v,docker-image Key :%s \n", err, string(event.Kv.Key))
						continue
					}
					log.Println("Keeper: update-image start")
					err = updateImage(dockerImage)
					// 启动镜像失败
					if err != nil {
						log.Printf("Error : updateImage has error,err:%v ,docker-image Key :%s \n", err, string(event.Kv.Key))
						log.Printf("Info: Send rolling-update failure info to the center \n")
						feedbackUpdateImage("false", dockerImage.Env["ServiceName"])
						continue
					}
					// 启动镜像成功
					log.Printf("Info: Send rolling-update success info to the center \n")
					feedbackUpdateImage("true", dockerImage.Env["ServiceName"])
				}
			}
		// rmwatchResponse 监控服务容器 销毁失败
		case rmwatchResponse := <-rm:
			for _, event := range rmwatchResponse.Events {
				if event.Type == mvccpb.PUT {
					log.Println("Keeper-Info : keeper has a remove task")
					var value map[string]string
					json.Unmarshal(event.Kv.Value, &value)
					// 服务名
					serviceName := value["serviceName"]
					//容器ID
					containerID := value["containerId"]
					log.Printf("Keeper-Info : serviceName : %s, containerId : %s  \n", serviceName, containerID)
					// 若需要删除的镜像服务，本地NodeImageCache存在，删除本地记录 同步到etcd上
					if nodeImage, ok := keeper.successNodeImages[serviceName]; ok {
						// 取到该服务的成功运行记录，与当前监控到的服务删除记录 containId是否一致，一致则删除成功记录；否则，不删除
						cID := string([]rune(nodeImage.ContainerID)[:12])
						log.Printf("Keeper-Info : successNodeImages: serviceName : %s, containerId : %s  \n", serviceName, cID)
						if cID == containerID {
							// 加入到重启缓存中
							log.Println("Keeper-Info : the service record add to keeper (failNodeImages)")
							keeper.failNodeImages[serviceName] = keeper.successNodeImages[serviceName]
							log.Println("Keeper-Info : remove service record from keeper (successNodeImages)")
							delete(keeper.successNodeImages, serviceName)
							log.Println("Keeper-Info : successNodeImages sync to etcd")
							keeper.syncNodeImage()
							// 删除 service-nodeImage 的 remove记录
							log.Printf("Keeper-Info : delete rm-info from etcd \n")
							cli.Delete(context.Background(), string(event.Kv.Key))
						}
					}
				}
			}
		case <-restartTicker.C:
			if len(keeper.failNodeImages) > 0 {
				// 进行重启
				log.Printf("Restart : restart keeper failNodeImages, %v \n", keeper.failNodeImages)
				keeper.restartFailImage()
			}
		case <-c:
			// keeper本身服务宕机 删除etcd上运行
			cli.Delete(context.Background(), "running-keeper-"+hostname)
			log.Println("Keeper-Info: keeper status stopped ")
		}
	}
}

// 为滚动升级 反馈镜像的执行信息
func feedbackUpdateImage(isSuccessful string, serviceName string) {
	_, err := cli.Put(context.Background(), "rolling-update-"+serviceName, isSuccessful)
	if err != nil {
		log.Printf("Error : feedback rolling-update info error, failed to send , err : %v \n", err)
		return
	}
	log.Printf("Successful: feedback rolling-update info ok, send  successfully \n")
}

func updateImage(dockerImage brisk.DockerImage) error {
	var imageInfo brisk.ImageInfo
	log.Println("Converter start")
	imageInfo.Converter(dockerImage)
	infoKey, containerID := getOldImageInfo(imageInfo.Name, imageInfo.Node)
	// 准备fail信息，若失败使用，添加入本地失败的缓存内；反之，搁置不用
	failNodeImage := brisk.NodeImage{
		ImageInfoKey: infoKey,
		Node:         imageInfo.Node,
		FullName:     imageInfo.FullName,
		Env:          imageInfo.Env,
		ContainerID:  imageInfo.ContainerID,
	}
	// pull新镜像
	log.Println("Pull: pullImage start")
	err := pullImage(dockerImage.FullName)
	if err != nil {
		log.Printf("Pull : pull image error, %v \n", err)
		keeper.failNodeImages[imageInfo.Name] = failNodeImage
		return err
	}
	log.Println("Pull: pullImage finished")

	// 是更新/新建
	if infoKey != "" && containerID != "" {
		//更新操作 stop 旧容器
		log.Printf("Stop 旧容器: start \n")
		err = stopImage(containerID)
		if err != nil {
			log.Printf("Stop : stop image error, %v \n", err)
		}
	}
	//run 新镜像
	log.Println("Run: runImage start")
	cid, err := runImage(imageInfo.FullName, imageInfo.Env)
	if err != nil {
		log.Printf("Run : run image error, %v \n", err)
		keeper.failNodeImages[imageInfo.Name] = failNodeImage
		return err
	}
	log.Println("Run: runImage ok")
	//删除可能存在的启动失败的旧镜像信息
	if _, ok := keeper.failNodeImages[imageInfo.Name]; ok {
		delete(keeper.failNodeImages, imageInfo.Name)
	}
	imageInfo.ContainerID = cid
	imageInfo.CreateTime = time.Now()
	//put 新镜像容器的信息
	key := putImageInfo(infoKey, imageInfo)

	keeper.successNodeImages[imageInfo.Name] = brisk.NodeImage{
		ImageInfoKey: key,
		FullName:     imageInfo.FullName,
		Env:          imageInfo.Env,
		Node:         imageInfo.Node,
		ContainerID:  imageInfo.ContainerID,
	}
	keeper.syncNodeImage()
	return nil
}

func putImageInfo(infoKey string, imageInfo brisk.ImageInfo) string {
	var key string
	if infoKey == "" {
		// 新建信息
		xID := fmt.Sprintf("%s", xid.New())
		key = fmt.Sprintf("%s-%s-%s", "image", imageInfo.Name, xID)
		log.Println("ImageInfo : create ImageInfo, ImageInfo put new key")
	} else {
		// 更新信息
		key = infoKey
		log.Println("ImageInfo : update ImageInfo, ImageInfo put old key")
	}
	value, err := json.Marshal(imageInfo)
	if err != nil {
		log.Printf("Error : Image Info has error, Error: %v \n", err)
	}
	// PUT 保存ImageInfo记录信息
	_, err = cli.Put(context.Background(), key, string(value))
	if err != nil {
		log.Printf("Error : Image info put error, Error: %v \n", err)
	}
	return key
}

func pullImage(imageFullName string) error {
	pullCmd := exec.Command("sh", "-c", fmt.Sprintf("docker pull %s", imageFullName))
	pullStdout, err := pullCmd.CombinedOutput()
	strStdout := string(pullStdout)
	if err != nil {
		log.Printf("Error : docker pull %s fail, error: %v ,info: %s \n", imageFullName, err, strStdout)
		return err
	}
	log.Printf("Info : docker pull %s successful, info: %s \n", imageFullName, strStdout)
	return nil
}

// stopImage 停止
func stopImage(containerID string) error {
	stopCmd := exec.Command("sh", "-c", fmt.Sprintf("docker stop %s", containerID))
	stopStdout, err := stopCmd.CombinedOutput()
	strStdout := string(stopStdout)
	if err != nil {
		log.Printf("Warning: docker stop %s fail, error: %v ,info: %s \n", containerID, err, strStdout)
		return err
	}
	log.Printf("Info : docker stop %s successful, info: %s \n", containerID, strStdout)
	return nil
}

// runImage运行
func runImage(imageFullName string, env map[string]string) (string, error) {
	var runCmd *exec.Cmd
	if len(env) != 0 {
		envStr := ""
		for key, value := range env {
			envStr += fmt.Sprintf("-e %s%s=%s%s ", `"`, key, value, `"`)
		}
		portStr := "-p "
		if env["Port"] == "" || env["ContainerPort"] == "" {
			err := errors.New("docker run image : missing important parameters")
			log.Printf("Error : %v，Port,ContainerPort \n", err)
			return "", err
		}
		portStr += fmt.Sprintf("%s:%s ", env["Port"], env["ContainerPort"])
		runCmd = exec.Command("sh", "-c", fmt.Sprintf("docker run -d %s%s%s", envStr, portStr, imageFullName))
		log.Println(fmt.Sprintf("docker run -d %s%s%s", envStr, portStr, imageFullName))
	} else {
		runCmd = exec.Command("sh", "-c", fmt.Sprintf("docker run -d %s", imageFullName))
	}
	runStdout, err := runCmd.CombinedOutput()
	stdoutStr := string(runStdout)
	// 去除换行符 避免意外情况
	stdoutStr = strings.Replace(stdoutStr, "\n", "", -1)
	// 去除空格 避免意外情况
	stdoutStr = strings.Replace(stdoutStr, " ", "", -1)
	if err != nil {
		log.Printf("Error : docker run %s fail, error: %v ,info: %s \n", imageFullName, err, stdoutStr)
		return "", err
	}
	log.Printf("Info : docker run %s successful, info: %s \n", imageFullName, stdoutStr)
	return stdoutStr, nil
}

func getOldImageInfo(name string, node string) (string, string) {
	log.Printf("getOldImageInfo() name %s, node: %s \n", name, node)
	log.Printf("successNodeImages %v \n", keeper.successNodeImages)
	nodeImage, ok := keeper.successNodeImages[name]
	infoKey, containerID := "", ""
	if !ok {
		log.Println("Error: the service-image not running node")
		return "", ""
	}

	if nodeImage.Node == node {
		log.Println("nodeImage.Node 与 node 是等于的")
		containerID = nodeImage.ContainerID
		infoKey = nodeImage.ImageInfoKey
	}
	//TODO
	log.Println("getOldImageInfo() ok", infoKey, " ", "containerID", containerID)
	return infoKey, containerID
}

func keeperStarted(hostName string) {
	_, err := cli.Put(context.Background(), "running-keeper-"+hostName, hostName)
	if err != nil {
		log.Printf("Error: keeper put etcd error %v \n", err)
	}
	log.Printf("Successful: Keeper put etcd successfully \n")
}
