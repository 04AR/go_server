package server

import (
	"log"
	"net/http"

	"go-server/internal/auth"
	"go-server/internal/db"

	"github.com/coder/websocket"
	"github.com/jmoiron/sqlx"
)

func ServeWS(rm *db.RedisManager, w http.ResponseWriter, r *http.Request, db *sqlx.DB) {
	// Simple token auth (in real app, use JWT or session)
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	// Validate JWT (now returns userID + isGuest flag)
	userID, isGuest, err := auth.ValidateJWT(tokenStr, db, rm.Client)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	if isGuest {
		log.Printf("Guest user %d connected via WebSocket", userID)
	} else {
		log.Printf("Registered user %d connected via WebSocket", userID)
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Println("WebSocket accept error:", err)
		return
	}

	conn := NewConnection(rm, c, userID, isGuest)

	// Start writer goroutine
	go conn.WritePump(r.Context())
	conn.ReadPump(r.Context()) // Blocking until client disconnects
}
