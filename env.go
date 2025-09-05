package main

import (
	"os"
)

const (
	sqlDb          = "./users.db"
	dsn            = "postgres://username:password@localhost:5432/mydb" // PostgressDB credentials
	wsAddr         = ":8080"
	profilerAddr   = ":9090"
	redisAddr      = ":6379"                // Assumes running in Docker Compose network
	redisPassword  = ""                     // No password set
	redisLuaScript = "./lua_scripts"        // Directory with Lua scripts
	jwtSecretKey   = "your-very-secret-key" // CHANGE THIS in production
)

// Env holds all application-wide environment values.
type Env struct {
	SQLDBPath      string
	PostgresDSN    string
	WSAddr         string
	ProfilerAddr   string
	RedisAddr      string
	RedisPassword  string
	RedisLuaScript string
	JWTSecretKey   string
}

// LoadEnv reads env vars with defaults for all required configs.
func LoadEnv() *Env {
	return &Env{
		SQLDBPath:      getEnv("APP_SQLITE_PATH", sqlDb),
		PostgresDSN:    getEnv("APP_POSTGRES_DSN", dsn),
		WSAddr:         getEnv("APP_WS_ADDR", wsAddr),
		ProfilerAddr:   getEnv("APP_PROFILER_ADDR", profilerAddr),
		RedisAddr:      getEnv("APP_REDIS_ADDR", redisAddr),
		RedisPassword:  getEnv("APP_REDIS_PASSWORD", redisPassword),
		RedisLuaScript: getEnv("APP_REDIS_LUA_SCRIPT", redisLuaScript),
		JWTSecretKey:   getEnv("APP_JWT_SECRET", jwtSecretKey),
	}
}

// Helper: read env or fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
