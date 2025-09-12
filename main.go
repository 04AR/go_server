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

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func main() {
	// --- Profiling Setup ---
	// InitProfiler(ProfilerAddr) // 2 goroutines

	// --- Database Setup ---
	// Initialize database and AuthProvider
	log.Println("Initialize database and AuthProvider...")
	db, authProvider, err := DB.InitDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Init Redis
	log.Println("Connecting to Redis...")
	rm, err := DB.InitRedis(RedisAddr, RedisPassword, RedisLuaScriptPath)
	if err != nil {
		log.Fatal("Error connecting to Redis:", err)
	}
	defer rm.Client.Close()
	// go rm.Listen(context.Background()) // Start Redis listener

	fmt.Println("Starting server...")
	// --- HTTP and Websocket Server Setup ---
	// Auth routes
	http.HandleFunc("/register", auth.RegisterHandler(authProvider))
	http.HandleFunc("/login", auth.LoginHandler(authProvider))
	http.HandleFunc("/guest", auth.GuestHandler(rm.Client))
	// WebSocket route
	log.Println("WSServer listening on", WSAddr)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWS(rm, w, r, authProvider)
	})

	// Start server
	srv := &http.Server{Addr: WSAddr}
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
	rm.Shutdown(ctx, true) // Shutdown Redis listeners
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Shutdown:", err)
	}
}
