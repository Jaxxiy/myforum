package services

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // разрешить все источники (не безопасно для продакшена)
	},
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan string)
var mu sync.Mutex

func StartWebSocket() {
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()
	log.Println("WebSocket server started on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal("WebSocket server error:", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()
	mu.Lock()
	clients[ws] = true
	mu.Unlock()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			mu.Lock()
			delete(clients, ws)
			mu.Unlock()
			break
		}
		broadcast <- string(msg)
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		mu.Lock()
		for client := range clients {
			if err := client.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				client.Close()
				delete(clients, client)
			}
		}
		mu.Unlock()
	}
}
