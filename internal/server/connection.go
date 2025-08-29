package server

import (
	"context"
	"log"
	"time"

	"github.com/coder/websocket"
)

type Connection struct {
	hub    *Hub
	conn   *websocket.Conn
	sendCh chan []byte
}

func NewConnection(hub *Hub, conn *websocket.Conn) *Connection {
	c := &Connection{
		hub:    hub,
		conn:   conn,
		sendCh: make(chan []byte, 16),
	}
	hub.register <- c
	return c
}

func (c *Connection) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "closing")
	}()

	for {
		_, msg, err := c.conn.Read(ctx)
		if err != nil {
			log.Println("read error:", err)
			break
		}
		// Broadcast incoming message to all
		c.hub.broadcast <- msg
	}
}

func (c *Connection) WritePump(ctx context.Context) {
	defer c.conn.Close(websocket.StatusNormalClosure, "writer closing")

	for msg := range c.sendCh {
		writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.conn.Write(writeCtx, websocket.MessageText, msg)
		cancel()
		if err != nil {
			log.Println("write error:", err)
			break
		}
	}
}
