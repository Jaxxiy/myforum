package services

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Общие переменные (для первого соединения)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешить все источники (не безопасно для продакшена)
	},
}

var clients = make(map[*websocket.Conn]bool) // Для первого соединения
var broadcast = make(chan string)            // Для первого соединения
var mu sync.Mutex                            // Для первого соединения

// Новые переменные для второго соединения (глобального чата)
var upgraderGlobal = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешить все источники (не безопасно для продакшена)
	},
}

var globalClients = make(map[*websocket.Conn]bool) // Для второго соединения
var globalBroadcast = make(chan string)            // Для второго соединения
var globalMu sync.Mutex                            // Для второго соединения

func StartWebSocket() {
	//Первое соединение
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()
	log.Println("WebSocket server (первое соединение) started on :8081")

	//Второе соединение
	http.HandleFunc("/ws/global", handleGlobalConnections)               // Новый обработчик
	go handleGlobalMessages()                                            // Новая горутина
	log.Println("WebSocket server (второе соединение) started on :8082") //Другой порт

	// Запускаем сервер
	go func() {
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatal("WebSocket server (первое соединение) error:", err)
		}
	}()

	if err := http.ListenAndServe(":8082", nil); err != nil {
		log.Fatal("WebSocket server (второе соединение) error:", err)
	}

}

// Обработчик для первого соединения
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

// Обработчик для второго соединения (глобального чата)
func handleGlobalConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgraderGlobal.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()

	globalMu.Lock()
	globalClients[ws] = true
	globalMu.Unlock()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			globalMu.Lock()
			delete(globalClients, ws)
			globalMu.Unlock()
			break
		}
		globalBroadcast <- string(msg)
	}
}

// Обработчик сообщений для первого соединения
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

// Обработчик сообщений для второго соединения (глобального чата)
func handleGlobalMessages() {
	for {
		msg := <-globalBroadcast
		globalMu.Lock()
		for client := range globalClients {
			if err := client.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				client.Close()
				delete(globalClients, client)
			}
		}
		globalMu.Unlock()
	}
}
