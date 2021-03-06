package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v7"
	ws "github.com/mileskies/websocket-go"
)

var redisClient *redis.Client
var wsServer *ws.Server

func redisInit() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})
	if _, err := redisClient.Ping().Result(); err != nil {
		panic(err)
	}
}

func wsInit() {
	wsServer = ws.NewServer(redisClient)
}

func GinMiddleware(allowOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, Content-Length, X-CSRF-Token, Token, session, Origin, Host, Connection, Accept-Encoding, Accept-Language, X-Requested-With")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Request.Header.Del("Origin")

		c.Next()
	}
}

func main() {
	router := gin.New()
	redisInit()
	wsInit()

	wsServer.On("onConnect", func(c ws.Client) error {
		fmt.Println("connected")
		fmt.Printf("context: %+v \n", c.Context)

		c.On("msg", func(msg string) {
			fmt.Println("msg:", msg)
			c.Emit("msg", msg)
			// broadcast msg to each client
			wsServer.Broadcast("msg", msg)
			// broadcast msg to each server
			wsServer.BroadcastToServer(msg)
		})
		c.On("onError", func(e error) {
			fmt.Println("error:", e)
		})
		c.On("onDisconnect", func(msg string) {
			fmt.Println("disconnect:", msg)
		})

		return nil
	})

	// listen msg by server broadcast
	wsServer.On("BroadcastToServer", func(msg string) {
		fmt.Println(msg)
	})

	router.Use(GinMiddleware("*"))
	router.GET("/socket.io/*any", wsHandler)

	router.Run(":80")
}

func wsHandler(c *gin.Context) {
	context := make(map[string]interface{})
	context["hello"] = "world"

	wsServer.ServeHTTP(c.Writer, c.Request, context)
}
