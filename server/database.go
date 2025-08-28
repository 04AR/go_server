package server

import (
	"context"
	"log"
	"fmt"
	// "os"

	"github.com/redis/go-redis/v9"
)


func InitRedis() {
	var rdb *redis.Client
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Default Redis address
		Password: "",               // No password by default
		DB:       0,               // Default DB
	})

	// Test Redis connection
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Println("Failed to connect to Redis: %v", err)
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
		// os.Exit(0) // Exit if Redis connection fails
	}
	log.Println("Connected to Redis successfully")
}
