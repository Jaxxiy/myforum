package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/jaxxiy/myforum/internal/business"
	"github.com/jaxxiy/myforum/internal/repository"
)

var (
	templates = template.Must(template.ParseGlob("C:/Users/Soulless/Desktop/myforum/templates/*.html"))
	upgrader  = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Для разработки
		},
	}
	clients   = make(map[int]map[*websocket.Conn]bool) // forumID -> connections
	clientsMu sync.RWMutex

	//mini-chat
	globalChatClients  = make(map[*websocket.Conn]bool)
	globalChatMu       sync.RWMutex
	globalChatUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Для разработки
		},
	}
	globalChatBroadcast = make(chan GlobalChatMessage)
	globalChatHistory   []GlobalChatMessage
)

type GlobalChatMessageRequest struct {
	Author  string `json:"username"`
	Content string `json:"text"`
}

type GlobalChatMessage struct {
	Author    string    `json:"username"`
	Content   string    `json:"text"`
	CreatedAt time.Time `json:"timestamp"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func RegisterForumHandlers(r *mux.Router, repo *repository.ForumsRepo) {
	r.HandleFunc("/ws/global", serveGlobalChat)

	r.HandleFunc("/ws/{forum_id:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
		serveWebSocket(w, r)
	})
	go handleGlobalChatMessages()

	api := r.PathPrefix("/api").Subrouter()

	api.HandleFunc("/forums", ListForums(repo)).Methods("GET")
	api.HandleFunc("/forums/new", NewForumForm()).Methods("GET")
	api.HandleFunc("/forums", CreateForum(repo)).Methods("POST")
	api.HandleFunc("/forums/{id:[0-9]+}", GetForum(repo)).Methods("GET")
	api.HandleFunc("/forums/{id:[0-9]+}", UpdateForum(repo)).Methods("PUT")
	api.HandleFunc("/forums/{id:[0-9]+}", DeleteForum(repo)).Methods("DELETE")

	// Обработчики сообщений
	api.HandleFunc("/forums/{id:[0-9]+}/messages", GetMessages(repo)).Methods("GET")
	api.HandleFunc("/forums/{id:[0-9]+}/messages", PostMessage(repo)).Methods("POST")
	api.HandleFunc("/forums/{forum_id:[0-9]+}/messages/{message_id:[0-9]+}", DeleteMessage(repo)).Methods("DELETE")
	api.HandleFunc("/forums/{id:[0-9]+}/messages/{message_id:[0-9]+}", UpdateMessage(repo)).Methods("PUT")

	api.HandleFunc("/global-chat", handleGlobalChatMessage(repo)).Methods("POST")

}

// Улучшенный обработчик WebSocket
func serveWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumID, err := strconv.Atoi(vars["forum_id"])
	if err != nil {
		http.Error(w, "Invalid forum ID", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer func() {
		unregisterClient(forumID, conn)
		conn.Close()
	}()

	registerClient(forumID, conn)

	// Настройка keep-alive
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Чтение сообщений (для поддержания соединения)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// Отправка сообщения всем клиентам форума
func broadcastToForum(forumID int, message WSMessage) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()

	if conns, ok := clients[forumID]; ok {
		for conn := range conns {
			if err := conn.WriteJSON(message); err != nil {
				log.Printf("WS send error: %v", err)
				go handleFailedConnection(forumID, conn)
			}
		}
	}
}

func handleFailedConnection(forumID int, conn *websocket.Conn) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if conns, ok := clients[forumID]; ok {
		conn.Close()
		delete(conns, conn)
		log.Printf("Connection removed for forum %d", forumID)
	}
}

func registerClient(forumID int, conn *websocket.Conn) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if clients[forumID] == nil {
		clients[forumID] = make(map[*websocket.Conn]bool)
	}
	clients[forumID][conn] = true
	log.Printf("New client for forum %d. Total: %d", forumID, len(clients[forumID]))
}

func unregisterClient(forumID int, conn *websocket.Conn) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if clients[forumID] != nil {
		delete(clients[forumID], conn)
	}
}

// Улучшенный обработчик сообщений
func PostMessage(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Получаем forumID из URL
		vars := mux.Vars(r)
		forumID, err := strconv.Atoi(vars["id"])
		if err != nil {
			sendError(w, http.StatusBadRequest, "Invalid forum ID")
			return
		}

		// Проверяем Content-Type
		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			sendError(w, http.StatusBadRequest, "Content-Type must be application/json")
			return
		}

		// Декодируем JSON
		var req struct {
			Author  string `json:"author"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}

		// Валидация
		if strings.TrimSpace(req.Author) == "" || strings.TrimSpace(req.Content) == "" {
			sendError(w, http.StatusBadRequest, "Author and content are required")
			return
		}

		// Создаем сообщение
		msg := business.Message{
			ForumID:   forumID,
			Author:    req.Author,
			Content:   req.Content,
			CreatedAt: time.Now(),
		}

		fmt.Println(msg.CreatedAt)

		// Сохраняем в БД
		id, err := repo.CreateMessage(msg)
		if err != nil {
			log.Printf("DB error: %v", err)
			sendError(w, http.StatusInternalServerError, "Failed to save message")
			return
		}
		msg.ID = id

		// Отправляем через WebSocket
		go broadcastToForum(forumID, WSMessage{
			Type:    "message_created",
			Payload: msg,
		})

		// Успешный ответ
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(msg)
	}
}

