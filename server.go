package ws

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/go-redis/redis/v7"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type serverHandler struct {
	onConnect func(c Client) error
}

// Server Struct
type Server struct {
	clients     map[string]*Client
	disconnect  chan *Client
	redisClient *redis.Client
	events      map[string]reflect.Value
	handler     *serverHandler
	sync.Mutex
}

// NewServer call to Init WebSocket Server
func NewServer(redisClient *redis.Client) *Server {
	server := Server{
		clients:     make(map[string]*Client),
		disconnect:  make(chan *Client),
		redisClient: redisClient,
		events:      make(map[string]reflect.Value),
		handler:     &serverHandler{},
	}
	server.events["onConnect"] = reflect.ValueOf(server.onConnect)

	go server.run()
	return &server
}

// NewClient When New Client Connected
func (s *Server) NewClient(conn *websocket.Conn, context map[string]interface{}) *Client {
	sid := uuid.New().String()
	pubsub := s.redisClient.Subscribe(sid, "ServerBroadcast")
	client := &Client{
		server:  s,
		sid:     sid,
		conn:    conn,
		pubsub:  pubsub,
		events:  make(map[string]reflect.Value),
		handler: &clientHandler{},
		Context: context,
	}
	client.events["onDisconnect"] = reflect.ValueOf(client.onDisconnect)
	client.events["onError"] = reflect.ValueOf(client.onError)
	s.Lock()
	defer s.Unlock()
	s.clients[sid] = client
	go client.writePump()
	go client.readPump()

	conn.WriteMessage(1, []byte("0{\"sid\":\""+sid+"\",\"upgrades\":[],\"pingInterval\":30000,\"pingTimeout\":60000}"))
	conn.WriteMessage(1, []byte("40"))
	s.handler.onConnect(*client)

	return client
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request, context ...map[string]interface{}) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("can't connect socker server.")
		return
	}

	go func() {
		c := make(map[string]interface{})
		if len(context) > 0 {
			c = context[0]
		}
		s.NewClient(conn, c)
	}()
}

// Broadcast Message to Each Client From Server
func (s *Server) Broadcast(event string, message string, room ...string) {
	msg := make(map[string]string)
	msg["event"] = event
	msg["payload"] = message
	str, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err)
	}

	r := "ServerBroadcast"
	if len(room) > 0 {
		r = room[0]
	}

	if err := s.redisClient.Publish(r, str).Err(); err != nil {
		log.Error().Err(err)
	}
}

// On Event Listener
func (s *Server) On(event string, handler interface{}) {
	fValue := reflect.ValueOf(handler)
	if fValue.Kind() != reflect.Func {
		panic("event handler must be a func.")
	}
	fType := fValue.Type()
	if fType.NumIn() < 1 || fType.In(0).Name() != "Client" {
		panic("handler function should be like func(c Client)")
	}

	defaultEvent := [3]string{"onConnect"}
	for _, e := range defaultEvent {
		if e == event {
			s.events[event].Call([]reflect.Value{fValue})
			return
		}
	}
}

// OnConnect as connection open handler
func (s *Server) onConnect(f func(Client) error) {
	s.handler.onConnect = f
}

// Run WebSocket Server
func (s *Server) run() {
	for {
		select {
		case client := <-s.disconnect:
			if _, ok := s.clients[client.sid]; ok {
				client.handler.onDisconnect("disconnect")
				delete(s.clients, client.sid)
			}
		}
	}
}
