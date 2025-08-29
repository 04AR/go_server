package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"net/http"
	"time"

	"go_server/internal/server"
	"go_server/internal/db"
	"go_server/internal/auth"

	// "github.com/redis/go-redis/v9"
	"database/sql"
	_ "modernc.org/sqlite"
)

const (
	dbFile         = "./users.db"
	wsAddr		   = ":8080"
	profilerAddr   = ":9090"
	redisAddr      = ":6379" // Assumes running in Docker Compose network
	redisPassword  = ""      // No password set
	jwtSecretKey   = "your-very-secret-key" // CHANGE THIS in production
)

func main() {
	// --- Profiling Setup ---
	InitProfiler(profilerAddr) // 2 goroutines

	// --- Database Setup ---
	// Init SQLite
	log.Println("Initializing SQLite database...") // 1 goroutine
	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	DB.InitSqlite(db)
	DB.TestData(db)

	// Init Redis
	log.Println("Connecting to Redis...")
	redisClient := DB.InitRedis(redisAddr, redisPassword)
	defer redisClient.Close()

	fmt.Println("Starting server...")
	// --- HTTP and Websocket Server Setup ---
	log.Println("WSServer listening on", wsAddr)

	hub := server.NewHub()
	go hub.Run() // start hub in background  // 1 goroutine

	// Auth routes
	http.HandleFunc("/register", auth.RegisterHandler(db))
	http.HandleFunc("/login", auth.LoginHandler(db))
	http.HandleFunc("/guest", auth.GuestHandler(redisClient))
	// WebSocket route
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWS(hub, w, r, db)
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

