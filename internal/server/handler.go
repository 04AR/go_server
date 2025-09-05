package server

import (
	"log"
	"net/http"

	"database/sql"
	"go-server/internal/auth"
	"go-server/internal/db"

	"github.com/coder/websocket"
)

func ServeWS(rm *db.RedisManager, w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Simple token auth (in real app, use JWT or session)
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}
	// Validate JWT
	userID, err := auth.ValidateJWT(tokenStr, db)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	log.Printf("User %d connected via WebSocket", userID)

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Println("WebSocket accept error:", err)
		return
	}

	conn := NewConnection(rm, c)

	// Start writer goroutine
	go conn.WritePump(r.Context())
	conn.ReadPump(r.Context()) // Blocking until client disconnects
}
