package msa_rpc

import (
	"fmt"
	"os"
	"reflect"
)

// BuildService 按照指定的接口与条件，生成实现接口实例的bind()，(bind的内容是：为方法绑定路由规则)；
// serviceInstance interface{} 指定接口, structName实现接口结构体名称，structVar结构体的变量名，生成文件所在路径：eg: . 表示当前文件夹路径
// * PS：serviceInterface 的参数格式，例如服务接口为UserServer 那么给定的参数格式应该为：(*UserServer)(nil)
func BuildService(serviceInstance interface{}, structName, structVar, filePath string) {
	t := reflect.TypeOf(serviceInstance).Elem()
	var s string
	for i := 0; i < t.NumMethod(); i++ {
		name := t.Method(i).Name
		var paramStr string
		if t.Method(i).Type.NumIn() == 0 {
			paramStr = `
			result := %s.%s()
			`
			paramStr = fmt.Sprintf(paramStr, structVar, name)
		} else {
			paramName := t.Method(i).Type.In(0).Name()
			paramStr = `
			var p %s
			c.Bind(&p)
			result := %s.%s(p)
			`
			paramStr = fmt.Sprintf(paramStr, paramName, structVar, name)
		}
		methodStr := ` %s.e.Add("POST","/%s/%s",func(c echo.Context) error {
			%s
			if result.Error != "" {
				return c.JSON(500,result)
			}
			return c.JSON(200,result)
		})
		`
		s = s + fmt.Sprintf(methodStr, structVar, t.Name(), name, paramStr)
	}
	fmt.Println(s)
	// 在指定目录生成文件
	filePath = fmt.Sprintf("%s/bind.go", filePath)
	f, _ := os.Create(filePath)
	defer f.Close()
	info := `
	package main
	import (
		"github.com/labstack/echo"
	)

	func (%s *%s)bind() {
		%s
	}
	`
	info = fmt.Sprintf(info, structVar, structName, s)
	f.Write([]byte(info))
}
