just learning to use Go by building the websocket server

# Go WebSocket Server with Redis + sqlite and postgres Backend

🚀 A lightweight and scalable **WebSocket server written in Go** that supports **real-time synchronization** between clients.  
It uses:

- **Redis** → for state replication, broadcasting, and real-time pub/sub.  
- **PostgreSQL or SQLite** → for persistent storage (user accounts, auth, metadata).  
- **JWT authentication** → for secure client identification.  

This setup is ideal for:
- Any real-time service that requires persistence + fast sync ⚡  

---

## ✨ Features

- 🔌 **WebSocket-based communication** (bidirectional, low latency).  
- 🗄️ **Pluggable database layer** → choose between **Postgres** or **SQLite**.  
- 📡 **Redis Pub/Sub** → ensures all clients across multiple servers stay in sync.  
- 🔒 **JWT-based authentication** → secure and stateless.  
- 🛠️ **Thin abstraction layer** → easy to extend with custom game/app logic.  
- ⚡ **Goroutine-based concurrency** → scales well with thousands of connections.  

---


