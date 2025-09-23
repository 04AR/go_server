package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"go-server/internal/db"

	"github.com/coder/websocket"
)

// Message represents a WebSocket message with type, sender ID, channel, and content.
type ClientMessage struct {
	ID     string        `json:"id"`
	Type   string        `json:"type"`
	Action string        `json:"action"`
	Args   []interface{} `json:"args"`
}

type ServerResponse struct {
	ID     string      `json:"id"`
	Type   string      `json:"type"` // "response"
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type Connection struct {
	rm      *db.RedisManager
	conn    *websocket.Conn
	SendCh  chan []byte
	UserID  int  // <-- Add this
	IsGuest bool // <-- Add this
}

func NewConnection(rm *db.RedisManager, conn *websocket.Conn, userID int, isGuest bool) *Connection {
	c := &Connection{
		rm:      rm,
		conn:    conn,
		SendCh:  make(chan []byte, 16),
		UserID:  userID, // <-- Set it
		IsGuest: isGuest,
	}
	return c
}

func (c *Connection) ReadPump(ctx context.Context) {
	defer func() {
		c.conn.Close(websocket.StatusNormalClosure, "closing")
	}()

	for {
		_, msg, err := c.conn.Read(ctx)
		if err != nil {
			log.Println("read error:", err)
			break
		}

		// Parse incoming message
		var packet ClientMessage
		if err := json.Unmarshal(msg, &packet); err != nil {
			log.Println("json unmarshal error:", err)
			continue
		}

		// Dispatch based on action
		switch packet.Action {
		case "ping":
			c.sendResponse(packet.ID, map[string]string{"message": "pong"})
		default:
			// Try to call Lua script
			// Ensure client provided at least one argument (the hash key)
			if len(packet.Args) < 1 {
				c.sendError(packet.ID, "invalid_args", "missing hash key")
				continue
			}
			// First argument is the hash name (KEYS[1])
			hashKey, ok := packet.Args[0].(string)
			if !ok || hashKey == "" {
				c.sendError(packet.ID, "invalid_args", "hash key must be a non-empty string")
				continue
			}
			// The rest of the args go to ARGV
			argv := packet.Args[1:]

			// Call Lua script dynamically
			res, err := c.rm.CallScript(ctx, packet.Action, []string{hashKey}, argv...)
			if err != nil {
				c.sendError(packet.ID, "script_error", err.Error())
			} else {
				c.sendResponse(packet.ID, res)
			}
		}
	}
}

func (c *Connection) WritePump(ctx context.Context) {
	defer c.conn.Close(websocket.StatusNormalClosure, "writer closing")

	// for msg := range c.SendCh {
	// 	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	// 	err := c.conn.Write(writeCtx, websocket.MessageText, msg)
	// 	cancel()
	// 	if err != nil {
	// 		log.Println("write error:", err)
	// 		break
	// 	}
	// }
	for {
		select {
		case msg := <-c.SendCh:
			// If you need a specific deadline per write:
			// c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			err := c.conn.Write(ctx, websocket.MessageText, msg)
			if err != nil {
				log.Println("write error:", err)
				return // Use return instead of break
			}
		case <-ctx.Done(): // Check for context cancellation
			return
		}
	}
}

func (c *Connection) sendResponse(id string, result interface{}) {
	resp := ServerResponse{
		ID:     id,
		Type:   "response",
		Status: "ok",
		Result: result,
	}
	data, _ := json.Marshal(resp)
	c.SendCh <- data
}

func (c *Connection) sendError(id, code, msg string) {
	resp := ServerResponse{
		ID:     id,
		Type:   "response",
		Status: "error",
		Error:  fmt.Sprintf("%s: %s", code, msg),
	}
	data, _ := json.Marshal(resp)
	c.SendCh <- data
}
