package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"eglass.com/brisk"

	"eglass.com/utils"
	"github.com/go-redis/redis"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

var (
	isProd        = os.Getenv("prod") == "true"
	ContainerPort = os.Getenv("ContainerPort") // ContainerPort os获取配置文件上容器端口
	db            = utils.GetDB(true)
	remoteRequest = utils.InitRequest()
	redisClient   = utils.NewRedisClient(true)
	ccof          = map[string]string{
		"component_token":     "eglass2015",
		"component_appsecret": "f6a24fe5d3181f0cf5a8594e6836e1b7",
		"component_key":       "vt16ul0py8ekh5w2rhy8n0zfr2tkh9ba4933ntroe41",
		"component_appid":     "wx83f34d54af1b48b4",
	}
)

func getComponentToken() (string, error) {
	actKey := "component-access-token"
	atoken, error := redisClient.Get(actKey).Result()
	if error == nil {
		return atoken, nil
	}
	if error != redis.Nil {
		return "", error
	}
	ticket, error := redisClient.Get("component-ticket").Result()
	if error != nil {
		return "", error
	}
	body := utils.Body{
		"component_appid":         ccof["component_token"],
		"component_appsecret":     ccof["component_appsecret"],
		"component_verify_ticket": ticket,
	}
	cr, error := remoteRequest.JPost("https://api.weixin.qq.com/cgi-bin/component/api_component_token", nil, body)
	if cat, ok := cr["component_access_token"].(string); ok {
		if redisClient.SetNX(actKey, cat, time.Second*7100).Err() != nil {
			return "", nil
		}
		return cat, nil
	}
	return "", error
}
func queryProductRefreshToken(appid string) (string, error) {
	var refreshToken string
	error := db.QuerySingle(&refreshToken, "select refresh_token from n_wechat_open_refresh where appid = ?", appid)
	return refreshToken, error
}
func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

func getAppidToken(appid string) (string, error) {
	if appid == "" {
		return "", errors.New("AppId needed")
	}
	appidKey := fmt.Sprintf("access-token-wechat-%s", appid)
	cachedToken, error := redisClient.Get(appidKey).Result()
	if error == nil {
		// 直接读取缓存
		return cachedToken, nil
	}
	if error != redis.Nil {
		// 异常错误
		log.Printf("%v", error)
		return "", errors.New("redis key not exist")
	}
	// refresh
	cToken, err := getComponentToken()
	if err != nil {
		return "", err
	}
	refreshToken, err := queryProductRefreshToken(appid)
	if err != nil {
		return "", err
	}
	r, error := remoteRequest.JPost(
		fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/component/api_authorizer_token?component_access_token=%s", cToken),
		nil,
		utils.Body{
			"component_appid":          ccof["component_appid"],
			"authorizer_appid":         appid,
			"authorizer_refresh_token": refreshToken,
		})
	if err != nil {
		return "", err
	}
	finalToken, ok := r["authorizer_access_token"].(string)
	if !ok {
		return "", errors.New("get authorizer_access_token failed ")
	}
	redisClient.SetNX(appidKey, finalToken, time.Second*7100)
	return finalToken, nil
}

// GetAppidToken 获得appid的access_token
func GetAppidToken(c echo.Context) error {
	appid := c.QueryParam("appid")
	token, error := getAppidToken(appid)
	if error != nil {
		c.String(400, error.Error())
		return error
	}
	c.String(200, token)
	return error
}

// SendTemplate  发送模板消息
func SendTemplate(c echo.Context) error {
	// remoteRequest.PostAsJSON(uri)
	req := c.Request()
	defer req.Body.Close()
	body, error := ioutil.ReadAll(req.Body)
	if error != nil {
		return error
	}
	url := "https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=" + c.QueryParam("access_token")
	proxyReq, error := http.NewRequest(req.Method, url, bytes.NewReader(body))
	if error != nil {
		return error
	}
	proxyReq.Header = req.Header
	resp, error := remoteRequest.GetClient().Do(proxyReq)
	if error != nil {
		return error
	}
	defer resp.Body.Close()
	_, error = io.Copy(c.Response(), resp.Body)
	return error
}

