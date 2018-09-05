package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"eglass.com/utils"

	"github.com/denverdino/aliyungo/oss"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"

	"github.com/disintegration/imaging"
)

var (
	utf8FontFile = "msyh.ttf"
	utf8FontSize = float64(15.0)
	spacing      = float64(1.5)
	dpi          = float64(72)
	ctx          = new(freetype.Context)
	red          = color.RGBA{255, 0, 0, 255}
	blue         = color.RGBA{0, 0, 255, 255}
	white        = color.RGBA{255, 255, 255, 255}
	black        = color.RGBA{0, 0, 0, 255}
	reqs         = utils.InitRequest()
)

const ACCESS_KEY_ID = "uslNjzUwTMSXnDF4"
const ACCESS_KEY_SECRET = "XTLgq8JSMIDPgZiGO6WZb7AHyqFPOJ"

var ossBucket *oss.Bucket
var utf8Font *truetype.Font

type circle struct {
	p image.Point
	r int
}

func (c *circle) ColorModel() color.Model {
	return color.AlphaModel
}

func (c *circle) Bounds() image.Rectangle {
	return image.Rect(c.p.X-c.r, c.p.Y-c.r, c.p.X+c.r, c.p.Y+c.r)
}

func (c *circle) At(x, y int) color.Color {
	xx, yy, rr := float64(x-c.p.X)+0.5, float64(y-c.p.Y)+0.5, float64(c.r)
	if xx*xx+yy*yy < rr*rr {
		return color.Alpha{255}
	}
	return color.Alpha{0}
}

func GetOssBucket() *oss.Bucket {
	if ossBucket == nil {
		ossClient := oss.NewOSSClient("oss-cn-shanghai", os.Getenv("prod") == "1", ACCESS_KEY_ID, ACCESS_KEY_SECRET, false)
		ossBucket := ossClient.Bucket("epj-images")
		return ossBucket
	}
	return ossBucket
}

type GenResult struct {
	OssUrl   string `json:"ossUrl"`
	MediaUrl string `json:"mediaUrl"`
}

func DoImageOperation(op *ImageOperation) (*GenResult, error) {
	var err error
	base := op.Base
	subImages := op.SubImages
	extra := op.Extra
	urls := make([]string, 1+len(subImages))

	result := make([]image.Image, 1+len(subImages))
	urls[0] = base
	for i, si := range subImages {
		urls[i+1] = si.URL
	}
	var w sync.WaitGroup
	for i, mUrl := range urls {
		w.Add(1)
		go func(i int, mUrl string) {
			defer w.Done()
			result[i], err = getImageFromUrl(mUrl)
		}(i, mUrl)
	}
	w.Wait()
	if err != nil {
		log.Print(err)
		return nil, err
	}
	baseImg := result[0]
	baseBound := baseImg.Bounds()
	newBound := baseBound
	// extra
	if extra.Width > 0 && extra.Height > 0 {
		newBound.Max = image.Pt(extra.Width, extra.Height)
	} else if extra.AddWidth > 0 || extra.AddHeight > 0 {
		newBound.Max = image.Pt(newBound.Max.X+extra.AddWidth, newBound.Max.Y+extra.AddHeight)
	}
	genImg := image.NewRGBA(newBound)
	blue := color.RGBA{255, 255, 255, 255}
	draw.Draw(genImg, genImg.Bounds(), &image.Uniform{blue}, image.ZP, draw.Src)
	// draw base
	draw.Draw(genImg, baseImg.Bounds(), baseImg, image.ZP, draw.Over)
	// draw sub
	for i, sub := range subImages {
		drawSub(genImg, baseBound, result[i+1], sub)
	}
	Text := op.Text
	if Text.Content != "" {
		drawText(genImg, baseBound, Text)
	}
	if len(op.Texts) > 0 {
		for _, t := range op.Texts {
			drawText(genImg, baseBound, t)
		}
	}
	bucket := GetOssBucket()
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, genImg, &jpeg.Options{90})
	if err != nil {
		return nil, err
	}
	var ossUrl, mediaIdUrl string
	w.Add(1)
	go func() {
		defer w.Done()
		filename := fmt.Sprintf("gen-imge/%d.jpg", time.Now().Unix())
		err := bucket.Put(filename, buf.Bytes(), "image/jpeg", oss.PublicRead, oss.Options{})
		if err != nil {
			log.Printf("oss error %v", err)
			ossUrl = ""
			return
		}
		ossUrl = fmt.Sprintf("%s/%s", "https://img.epeijing.cn", filename)
		log.Print(ossUrl)
	}()
	if op.Appid != "" {
		w.Add(1)
		go func() {
			defer w.Done()
			token, err := getAppidToken(op.Appid)
			if err != nil {
				log.Printf("get accesstoken error %v", err)
				mediaIdUrl = ""
				return
			}
			result, err := reqs.Upload("https://api.weixin.qq.com/cgi-bin/media/upload", utils.Query{"access_token": token, "type": "image"}, map[string]io.Reader{"media": bytes.NewReader(buf.Bytes())})
			if err != nil {
				log.Printf("media upload error %v", err)
				mediaIdUrl = ""
				return
			}
			log.Print(result)
			mediaIdUrl = result["media_id"].(string)
		}()
	}
	w.Wait()
	return &GenResult{ossUrl, mediaIdUrl}, nil
}

