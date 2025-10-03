package server

import (
	"log"
	"net/http"

	"go-server/internal/auth"
	"go-server/internal/db"

	"github.com/coder/websocket"
)

func ServeWS(rm *db.RedisManager, w http.ResponseWriter, r *http.Request, authProvider auth.AuthProvider) {
	// Simple token auth (in real app, use JWT or session)
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	// Validate JWT (now returns userID + isGuest flag)
	user, err := auth.ValidateJWT(tokenStr, authProvider, rm.Client)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	log.Printf("WebSocket: Validated userID=%d, username=%s, isGuest=%v", user.ID, user.Username, user.IsGuest)

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Println("WebSocket accept error:", err)
		return
	}

	conn := NewConnection(rm, c, user)

	// Start writer goroutine
	go conn.WritePump(r.Context())
	conn.ReadPump(r.Context()) // Blocking until client disconnects
}
