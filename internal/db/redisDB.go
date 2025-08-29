package DB

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)


func InitRedis(addr string, password string) *redis.Client {
	client := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: 0})
	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	return client
}