func drawSub(genImg *image.RGBA, baseBound image.Rectangle, subImage image.Image, meta SubImage) {
	if subImage == nil {
		return
	}
	w, h := meta.Width, meta.Height
	if w == 0 {
		w = baseBound.Size().X
	}
	if h == 0 {
		w = baseBound.Size().Y
	}
	img := imaging.Resize(subImage, w, h, imaging.Lanczos)
	var pos image.Point
	pos.X, pos.Y = meta.Left.Value, meta.Top.Value
	if meta.Left.Relative {
		// 超出base
		pos.X = baseBound.Size().X + meta.Left.Value
	}
	if meta.Top.Relative {
		// 超出base
		pos.Y = baseBound.Size().Y + meta.Top.Value
	}

	dstRetange := image.Rectangle{pos, pos.Add(image.Pt(w, h))}
	if !meta.WithArc {
		draw.Draw(genImg, dstRetange, img, image.ZP, draw.Over)
	} else {
		log.Print("draw mask")
		radis := w / 2
		draw.DrawMask(genImg, dstRetange, img, image.ZP, &circle{image.Pt(radis, radis), radis}, image.ZP, draw.Over)
	}
}
func getFont() {
	if utf8Font == nil {
		fontBytes, _ := ioutil.ReadFile(utf8FontFile)
		utf8Font, _ = freetype.ParseFont(fontBytes)

	}
}
func drawText(genImg *image.RGBA, baseBound image.Rectangle, meta TextOption) {
	getFont()
	defaultSize := float64(36.0)
	if meta.Size > 0 {
		defaultSize = float64(meta.Size)
	}
	ctx = freetype.NewContext()
	ctx.SetDPI(dpi) //screen resolution in Dots Per Inch
	ctx.SetFont(utf8Font)
	ctx.SetFontSize(defaultSize) //font size in points
	ctx.SetClip(genImg.Bounds())
	ctx.SetDst(genImg)
	fontForeGroundColor := image.NewUniform(white)
	ctx.SetSrc(fontForeGroundColor)
	var pos image.Point
	pos.X, pos.Y = meta.Left.Value, meta.Top.Value
	if meta.Left.Relative {
		// 超出base
		pos.X = baseBound.Size().X + meta.Left.Value
	}
	if meta.Top.Relative {
		// 超出base
		pos.Y = baseBound.Size().Y + meta.Top.Value
	}
	pt := freetype.Pt(pos.X, pos.Y+int(ctx.PointToFixed(defaultSize)>>6))
	ctx.DrawString(meta.Content, pt)

}

var eglassImg = []string{"img.schoolgater", "img.epeijing"}

func getImageFromUrl(url string) (image.Image, error) {
	if strings.Contains(url, "img.schoolgater") || strings.Contains(url, "img.epeijing") || strings.Contains(url, "epj-images") {
		return getImageFromOss(url)
	}
	rc, err := reqs.GetRaw(url, nil)
	if err != nil {
		return nil, err
	}
	baseImg, _, err := image.Decode(bytes.NewReader(rc))
	if err != nil {
		return nil, err
	}
	return baseImg, nil
}

func getImageFromOss(mUrl string) (image.Image, error) {
	u, urlParseError := url.Parse(mUrl)
	if urlParseError != nil {
		return nil, urlParseError
	}
	ossObject := u.Path
	log.Print(ossObject)
	oss := GetOssBucket()
	rc, err := oss.GetReader(ossObject)
	if err != nil {
		return nil, err
	}
	baseImg, _, err := image.Decode(rc)
	if err != nil {
		return nil, err
	}
	return baseImg, nil
}
