package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"go-server/internal/auth"
	"go-server/internal/db"

	"github.com/coder/websocket"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/redis/go-redis/v9"
)

// Message represents a WebSocket message with type, sender ID, channel, and content.
type ClientMessage struct {
	ID     string        `json:"id"`
	Type   string        `json:"type"`
	Action string        `json:"action"`
	Keys   []string      `json:"keys"` // Add this
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
			lobby_id, err := gonanoid.New(5)
			if err != nil {
				c.sendError(packet.ID, "internal_error", "failed to generate lobby")
				break
			}
			// Call script to create lobby in Redis
			// KEYS:
			// KEYS[1] = "lobby:<lobbyId>"

			// ARGV:
			// ARGV[1] = lobbyId
			// ARGV[2] = maxPlayers
			_, err = c.rm.CallScript(ctx, "create_lobby", []string{"lobby:" + lobby_id}, lobby_id)
			if err != nil {
				log.Println("create_lobby script error:", err)
				c.sendError(packet.ID, "internal_error", "failed to call create_lobby script")
				break
			}
			c.sendResponse(packet.ID, map[string]string{"lobby_id": lobby_id})
		case "join_lobby":
			if len(packet.Args) < 1 {
				c.sendError(packet.ID, "missing_args", "lobby id required")
				break
			}
			lobby_id, _ := packet.Args[0].(string)
			player_id := c.user.Username

			keys := []string{
				"lobby:" + lobby_id,
				"lobby:" + lobby_id + ":players",
			}
			playerStateJson := packet.Args[1]

			_, err = c.rm.CallScript(ctx, "join_lobby", keys, lobby_id, player_id, playerStateJson)
			if err != nil {
				log.Println("join_lobby script error:", err)
				c.sendError(packet.ID, "internal_error", "failed to call join_lobby script")
				break
			}

			c.handleSubscribe(ctx, "lobby:"+lobby_id+":events")

			c.sendResponse(packet.ID, map[string]string{
				"lobby_id":  lobby_id,
				"joined_as": player_id,
			})
		default:
			// Allow multi-key Lua script calls
			if len(packet.Keys) < 1 {
				c.sendError(packet.ID, "invalid_keys", "missing Redis keys")
				continue
			}
			// Optional: Validate keys are non-empty strings
			for _, k := range packet.Keys {
				if k == "" {
					c.sendError(packet.ID, "invalid_keys", "all keys must be non-empty strings")
					continue
				}
			}
			// Call Lua script dynamically with any number of keys
			res, err := c.rm.CallScript(ctx, packet.Action, packet.Keys, packet.Args...)
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
