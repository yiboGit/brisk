package utils

import "testing"
import "github.com/stretchr/testify/assert"
import "os"
import "fmt"
import "io"

const uri = "https://epeijing.cn/api/call/wechat/getTheAppidToken"

var rreq = InitRequest()

func TestRequest(t *testing.T) {
	// expected := Result{"errmsg":"ok", "auditid": float64(424668642) , "status": float64(2), "errcode": float64(0)}
	result, _ := rreq.GetRaw(uri, Query{"appid": "wxe5e1ebb94f05c1ae"})
	assert.Equal(t, "", string(result))
}

func TestRequestPost(t *testing.T) {
	// expected := Result{"url": "http://weixin.qq.com/q/02wtZ80Qfo9OP10000003G"}
	result, _ := rreq.KfSendMessage("wx2f69092f1ba47691", "o8X_Ajp2c4easBFLJyRLNR3Z7auY", "wxcard", Body{"card_id": "p8X_AjsVSZOiOHQBKT-YgL21uAxg"})
	assert.Equal(t, nil, result)
}
func TestUpload(t *testing.T) {
	file, err := os.Open("g.jpg")
	v := map[string]io.Reader{
		"media": file,
	}
	r, err := rreq.Upload("https://t.epeijing.cn/api/v2/wechat/cgi-bin/media/uploadimg", Query{"appid": "wx7cc34ee2a43fe772"}, v)
	fmt.Print(r)
	assert.Equal(t, nil, err)
}
