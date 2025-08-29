package server

type Hub struct {
	clients    map[*Connection]bool
	register   chan *Connection
	unregister chan *Connection
	broadcast  chan []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Connection]bool),
		register:   make(chan *Connection),
		unregister: make(chan *Connection),
		broadcast:  make(chan []byte),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.sendCh)
			}

		case msg := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.sendCh <- msg:
				default:
					// drop slow client
					delete(h.clients, client)
					close(client.sendCh)
				}
			}
		}
	}
}
