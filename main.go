package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-server/internal/auth"
	DB "go-server/internal/db"
	"go-server/internal/server"

	// "github.com/redis/go-redis/v9"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func main() {

	// Load config from env
	cfg := LoadEnv()
	sqlDb := cfg.SQLDBPath
	// dsn := cfg.PostgresDSN
	wsAddr := cfg.WSAddr
	profilerAddr := cfg.ProfilerAddr
	redisAddr := cfg.RedisAddr
	redisPassword := cfg.RedisPassword
	redisLuaScript := cfg.RedisLuaScript

	// --- Profiling Setup ---
	InitProfiler(profilerAddr) // 2 goroutines

	// --- Database Setup ---
	// Init SQLite
	log.Println("Initializing SQLite database...") // 1 goroutine
	db, err := sql.Open("sqlite", sqlDb)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	DB.InitSqlite(db)
	DB.TestDataSqlite(db)

	// Init Postgres
	// log.Println("Initializing Postgress database...")
	// db, err := sql.Open("pgx", dsn)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer db.Close()
	// DB.InitPG(db)
	// DB.TestDataPG(db)

	// Init Redis
	log.Println("Connecting to Redis...")
	rm, err := DB.InitRedis(redisAddr, redisPassword, redisLuaScript)
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	defer rm.Client.Close()
	// go rm.Listen(context.Background()) // Start Redis listener

	fmt.Println("Starting server...")
	// --- HTTP and Websocket Server Setup ---
	// Auth routes
	http.HandleFunc("/register", auth.RegisterHandler(db))
	http.HandleFunc("/login", auth.LoginHandler(db))
	http.HandleFunc("/guest", auth.GuestHandler(rm.Client))
	// WebSocket route
	log.Println("WSServer listening on", wsAddr)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWS(rm, w, r, db)
	})

	// Start server
	srv := &http.Server{Addr: wsAddr}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down Server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Shutdown:", err)
	}
}
