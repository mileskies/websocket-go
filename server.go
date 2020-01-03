package ws

import (
	"encoding/json"
	"reflect"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/go-redis/redis/v7"
	"github.com/gorilla/websocket"
)

// Server Struct
type Server struct {
	clients     map[string]*Client
	disconnect  chan string
	redisClient *redis.Client
	sync.Mutex
}

// NewServer call to Init WebSocket Server
func NewServer(redisClient *redis.Client) *Server {
	server := Server{
		clients:     make(map[string]*Client),
		disconnect:  make(chan string),
		redisClient: redisClient,
	}
	go server.run()
	return &server
}

// NewClient When New Client Connected
func (server *Server) NewClient(conn *websocket.Conn) *Client {
	uid := uuid.New().String()
	pubsub := server.redisClient.Subscribe(uid, "ServerBroadcast")
	client := &Client{server: server, uid: uid, conn: conn, pubsub: pubsub, events: make(map[string]reflect.Value)}
	server.Lock()
	defer server.Unlock()
	server.clients[uid] = client
	go client.writePump()
	go client.readPump()

	return client
}

// Broadcast Message to Each Client From Server
func (server *Server) Broadcast(event string, message string) {
	msg := make(map[string]string)
	msg["event"] = event
	msg["payload"] = message
	str, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err)
	}

	if err := server.redisClient.Publish("ServerBroadcast", str).Err(); err != nil {
		log.Error().Err(err)
	}
}

// Run WebSocket Server
func (server *Server) run() {
	for {
		select {
		case client := <-server.disconnect:
			if _, ok := server.clients[client]; ok {
				delete(server.clients, client)
			}
		}
	}
}
