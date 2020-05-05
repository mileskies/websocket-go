# WebSocket-Go
<p>
  <a href="#" target="_blank">
    <img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-yellow.svg" />
  </a>
</p>

WebSocket-Go is an implementation for Golang, which is a `realtime`, `fast` and `scalable` websocket(Socket.IO-like) library based on [Gorilla WebSocket](https://github.com/gorilla/websocket), [go-redis](https://github.com/go-redis/redis) and [uuid](https://github.com/google/uuid).

Compatible with [Socket.IO](https://socket.io/) js client

## Installation
Install:
```
go get github.com/mileskies/websocket-go
```

Import:
```
import "github.com/mileskies/websocket-go"
```


## Example

Requires a running `Redis` service for handling message exchange from replicas of your application runs on different machines or container.

```go
package main

import (
    "log"
    "net/http"

    "github.com/go-redis/redis/v7"
    "github.com/gorilla/websocket"
    ws "github.com/mileskies/websocket-go"
)

func main() {
    redisClient := redis.NewClient(&redis.Options{
        Addr:     "127.0.0.1:6379",
        Password: "",
        DB:       0,
    })
    if _, err := redisClient.Ping().Result(); err != nil {
        panic(err)
    }

    wsServer = ws.NewServer(redisClient)

    wsServer.On("onConnect", func(c ws.Client) error {
		fmt.Println("connected")

        c.On("msg", func(msg string) {
            fmt.Println("msg:", msg)
            c.Emit("msg", msg)
        })
        c.On("onError", func(e error) {
            fmt.Println("error:", e)
        })
        c.On("onDisconnect", func(msg string) {
            fmt.Println("disconnect:", msg)
        })
		return nil
	})

    http.HandleFunc("/socket.io/*any", func(w http.ResponseWriter, r *http.Request) {
        wsServer.ServeHTTP(w, r)
    })
    log.Fatal(http.ListenAndServe(":80", nil))
}
```

Also redis sentinel
```go
    redisClient := redis.NewFailoverClient(&redis.FailoverOptions{
        MasterName:    "master",
        SentinelAddrs: []string{":26379"},
    })
    if _, err := redisClient.Ping().Result(); err != nil {
        panic(err)
    }
```

## How to use

### Server

- Broadcast message to each client
```go
    // Broadcast(event, message)
    server.Broadcast("hello", "hello world")
```

- Broadcast message to specific room
```go
    // Broadcast(event, message, room)
    server.Broadcast("chat", "Hi!", "coffee meets")
```

- Broadcast message to each server
```go
    // BroadcastToServer(message)
    server.BroadcastToServer("Hello!")
```

- Event Listen
```go
    // On(event, func)
    server.On("onConnect", func(c ws.Client) error {
        // do something
    })
```

- Receive message from other server
```go
    // On(event, func)
    server.On("BroadcastToServer", func(msg string) {
        // do something
    })
```

### Client

- Event Listen
```go
    // On(event, func)
    client.On("hello", func(msg string) {
        // do something
    })

    client.On("onDisconnect", func(msg string) {
        // do something
    })

    client.On("onError", func(e error) {
        // do something
    })
```

- Join & Leave room
```go
    // Join(room) & Leave(room)
    client.Join("yeeeee")

    client.Leave("yeeeee")
```

- Emit message to specific room
```go
    // To(room, event, message)
    client.To("yeeeee", "hello", "hello world")
```

- Emit message
```go
    // Emit(event, message)
    client.Emit("hello", "hello world")
```

## Todo
- Testing
- Go doc

## Show your support

Give a ⭐️ if this project helped you!

## License
This Project is [MIT](LICENSE) license.

gorilla/webSocket [BSD-2-Clause](https://github.com/gorilla/websocket/blob/master/LICENSE)

go-redis/redis [BSD-2-Clause](https://github.com/go-redis/redis/blob/master/LICENSE)

uuid [BSD-3-Clause](https://github.com/google/uuid/blob/master/LICENSE)

***
_This README was generated with ❤️ by [readme-md-generator](https://github.com/kefranabg/readme-md-generator)_