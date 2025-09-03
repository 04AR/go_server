package main

import (
	"os"
)

const (
	sqlDb         = "./users.db"
	dsn           = "postgres://username:password@localhost:5432/mydb" // PostgressDB credentials
	wsAddr        = ":8080"
	profilerAddr  = ":9090"
	redisAddr     = ":6379"                // Assumes running in Docker Compose network
	redisPassword = ""                     // No password set
	jwtSecretKey  = "your-very-secret-key" // CHANGE THIS in production
)

// Config holds all application-wide configuration values.
type Config struct {
	SQLDBPath     string
	PostgresDSN   string
	WSAddr        string
	ProfilerAddr  string
	RedisAddr     string
	RedisPassword string
	JWTSecretKey  string
}

// LoadConfig reads env vars with defaults for all required configs.
func LoadConfig() *Config {
	return &Config{
		SQLDBPath:     getEnv("APP_SQLITE_PATH", sqlDb),
		PostgresDSN:   getEnv("APP_POSTGRES_DSN", dsn),
		WSAddr:        getEnv("APP_WS_ADDR", wsAddr),
		ProfilerAddr:  getEnv("APP_PROFILER_ADDR", profilerAddr),
		RedisAddr:     getEnv("APP_REDIS_ADDR", redisAddr),
		RedisPassword: getEnv("APP_REDIS_PASSWORD", redisPassword),
		JWTSecretKey:  getEnv("APP_JWT_SECRET", jwtSecretKey),
	}
}

// Helper: read env or fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
