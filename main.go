package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	nats "github.com/nats-io/go-nats"
)

var upgrader = websocket.Upgrader{}
var clientsMutex = sync.Mutex{}
var clients = make(map[*websocket.Conn]struct{})

func main() {
	conn, err := nats.Connect(os.Getenv("NATS_URI"))
	if err != nil {
		panic(err)
	}

	if _, err = conn.Subscribe("hm.agent.heartbeat.*", relayHeartbeat); err != nil {
		panic(err)
	}

	http.HandleFunc("/ws", acceptWebsocket)
	http.Handle("/", http.FileServer(http.Dir("docroot")))

	if err = http.ListenAndServe(":8000", nil); err != nil {
		panic(err)
	}
}

func relayHeartbeat(msg *nats.Msg) {
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"heartbeat","payload":%s}`, msg.Data)))
		if err != nil {
			log.Println(err)
			client.Close()
		}
	}
}

func acceptWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	clientsMutex.Lock()
	clients[conn] = struct{}{}
	clientsMutex.Unlock()

	defer func() {
		clientsMutex.Lock()
		delete(clients, conn)
		clientsMutex.Unlock()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
