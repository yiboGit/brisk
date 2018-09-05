package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/labstack/echo"
	"github.com/rs/xid"
)

const redisLogKey = "logstash-list"

type CustomContext struct {
	echo.Context
	serviceName string
	requestID   string
}

type logMessage struct {
	RequestID   string `json:"requestID"`
	ServiceName string `json:"serviceName"`
	Message     string `json:"message"`
}

var redisClient = NewRedisClient(false)

func GetLoggerMiddleWare(serviceName string) echo.MiddlewareFunc {
	return func(n echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			id := c.QueryParam("requestID")
			fmt.Println("id : " + id)
			if id == "" {
				id = fmt.Sprint(xid.New())
			}
			customContext := &CustomContext{c, serviceName, id}
			defer customContext.requestLog()
			err = n(customContext)
			if err != nil {
				errLog := fmt.Sprintf("requestID : %s , service name : %s ,error log message marshal error, err : %v", customContext.requestID, customContext.serviceName, err)
				redisClient.LPush(redisLogKey, errLog)
				return err
			}
			return nil
		}
	}
}

func (c CustomContext) requestLog() {
	requestMsg := make(map[string]interface{})
	req := c.Request()
	res := c.Response()
	requestMsg["requestID"] = c.requestID
	requestMsg["serviceName"] = c.serviceName
	requestMsg["host"] = req.Host
	requestMsg["uri"] = req.RequestURI
	requestMsg["queryParam"] = c.QueryString()
	requestMsg["method"] = req.Method
	p := req.URL.Path
	if p == "" {
		p = "/"
	}
	requestMsg["path"] = p
	requestMsg["status"] = res.Status
	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	body := buf.String()
	body = strings.Replace(body, "\n", "", -1)
	body = strings.Replace(body, "\t", "", -1)
	requestMsg["body"] = body
	requestMsg["endTime"] = ToFullTimeString(nil)
	logMessage, err := json.Marshal(requestMsg)
	if err != nil {
		errLog := fmt.Sprintf("requestID : %s , service name : %s ,request log message marshal error, err : %v", c.requestID, c.serviceName, err)
		redisClient.LPush(redisLogKey, errLog)
		return
	}
	// 初始log记录
	redisClient.LPush(redisLogKey, string(logMessage))
}

/**
 * log CustomContext记录日志，保存入elastic
 */
func (c CustomContext) Log(msg string) {
	msg = fmt.Sprintf("%s : %s", ToFullTimeString(nil), msg)
	fmt.Printf("%s \n", msg)
	logMsg := logMessage{
		c.requestID,
		c.serviceName,
		msg,
	}
	fmt.Printf("%v \n", logMsg)
	message, err := json.Marshal(logMsg)
	if err != nil {
		errLog := fmt.Sprintf("requestID : %s , service name : %s ,log message marshal error, err : %v", c.requestID, c.serviceName, err)
		redisClient.LPush(redisLogKey, errLog)
		return
	}
	// 保存入redis
	redisClient.LPush(redisLogKey, string(message))
}

/**
 * esLog 需要保存入elastic的重要日志
 */
func EsLog(serviceName string, logMsg interface{}) {
	msg := fmt.Sprintf("%s %s: %v", ToFullTimeString(nil), serviceName, logMsg)
	// 存入redis
	redisClient.LPush(redisLogKey, string(msg))
}