func sendError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// sendWSMessage отправляет сообщение всем клиентам в указанном форуме
func sendWSMessage(forumID int, message WSMessage) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()

	if conns, ok := clients[forumID]; ok {
		for conn := range conns {
			if err := conn.WriteJSON(message); err != nil {
				log.Printf("WebSocket send error: %v", err)
				go handleFailedConnection(forumID, conn)
			}
		}
	}
}

func ListForums(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		forums, err := repo.GetAll()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderTemplate(w, "list_forums.html", map[string]interface{}{
			"Forums": forums,
		})
	}
}

func NewForumForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderTemplate(w, "new_forum.html", nil)
	}
}

// Обработчик для создания форума
func CreateForum(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		title := r.FormValue("title")
		description := r.FormValue("description")

		forum := business.Forum{
			Title:       title,
			Description: description,
			CreatedAt:   time.Now(),
		}

		id, err := repo.Create(forum)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		forum.ID = id

		// Отправляем уведомление через WebSocket
		sendWSMessage(id, WSMessage{
			Type: "forum_created",
			Payload: map[string]interface{}{
				"forum": forum,
			},
		})

		http.Redirect(w, r, "/api/forums", http.StatusSeeOther)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(forum)

	}
}

// Обработчик для получения форума по ID
func GetForum(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Некорректный ID", http.StatusBadRequest)
			return
		}
		f, err := repo.GetByID(id)
		if err != nil {
			http.Error(w, "Форум не найден", http.StatusNotFound)
			return
		}

		renderTemplate(w, "forum_detail.html", f)
	}
}

// GetAllForums возвращает все форумы
func GetAllForums(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		forums, err := repo.GetAll()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(forums)
	}
}

func UpdateForum(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, _ := strconv.Atoi(vars["id"])

		var forum business.Forum
		if err := json.NewDecoder(r.Body).Decode(&forum); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := repo.Update(id, forum); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// DeleteForum (новая функция)
func DeleteForum(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, _ := strconv.Atoi(vars["id"])

		if err := repo.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func GetMessages(repo *repository.ForumsRepo) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		forumID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.Error(w, "Invalid forum ID", http.StatusBadRequest)
			return
		}

		// Получаем форум
		forum, err := repo.GetByID(forumID)
		if err != nil {
			http.Error(w, "Forum not found", http.StatusNotFound)
			return
		}

		// Получаем сообщения
		messages, err := repo.GetMessages(forumID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Рендерим шаблон
		data := struct {
			Forum    *business.Forum
			Messages []business.Message
		}{
			Forum:    forum,
			Messages: messages,
		}

		renderTemplate(w, "message_list.html", data)
	}
}

func UpdateMessage(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем ID сообщения из URL
		vars := mux.Vars(r)
		messageID, err := strconv.Atoi(vars["message_id"])
		fmt.Println(messageID)
		if err != nil {
			http.Error(w, "Invalid message ID", http.StatusBadRequest)
			return
		}

		// Парсим тело запроса
		var request struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Обновляем сообщение в репозитории
		updatedMessage, err := repo.PutMessage(messageID, request.Content)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Возвращаем обновленное сообщение
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updatedMessage)
	}
}

