package utils

import (
	"log"
	"os"
	"runtime"

	"upper.io/db.v3"
	"upper.io/db.v3/lib/sqlbuilder"
	"upper.io/db.v3/mysql"
)

// MysqlLogger db.v3 数据库日志的封装
type MysqlLogger struct {
	log     *log.Logger
	logging bool
}

// Log 实现 db.v3的Log接口
func (logger MysqlLogger) Log(q *db.QueryStatus) {
	if q.Err != nil {
		logger.log.Println(q.Err)
		logger.log.Println(q)
		stack := make([]byte, 2048)
		length := runtime.Stack(stack, false)
		logger.log.Printf("[mysql error] %v %s\n", q.Err, stack[:length])
	} else if logger.logging {
		logger.log.Println(q)
	}
}

// NewMysqlLogger 初始化
func NewMysqlLogger(logging bool) MysqlLogger {
	return MysqlLogger{log: log.New(os.Stderr, "[mysql query] ", 1), logging: logging}
}

// NewMysql 获得mysql
func NewMysql(isProd, logging bool) (sqlbuilder.Database, error) {
	str := "root:epeijing@tcp(t.epeijing.cn:3306)/db_new?charset=utf8mb4&loc=Asia%2FShanghai"
	if isProd {
		str = "epeijing:B0vZxXWV@tcp(rm-uf6th65g631q9kc94.mysql.rds.aliyuncs.com:3306)/db_new?charset=utf8mb4&loc=Asia%2FShanghai"
	}
	log.Printf("connecting to mysql: %s\n", str)
	setting, error := mysql.ParseURL(str)
	if error != nil {
		panic("mysqlStr error")
	}
	sess, error := mysql.Open(setting)
	if error != nil {
		return nil, error
	}
	sess.SetLogging(true)
	sess.SetLogger(NewMysqlLogger(logging))
	return sess, nil
}
func NewPuruiMysql(isProd, logging bool) (sqlbuilder.Database, error) {
	str := "root:epeijing@tcp(t.epeijing.cn:3306)/purui?charset=utf8mb4&loc=Asia%2FShanghai"
	if isProd {
		str = "epeijing:B0vZxXWV@tcp(rm-uf6th65g631q9kc94.mysql.rds.aliyuncs.com:3306)/purui?charset=utf8mb4&loc=Asia%2FShanghai"
	}
	log.Printf("connecting to mysql: %s\n", str)
	setting, error := mysql.ParseURL(str)
	if error != nil {
		panic("mysqlStr error")
	}
	sess, error := mysql.Open(setting)
	if error != nil {
		return nil, error
	}
	sess.SetLogging(true)
	sess.SetLogger(NewMysqlLogger(logging))
	return sess, nil
}


