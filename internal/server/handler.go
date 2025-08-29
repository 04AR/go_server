package server

import (
	"net/http"
	"log"

	"database/sql"
	"go_server/internal/auth"
	"github.com/coder/websocket"
)

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, db *sql.DB) {

	// Simple token auth (in real app, use JWT or session)
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

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
		return
	}

	conn := NewConnection(hub, c)

	ctx := r.Context()
	// Start reader & writer goroutines
	go conn.WritePump(ctx)
	conn.ReadPump(ctx) // blocking until client disconnects
}
