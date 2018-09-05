package utils

import (
	"log"

	"github.com/go-redis/redis"
)

func NewRedisClient(isProd bool) *redis.Client {
	addr := "t.epeijing.cn:6380"
	if isProd {
		addr = "epeijing.cn:6379"
	}
	log.Printf("connecting to redis: %s ", addr)

	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "epj2016666",
		DB:       0,
	})
}
func GetHost(isProd bool) string {
	host := "epeijing.cn"
	if isProd {
		return host
	}
	return "t." + host
}
