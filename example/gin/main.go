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
		return nil
	})
	wsServer.On("msg", func(c ws.Client, msg string) {
		fmt.Println("msg:", msg)
		c.Emit("msg", msg)
	})
	wsServer.On("onError", func(c ws.Client, e error) {
		fmt.Println("error:", e)
	})
	wsServer.On("onDisconnect", func(c ws.Client, msg string) {
		fmt.Println("disconnect:", msg)
	})

	router.Use(GinMiddleware("*"))
	router.GET("/socket.io/*any", gin.WrapH(wsServer))

	router.Run(":80")
}
