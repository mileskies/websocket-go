# WebSocket-Go
<p>
  <a href="#" target="_blank">
    <img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-yellow.svg" />
  </a>
</p>

WebSocket-Go is an implementation for Golang, which is a `realtime`, `fast` and `scalable` websocket(Socket.IO-like) library based on [Gorilla WebSocket](https://github.com/gorilla/websocket), [go-redis](https://github.com/go-redis/redis) and [uuid](https://github.com/google/uuid).


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

```
package main

import (
    "log"
    "net/http"

    "github.com/go-redis/redis/v7"
    "github.com/gorilla/websocket"
    "github.com/mileskies/websocket-go"
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

    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        WSHandler(wsServer, w, r)
    })
    log.Fatal(http.ListenAndServe(":80", nil))
}

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func WSHandler(server *ws.Server, w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    client := server.NewClient(conn)
}
```

Also redis sentinel
```
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
```
    // Broadcast(event, message)
    server.Broadcast("hello", "hello world")
```

### Client

- Event Listen
```
    // On(event, func)
    client.On("hello", func(c ws.Client, msg string) {
        // do something
    })
```

- Join & Leave room
```
    // Join(room) & Leave(room)
    client.Join("yeeeee")

    client.Leave("yeeeee")
```

- Emit message to specific room
```
    // To(room, event, message)
    client.To("yeeeee", "hello", "hello world")
```

- Emit message
```
    // Emit(event, message)
    client.Emit("hello", "hello world")
```

## Features
- Fully compatible with [Socket.IO](https://socket.io/) js client

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