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

type defaultHandler struct {
	onConnect    func(c Client) error
	onDisconnect func(c Client, msg string)
	onError      func(c Client, err error)
}

// Server Struct
type Server struct {
	connChan    chan *websocket.Conn
	clients     map[string]*Client
	disconnect  chan *Client
	redisClient *redis.Client
	events      map[string]reflect.Value
	handler     *defaultHandler
	sync.Mutex
}

// NewServer call to Init WebSocket Server
func NewServer(redisClient *redis.Client) *Server {
	server := Server{
		connChan:    make(chan *websocket.Conn),
		clients:     make(map[string]*Client),
		disconnect:  make(chan *Client),
		redisClient: redisClient,
		events:      make(map[string]reflect.Value),
		handler:     &defaultHandler{},
	}
	server.events["onConnect"] = reflect.ValueOf(server.onConnect)
	server.events["onDisconnect"] = reflect.ValueOf(server.onDisconnect)
	server.events["onError"] = reflect.ValueOf(server.onError)
	go server.run()
	return &server
}

// NewClient When New Client Connected
func (s *Server) NewClient(conn *websocket.Conn) *Client {
	uid := uuid.New().String()
	pubsub := s.redisClient.Subscribe(uid, "ServerBroadcast")
	client := &Client{server: s, uid: uid, conn: conn, pubsub: pubsub}
	s.Lock()
	defer s.Unlock()
	s.clients[uid] = client
	go client.writePump()
	go client.readPump()

	conn.WriteMessage(1, []byte("0{\"sid\":\""+uid+"\",\"upgrades\":[],\"pingInterval\":30000,\"pingTimeout\":60000}"))
	conn.WriteMessage(1, []byte("40"))
	s.handler.onConnect(*client)

	return client
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("can't connect socker server.")
		return
	}

	go func() {
		s.connChan <- conn
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
		panic("handler function should be like func(c Client, msg string)")
	}

	defaultEvent := [3]string{"onConnect", "onDisconnect", "onError"}
	for _, e := range defaultEvent {
		if e == event {
			s.events[event].Call([]reflect.Value{fValue})
			return
		}
	}

	s.events[event] = fValue
}

// OnConnect as connection open handler
func (s *Server) onConnect(f func(Client) error) {
	s.handler.onConnect = f
}

// OnDisconnect as disconnect handler
func (s *Server) onDisconnect(f func(Client, string)) {
	s.handler.onDisconnect = f
}

// OnError as error handler
func (s *Server) onError(f func(Client, error)) {
	s.handler.onError = f
}

// Run WebSocket Server
func (s *Server) run() {
	for {
		select {
		case c := <-s.connChan:
			s.NewClient(c)
		case client := <-s.disconnect:
			if _, ok := s.clients[client.uid]; ok {
				s.handler.onDisconnect(*client, "disconnect")
				delete(s.clients, client.uid)
			}
		}
	}
}