// MakeProxyRequest 代理到微信服务器
func MakeProxyRequest(method, weixinURL string) func(c echo.Context) error {
	return func(c echo.Context) error {
		appid := c.QueryParam("appid")
		if appid == "" {
			c.String(400, "appid shound not be empty")
			return nil
		}
		token, err := getAppidToken(appid)
		if err != nil {
			c.String(400, fmt.Sprintf("get access token failed: %v", err)
			log.Printf("get access token failed: appid: %s, %v", appid, err)
			return err
		}
		c.QueryParams().Add("access_token", token)
		qs := c.QueryParams().Encode()
		c.Logger().Info(qs)
		req := c.Request()
		url := weixinURL
		if qs != "" {
			url = strings.Join([]string{weixinURL, qs}, "?")
		}
		var proxyReq *http.Request
		if req.Method == "GET" {
			proxyReq, err = http.NewRequest(req.Method, url, nil)
			if err != nil {
				return err
			}
		} else {
			defer req.Body.Close()
			body, error := ioutil.ReadAll(req.Body)
			if error != nil {
				return error
			}
			reqBody := bytes.NewReader(body)
			proxyReq, err = http.NewRequest(req.Method, url, reqBody)
			if err != nil {
				return err
			}
		}
		proxyReq.Header = req.Header
		resp, error := remoteRequest.GetClient().Do(proxyReq)
		if error != nil {
			return error
		}
		defer resp.Body.Close()
		_, error = io.Copy(c.Response(), resp.Body)
		return error
	}
}

const (
	GET  = "GET"
	POST = "POST"
)

// 前缀是 "http://t.epeijing.cn/wechat", 加上 微信api的路径
// example
// https://t.epeijing.cn/wechat/cgi-bin/user/info
// https://t.epeijing.cn/wechat/cgi-bin/user/batchget
// https://t.epeijing.cn/wechat/cgi-bin/message/template/send
var weixinApis = map[string]string{
	// 左边是微信api
	// 绑定微信用户为小程序体验者
	"https://api.weixin.qq.com/wxa/bind_tester": POST,
	// 获取单个userinfo
	"https://api.weixin.qq.com/wxa/get_qrcode":    GET,
	"https://api.weixin.qq.com/cgi-bin/user/info": GET,
	// 批量获取userinfo
	"https://api.weixin.qq.com/cgi-bin/user/info/batchget": POST,
	// 模板消息发送
	"https://api.weixin.qq.com/cgi-bin/message/template/send": POST,
	// 临时素材上传
	"https://api.weixin.qq.com/cgi-bin/media/uploadimg": POST,
	"https://api.weixin.qq.com/cgi-bin/media/upload":    POST,
	// 卡券新建
	"https://api.weixin.qq.com/card/create": POST,
	// 卡券更新
	"https://api.weixin.qq.com/card/update": POST,
	// 修改库存接口
	"https://api.weixin.qq.com/card/modifystock": POST,
	// 创建 开放平台帐号
	"https://api.weixin.qq.com/cgi-bin/open/create": POST,
	// 将公众号/小程序绑定到开放平台帐号下
	"https://api.weixin.qq.com/cgi-bin/open/bind": POST,
	// 获取公众号/小程序所绑定的开放平台帐号
	"https://api.weixin.qq.com/cgi-bin/open/get": POST,
	// 将公众号/小程序从开放平台帐号下解绑
	"https://api.weixin.qq.com/cgi-bin/open/unbind": POST,
	// 拉取用户会员卡信息
	"https://api.weixin.qq.com/card/membercard/userinfo/get": POST,
	// 获取用户已领取卡券接口
	"https://api.weixin.qq.com/card/user/getcardlist": POST,
	// 查看卡券详情
	"https://api.weixin.qq.com/card/get": POST,
	// 客服消息
	"https://api.weixin.qq.com/cgi-bin/message/custom/send": POST,
}

func extractResource(URL string) string {
	s := strings.Split(URL, "/")
	var i int
	for i = len(s) - 1; i > 0; i-- {
		if s[i] == "api.weixin.qq.com" {
			break
		}
	}
	return "/" + strings.Join(s[i+1:], "/")
}

func BulkBind(g *echo.Group) {
	for url, method := range weixinApis {
		endPoint := extractResource(url)
		g.Add(method, endPoint, MakeProxyRequest(method, url))
		log.Printf("add route [%s] %s for weixin url: %s\n ", method, endPoint, url)
	}
}

func main() {
	server := ServerInstance{}
	brisk.HandleServiceLifeCycle(server)
}

// ServerInstance 实现接口
type ServerInstance struct {
	err error
}

// Server 实现接口方法
func (s ServerInstance) Service() error {
	defer db.Close()
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	// wechat
	w := e.Group("/wechat")
	w.GET("/appidtoken", GetAppidToken)
	w.POST("/template/send", SendTemplate)
	BulkBind(w)
	// image
	image := e.Group("/image")
	image.POST("/composite", Composite)
	return e.Start(fmt.Sprintf(":%s", ContainerPort))
}
