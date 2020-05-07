package ws

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 0
)

type clientHandler struct {
	onDisconnect func(msg string)
	onError      func(err error)
}

// Client Structure
type Client struct {
	server  *Server
	sid     string
	conn    *websocket.Conn
	pubsub  *redis.PubSub
	events  map[string]reflect.Value
	handler *clientHandler
	Context map[string]interface{}
}

// Emit Message
func (c *Client) Emit(event string, message string, room ...string) {
	msg := []string{event, message}
	str, err := json.Marshal(msg)
	if err != nil {
		c.handler.onError(err)
		log.Error().Err(err).Msg("")
	}
	str = append([]byte{52, 50}, str...)

	r := c.sid
	if len(room) > 0 {
		r = room[0]
	}
	if err := c.server.redisClient.Publish(r, str).Err(); err != nil {
		c.handler.onError(err)
		log.Error().Err(err).Msg("")
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

// On Event Listener
func (c *Client) On(event string, handler interface{}) {
	fValue := reflect.ValueOf(handler)
	if fValue.Kind() != reflect.Func {
		panic("event handler must be a func.")
	}

	defaultEvent := [3]string{"onDisconnect", "onError"}
	for _, e := range defaultEvent {
		if e == event {
			c.events[event].Call([]reflect.Value{fValue})
			return
		}
	}
	c.events[event] = fValue
}

// OnDisconnect as disconnect handler
func (c *Client) onDisconnect(f func(string)) {
	c.handler.onDisconnect = f
}

// OnError as error handler
func (c *Client) onError(f func(error)) {
	c.handler.onError = f
}

func (c *Client) eventHandle(handler reflect.Value, args ...reflect.Value) {
	handler.Call(args)
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		c.pubsub.Close()
		c.server.disconnect <- c
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, m, err := c.conn.ReadMessage()
		if err != nil {
			c.handler.onError(err)
			log.Error().Err(err).Msg("")
			break
		}

		re := regexp.MustCompile(`\d+(.*)`)
		if num, err := strconv.Atoi(string(m)); err == nil {
			// handle status dispatch
			if num == 2 {
				c.conn.SetWriteDeadline(time.Now().Add(writeWait))
				c.conn.WriteMessage(1, []byte("3"))
			}
		} else if matchs := re.FindSubmatch(m); len(matchs) > 0 {
			req := matchs[1]
			var arr []string
			err = json.Unmarshal(req, &arr)

			if err != nil {
				log.Error().Err(err).Msg("")
				continue
			}
			event := arr[0]
			payload := arr[1]

			if handler, ok := c.events[event]; ok {
				go c.eventHandle(handler, reflect.ValueOf(payload))
			}
		}

	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		c.pubsub.Close()
		c.server.disconnect <- c
	}()

	for {
		select {
		case msg, ok := <-c.pubsub.Channel():
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(1, []byte(msg.Payload))
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.handler.onError(err)
				log.Error().Err(err).Msg("")
				return
			}
		}
	}
}
