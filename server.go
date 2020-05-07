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
	pubsub      *redis.PubSub
	events      map[string]reflect.Value
	handler     *serverHandler
	sync.Mutex
}

// NewServer call to Init WebSocket Server
func NewServer(redisClient *redis.Client) *Server {
	pubsub := redisClient.Subscribe("BroadcastToServer")
	server := Server{
		clients:     make(map[string]*Client),
		disconnect:  make(chan *Client),
		redisClient: redisClient,
		pubsub:      pubsub,
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
	msg := []string{event, message}
	str, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	str = append([]byte{52, 50}, str...)

	r := "ServerBroadcast"
	if len(room) > 0 {
		r = room[0]
	}

	if err := s.redisClient.Publish(r, str).Err(); err != nil {
		log.Error().Err(err).Msg("")
	}
}

// BroadcastToServer for sending msg to each server
func (s *Server) BroadcastToServer(msg string) {
	if err := s.redisClient.Publish("BroadcastToServer", msg).Err(); err != nil {
		log.Error().Err(err).Msg("")
	}
}

// On Event Listener
func (s *Server) On(event string, handler interface{}) {
	fValue := reflect.ValueOf(handler)
	fType := fValue.Type()
	if fValue.Kind() != reflect.Func {
		panic("event handler must be a func.")
	}

	defaultEvent := [3]string{"onConnect"}
	for _, e := range defaultEvent {
		if e == event {
			if fType.NumIn() < 1 || fType.In(0).Name() != "Client" {
				panic("handler function should be like func(c Client)")
			}
			s.events[event].Call([]reflect.Value{fValue})
			return
		}
	}
	if fType.NumIn() < 1 || fType.In(0).Name() != "string" {
		panic("handler function should be like func(str string)")
	}
	s.events[event] = fValue
}

// OnConnect as connection open handler
func (s *Server) onConnect(f func(Client) error) {
	s.handler.onConnect = f
}

func (s *Server) eventHandle(handler reflect.Value, args ...reflect.Value) {
	handler.Call(args)
}

// Run WebSocket Server
func (s *Server) run() {
	defer func() {
		s.pubsub.Close()
	}()
Loop:
	for {
		select {
		case msg, ok := <-s.pubsub.Channel():
			if !ok {
				log.Error().Msg("Receive msg from BroadcastToServer error.")
				break Loop
			}
			if handler, ok := s.events["BroadcastToServer"]; ok {
				go s.eventHandle(handler, reflect.ValueOf(msg.Payload))
			}
		case client := <-s.disconnect:
			if _, ok := s.clients[client.sid]; ok {
				client.handler.onDisconnect("disconnect")
				delete(s.clients, client.sid)
			}
		}
	}
}

// SetReadLimit sets the maximum size in bytes for a message read from the peer.
func (s *Server) SetReadLimit(limit int) {
	maxMessageSize = limit
}