// Отправка сообщения

func DeleteMessage(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		messageID, err := strconv.Atoi(vars["message_id"])
		if err != nil {
			http.Error(w, "Invalid message ID", http.StatusBadRequest)
			return
		}

		err = repo.DeleteMessage(messageID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w, tmpl, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func serveGlobalChat(w http.ResponseWriter, r *http.Request) {
	conn, err := globalChatUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Global chat WebSocket upgrade error: %v", err)
		return
	}

	log.Println("Новое WebSocket соединение установлено") // Добавляем логи

	defer func() {
		log.Println("WebSocket соединение закрыто") // Добавляем логи
		globalChatMu.Lock()
		delete(globalChatClients, conn)
		globalChatMu.Unlock()
		conn.Close()
	}()

	// Простая аутентификация (можно улучшить)

	// Регистрация клиента
	globalChatMu.Lock()
	globalChatClients[conn] = true
	globalChatMu.Unlock()

	// Отправка истории чата (только при подключении)
	globalChatMu.Lock()
	for _, msg := range globalChatHistory {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("Error sending chat history: %v", err)
			break // Прерываем отправку истории, если произошла ошибка
		}
	}
	globalChatMu.Unlock()

	//Читаем сообщения
	go func() { // Запускаем горутину для чтения сообщений
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
					log.Printf("Global chat error: %v", err)
				}
				globalChatMu.Lock()
				delete(globalChatClients, conn)
				globalChatMu.Unlock()
				conn.Close()
				break
			}
		}
	}()

}

func handleGlobalChatMessages() {
	for {
		msg := <-globalChatBroadcast // Получаем сообщение из канала
		log.Printf("Новое сообщение для рассылки (Global Chat): %+v", msg)
		globalChatMu.Lock()
		// Добавляем сообщение в историю (ограничиваем размер истории)
		globalChatHistory = append(globalChatHistory, msg)
		log.Printf("История чата: %+v", globalChatHistory) // Добавили логи
		if len(globalChatHistory) > 100 {
			globalChatHistory = globalChatHistory[1:]
		}
		for client := range globalChatClients {
			log.Printf("Отправляем сообщение клиенту (Global Chat): %+v", msg)
			err := client.WriteJSON(msg) // Отправляем сообщение клиенту
			if err != nil {
				log.Printf("Ошибка при отправке сообщения клиенту (Global Chat): %v", err)
				client.Close()
				delete(globalChatClients, client)
			}
		}
		globalChatMu.Unlock()
	}
}

// Обработчик POST-запроса для глобального чата
func handleGlobalChatMessage(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// 1. Проверяем Content-Type
		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, `{"error": "Content-Type must be application/json"}`, http.StatusBadRequest)
			return
		}

		// 2. Парсим JSON
		var req GlobalChatMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid JSON format"}`, http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// 3. Валидация
		if strings.TrimSpace(req.Author) == "" || strings.TrimSpace(req.Content) == "" {
			http.Error(w, `{"error": "Username and text are required"}`, http.StatusBadRequest)
			return
		}

		// 4. Создаем структуру business.GlobalMessage для сохранения в БД
		msgBusiness := business.GlobalMessage{
			Author:    req.Author,
			Content:   req.Content,
			CreatedAt: time.Now(),
		}

		// 5. Сохраняем в БД
		id, err := repo.CreateGlobalMessage(msgBusiness)
		if err != nil {
			log.Printf("DB error: %v", err)
			http.Error(w, `{"error": "Failed to save message"}`, http.StatusInternalServerError)
			return
		}

		// 6. Создаем структуру GlobalChatMessage для отправки в WebSocket
		msgWebSocket := GlobalChatMessage{
			Author:    req.Author,
			Content:   req.Content,
			CreatedAt: time.Now(),
		}
		log.Printf("Sending message %v to websocket", msgWebSocket)

		// 7. Отправляем в WebSocket
		globalChatBroadcast <- msgWebSocket

		// 8. Успешный ответ
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":        id,
			"username":  req.Author,
			"text":      req.Content,
			"timestamp": time.Now(),
		})

	}
}
