### msa_rpc 使用介绍 & 案例

####build_client.go --- 自动生成服务间调用所需的client端
对外提供的方法：
*BuildClient()* 自动生成服务间通信所需要的被调用服务的client端，生成之后，只需要调用client端内的方法，即可完成服务间通信。
* 所需要的参数：
- [x] serviceInterface interface{}指定接口，参数格式：服务接口为UserServer 那么给定的参数格式应该为：(*UserServer)(nil)
- [x] clientVar client端结构体变量名
- [x] serviceName服务对外暴露时使用的服务名
- [x] filePath string 生成文件所在路径：eg：. 表示当前文件夹路径


####build_service.go --- 自动生成服务路由绑定规则

对外提供的方法：    
 *BuildService()* 自动生成服务端所必须的路由规则，封装入bind方法中，使用时调用bind方法，进行绑定.  
* 所需要的参数：    
- [x] serviceInstance interface{} 指定接口,参数格式：服务接口为UserServer 那么给定的参数格式应该为：(*UserServer)(nil)   
- [x] structName实现接口结构体名称  
- [x] structVar实现接口结构体的变量名  
- [x] 生成文件所在路径：eg: . 表示当前文件夹路径  
 

使用方式，建立main.go，使用期main() 调用以上两种方法，传入参数，进行运行，即可将目标文件生成到指定文件夹内
*eg:*    
例如目标服务接口：user_interface.go；
main.go中调用自动生成方法。
进入当前路径的终端 使用命令：go run main.go user_interface.go  即可在指定路径下，生成所需要的文件。