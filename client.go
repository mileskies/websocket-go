package ws

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// Client Structure
type Client struct {
	server *Server
	uid    string
	conn   *websocket.Conn
	pubsub *redis.PubSub
	events map[string]reflect.Value
}

// Message Struct
type Message struct {
	Event   string `json:"event"`
	Payload string `json:"payload"`
}

// On Event Listener
func (c *Client) On(event string, handler interface{}) {
	fValue := reflect.ValueOf(handler)
	if fValue.Kind() != reflect.Func {
		panic("event handler must be a func.")
	}
	fType := fValue.Type()
	if fType.NumIn() < 1 || fType.In(0).Name() != "Client" {
		panic("handler function should be like func(c Client, msg string)")
	}

	c.events[event] = fValue
}

// Emit Message
func (c *Client) Emit(event string, message string, room ...string) {
	msg := make(map[string]string)
	msg["event"] = event
	msg["payload"] = message
	str, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err)
	}

	r := c.uid
	if len(room) > 0 {
		r = room[0]
	}
	if err := c.server.redisClient.Publish(r, str).Err(); err != nil {
		log.Error().Err(err)
	}
}

// Join Room
func (c *Client) Join(room string) {
	c.pubsub.Subscribe(room)
}

// Leave Room
func (c *Client) Leave(room string) {
	c.pubsub.Unsubscribe(room)
}

// To Emit Message on Specific Room
func (c *Client) To(room string, event string, message string) {
	c.Emit(event, message, room)
}

func (c *Client) eventHandle(handler reflect.Value, msg string) {
	args := append([]reflect.Value{
		reflect.ValueOf(*c),
		reflect.ValueOf(msg),
	})
	handler.Call(args)
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		c.pubsub.Close()
		c.server.disconnect <- c.uid
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			log.Error().Err(err)
			break
		}

		if handler, ok := c.events[msg.Event]; ok {
			go c.eventHandle(handler, msg.Payload)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		c.pubsub.Close()
		c.server.disconnect <- c.uid
	}()

	for {
		select {
		case msg, ok := <-c.pubsub.Channel():
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteJSON(msg.Payload)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
