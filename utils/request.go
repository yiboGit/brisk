package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Method string

const GET Method = "GET"

type RemoteRequest struct {
	client *http.Client
}
type Query map[string]interface{}
type Result Query
type Body Query

var isProd = os.Getenv("prod") == "true"

var ServerError = errors.New("status not 200")

func InitRequest() *RemoteRequest {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	return &RemoteRequest{&http.Client{Transport: tr, Timeout: 5 * time.Second}}
}
func (r *RemoteRequest) GetRaw(uri string, qs Query) ([]byte, error) {
	endPoint := getEndPoint(uri, qs)
	resp, err := r.client.Get(endPoint)
	if err != nil {
		log.Printf("client request error: url-> %s, %v ", endPoint, err)
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
func (r *RemoteRequest) JGet(uri string, qs Query) (Result, error) {
	body, err := r.GetRaw(uri, qs)
	if err != nil {
		return nil, err
	}
	return decodeJson(body)
}

// PostAsJSON  PostAsJson make json request, return raw response
func (r *RemoteRequest) PostAsJSON(uri string, body io.Reader) (*http.Response, error) {
	return r.client.Post(uri, "application/json", body)
}
func (r *RemoteRequest) GetClient() *http.Client {
	return r.client
}

// PostRaw post json & parse response as json
func (r *RemoteRequest) PostRaw(uri string, qs Query, body Body) ([]byte, error) {
	endPoint := getEndPoint(uri, qs)
	d, err := json.Marshal(body)
	if err != nil {
		log.Printf("json encode error : data-> %s, %v ", body, err)
		return []byte(""), err
	}
	log.Printf("post: %s ", string(d))
	resp, err := r.client.Post(endPoint, "application/json; charset=utf-8", bytes.NewReader(d))
	if err != nil {
		log.Printf("client request error: url-> %s, %v ", endPoint, err)
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
func (r *RemoteRequest) Upload(uri string, qs Query, values map[string]io.Reader) (Result, error) {
	var err error
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range values {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		// Add an image file
		if x, ok := r.(*os.File); ok {
			if fw, err = w.CreateFormFile(key, x.Name()); err != nil {
				return nil, err
			}
		} else {
			// Add other fields
			if fw, err = w.CreateFormFile(key, "image.jpg"); err != nil {
				return nil, err
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			return nil, err
		}

	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	endPoint := getEndPoint(uri, qs)
	req, err := http.NewRequest("POST", endPoint, &b)
	if err != nil {
		return nil, err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Submit the request
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return decodeJson(data)
}
func decodeJson(body []byte) (Result, error) {
	var dat Result
	err := json.Unmarshal(body, &dat)
	if err != nil {
		log.Printf("json parse error: json-> %s %v", body, err)
		return nil, err
	}
	return dat, nil
}

func (r *RemoteRequest) JPost(uri string, qs Query, body Body) (Result, error) {
	resp, err := r.PostRaw(uri, qs, body)
	if err != nil {
		return nil, err
	}
	return decodeJson(resp)
}

func (r *RemoteRequest) GetAppidToken(appid string) (string, error) {
	token, err := r.GetRaw("https://epeijing.cn/api/v2/wechat/appidtoken?appid="+appid, nil)
	if err != nil {
		return "", fmt.Errorf("get appid %s token error: %v", appid, err)
	}
	return string(token), nil
}

func (r *RemoteRequest) GetUserInfo(appid, openid string) (Result, error) {
	token, error := r.GetAppidToken(appid)
	if error != nil {
		return nil, error
	}
	return r.JGet("https://api.weixin.qq.com/cgi-bin/user/info",
		Query{"access_token": token, "openid": openid, "lang": "zh_CN"})
}

// KfSendMessage 发送客服消息
func (r *RemoteRequest) KfSendMessage(appid, openid, msgtype string, extra map[string]interface{}) (Result, error) {
	body := Body{
		"touser":  openid,
		"msgtype": msgtype,
	}
	body[msgtype] = extra
	return r.JPost("https://epeijing.cn/api/v2/wechat/cgi-bin/message/custom/send", Query{"appid": appid}, body)
}

// SendTemplateMsg 发送模板消息
func (r *RemoteRequest) SendTemplateMsg(appid, touser, templateID string, body Body, uri string, mini Body) (Result, error) {
	finalBody := make(Body)
	finalBody["touser"] = touser
	finalBody["template_id"] = templateID
	if uri != "" {
		finalBody["url"] = uri
	}
	if mini != nil {
		finalBody["miniprogram"] = mini
	}
	finalBody["data"] = body
	url := "https://epeijing.cn/api/v2/wechat/cgi-bin/message/template/send"
	result, err := r.JPost(url, Query{"appid": appid}, finalBody)
	if err != nil {
		return nil, err
	}
	log.Printf("%v, %v", result, err)
	errcode, ok := result["errcode"].(float64)
	if ok && errcode > 0 {
		return nil, fmt.Errorf("%v", result)
	}
	return result, nil
}

func getEndPoint(uri string, qs Query) string {
	var endPoint = uri
	if qs != nil {
		endPoint += "?"
		var buffer []string
		var s string
		for k, v := range qs {
			s, _ = v.(string)
			buffer = append(buffer, fmt.Sprintf("%s=%s", k, url.QueryEscape(s)))
		}
		endPoint += strings.Join(buffer, "&")
	}
	return endPoint
}
