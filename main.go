package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"go-server/server"

	"github.com/coder/websocket"
)

var ctx = context.Background()

func handleWS(w http.ResponseWriter, r *http.Request) {
	fmt.Println("New WebSocket connection")
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // For self-signed certificates
		// OriginPatterns:    []string{"http://localhost:*", "https://example.com:*"},
	})
	if err != nil {
		log.Printf("websocket.Accept error: %v", err)
		return
	}
	defer conn.CloseNow()


	for {
		typ, data, err := conn.Reader(ctx)
		if websocket.CloseStatus(err) == -1{
			log.Println("websocket connection closed")
			break
		}
		if err != nil {
			log.Printf("websocket.Reader error: %v", err)
		}
		log.Printf("received typ: %v", typ)
		log.Printf("received: %v", data)
	}
}

func main() {
	// InitProfiler("")
	server.InitRedis()

	http.HandleFunc("/ws", handleWS) 

	addr := ":8000"
	fmt.Println("WebSocket server listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}


