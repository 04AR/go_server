package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"go-server/internal/auth"
	"go-server/internal/db"

	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

// Message represents a WebSocket message with type, sender ID, channel, and content.
type ClientMessage struct {
	ID     string        `json:"id"`
	Type   string        `json:"type"`
	Action string        `json:"action"`
	Args   []interface{} `json:"args"`
}

type ServerResponse struct {
	ID     string      `json:"Id"`
	Type   string      `json:"Type"` // "response"
	Status string      `json:"Status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type Connection struct {
	rm        *db.RedisManager
	conn      *websocket.Conn
	SendCh    chan []byte
	user      auth.User
	subClient *redis.Client
	pubsub    *redis.PubSub
	// UserID  int  // <-- Add this
	// IsGuest bool // <-- Add this
}

func NewConnection(rm *db.RedisManager, conn *websocket.Conn, user auth.User) *Connection {
	subClient := redis.NewClient(&redis.Options{
		Addr:     rm.Client.Options().Addr,
		Password: rm.Client.Options().Password,
		DB:       0, // use default DB
	})
	c := &Connection{
		rm:        rm,
		conn:      conn,
		SendCh:    make(chan []byte, 16),
		user:      user,
		subClient: subClient,
		// UserID:  userID, // <-- Set it
		// IsGuest: isGuest,
	}
	return c
}

func (c *Connection) handleSubscribe(ctx context.Context, roomID string) {
	if c.pubsub == nil {
		c.pubsub = c.subClient.Subscribe(ctx, roomID)
		go c.listenPubSub(ctx)
	} else {
		err := c.pubsub.Subscribe(ctx, roomID)
		if err != nil {
			c.sendError("", "subscribe", err.Error())
			return
		}
	}
	c.sendResponse("", map[string]string{"subscribed": roomID})
}

func (c *Connection) handleUnsubscribe(ctx context.Context, roomID string) {
	if c.pubsub != nil {
		err := c.pubsub.Unsubscribe(ctx, roomID)
		if err != nil {
			c.sendError("", "unsubscribe", err.Error())
			return
		}
		c.sendResponse("", map[string]string{"unsubscribed": roomID})
	}
}

func (c *Connection) listenPubSub(ctx context.Context) {
	ch := c.pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			if msg == nil {
				return
			}
			c.SendCh <- []byte(msg.Payload)
		case <-ctx.Done():
			return
		}
	}
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
		case "subscribe":
			if len(packet.Args) < 1 {
				c.sendError(packet.ID, "missing_args", "room id required")
				break
			}
			roomID, _ := packet.Args[0].(string)
			c.handleSubscribe(ctx, roomID)
		case "unsubscribe":
			if len(packet.Args) < 1 {
				c.sendError(packet.ID, "missing_args", "room id required")
				break
			}
			roomID, _ := packet.Args[0].(string)
			c.handleUnsubscribe(ctx, roomID)
		case "create_lobby":
			// Special case for lobby id creation: just return some info
			id, err := gonanoid.New(5)
			if err != nil {
				c.sendError(packet.ID, "internal_error", "failed to generate lobby")
				break
			}
			// Call script
			_, err = c.rm.CallScript(ctx, "create_lobby", []string{id}, id, c.user.Username, 10, "{}")
			if err != nil {
				log.Println("create_lobby script error:", err)
				c.sendError(packet.ID, "internal_error", "failed to call create_lobby script")
				break
			}
			c.sendResponse(packet.ID, map[string]string{"lobby_id": id})
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
	data, err := json.Marshal(resp)
	log.Println("Sending response:", string(data))
	if err != nil {
		log.Println("json marshal error:", err)
		return
	}
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
