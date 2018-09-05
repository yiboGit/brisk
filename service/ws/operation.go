package main

import "github.com/labstack/echo"
import "log"

type ImageOperation struct {
	Base      string       `json:"base"` // 基准图片链接
	SubImages []SubImage   `json:"subImages"`
	Text      TextOption   `json:"text"`
	Texts     []TextOption `json:"texts"`
	Extra     Extra        `json:"extra"`
	Appid     string       `json:"appid"`
}
type Extra struct {
	Width     int `json:"width"`   // 指定高度
	Height    int `json:"height"`  // 指定高度
	AddWidth  int `json:"addWidth"`  // base 基础上增加宽度
	AddHeight int `json:"addHeight"` // base 基础上增加高度
}
type TextOption struct {
	Content string `json:"content"`
	Size    int    `json:"size"`
	Left    Pos    `json:"left"` // 水平偏移
	Top     Pos    `json:"top"`  // 垂直偏移
}
type SubImage struct {
	URL     string `json:"url"`     // 图片链接
	Left    Pos    `json:"left"`    // 水平偏移
	Top     Pos    `json:"top"`     // 垂直偏移
	Width   int    `json:"width"`   // 绘制的宽度， url的图片会resize到这个宽度
	Height  int    `json:"height"`  // 绘制的宽度， url的图片会resize到这个宽度
	WithArc bool   `json:"withArc"` // 是否有圆
}

type Pos struct {
	Relative bool `json:"relative"` // 是否相对于base
	Value    int  `json:"value"`    // 偏移，如果relative = true, 绝对值为 base[width|height] + value
}

func Composite(c echo.Context) error {
	var operation ImageOperation
	c.Bind(&operation)
	log.Print(operation)
	result, err := DoImageOperation(&operation)
	if err != nil {
		c.JSON(403, err.Error())
		return err
	}
	return c.JSON(200, result)
}
