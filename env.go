package main

import (
	"os"
)

const (
	wsAddr             = ":8080"
	profilerAddr       = ":9090"
	redisAddr          = ":6379"         // Assumes running in Docker Compose network
	redisPassword      = ""              // No password set
	redisLuaScriptPath = "./lua_scripts" // Directory with Lua scripts
)

// Env holds all application-wide environment values.
var (
	WSAddr             string
	ProfilerAddr       string
	RedisAddr          string
	RedisPassword      string
	RedisLuaScriptPath string
)

// LoadEnv reads env vars with defaults for all required configs.
func init() {
	WSAddr = getEnv("APP_WS_ADDR", wsAddr)
	ProfilerAddr = getEnv("APP_PROFILER_ADDR", profilerAddr)
	RedisAddr = getEnv("APP_REDIS_ADDR", redisAddr)
	RedisPassword = getEnv("APP_REDIS_PASSWORD", redisPassword)
	RedisLuaScriptPath = getEnv("APP_REDIS_LUA_SCRIPT_Path", redisLuaScriptPath)
}

// Helper: read env or fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
