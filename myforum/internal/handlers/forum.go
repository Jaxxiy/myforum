package handlers

import (
	"encoding/json"
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
			return true
		},
	}
	clients   = make(map[int]map[*websocket.Conn]bool)
	clientsMu sync.RWMutex
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
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

// handleFailedConnection обрабатывает неудачные соединения
func handleFailedConnection(forumID int, conn *websocket.Conn) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if conns, ok := clients[forumID]; ok {
		conn.Close()
		delete(conns, conn)
		log.Printf("Connection removed for forum %d", forumID)
	}
}

func RegisterForumHandlers(r *mux.Router, repo *repository.ForumsRepo) {
	r.HandleFunc("/ws/{forum_id:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
		serveWebSocket(repo, w, r)
	})

	api := r.PathPrefix("/api").Subrouter()

	// CRUD для форумов
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
}

// PostMessage - создание нового сообщения
func PostMessage(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		// Получаем forumID из URL
		vars := mux.Vars(r)
		forumID, err := strconv.Atoi(vars["id"])
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid forum ID"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Проверяем Content-Type
		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			json.NewEncoder(w).Encode(map[string]string{"error": "Content-Type must be application/json"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Декодируем JSON
		var req struct {
			Author  string `json:"author"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON format"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Валидация
		req.Author = strings.TrimSpace(req.Author)
		req.Content = strings.TrimSpace(req.Content)
		if req.Author == "" || req.Content == "" {
			json.NewEncoder(w).Encode(map[string]string{"error": "Author and content are required"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Создаем сообщение
		msg := business.Message{
			ForumID:   forumID,
			Author:    req.Author,
			Content:   req.Content,
			CreatedAt: time.Now().UTC(),
		}

		// Сохраняем в БД
		id, err := repo.CreateMessage(msg)
		if err != nil {
			log.Printf("Database error: %v", err)

			errorMsg := "Failed to save message"
			if strings.Contains(err.Error(), "not found") {
				errorMsg = "Forum not found"
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}

			json.NewEncoder(w).Encode(map[string]string{"error": errorMsg})
			return
		}
		msg.ID = id

		// Отправляем через WebSocket
		go sendWSMessage(forumID, WSMessage{
			Type:    "message_created",
			Payload: msg,
		})

		// Успешный ответ
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(msg)
	}
}

// sendJSONError - отправка ошибки в JSON формате
func sendJSONError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// serveWebSocket - обработчик WebSocket соединений
func serveWebSocket(repo *repository.ForumsRepo, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumID, err := strconv.Atoi(vars["forum_id"])
	if err != nil {
		http.Error(w, "Invalid forum ID", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}
	defer unregisterClient(forumID, conn)

	registerClient(forumID, conn)

	// Настройка keep-alive
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Чтение входящих сообщений (для поддержания соединения)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// registerClient - регистрация нового клиента WebSocket
func registerClient(forumID int, conn *websocket.Conn) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if clients[forumID] == nil {
		clients[forumID] = make(map[*websocket.Conn]bool)
	}
	clients[forumID][conn] = true
	log.Printf("New client connected to forum %d", forumID)
}

// unregisterClient - удаление клиента WebSocket
func unregisterClient(forumID int, conn *websocket.Conn) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if clients[forumID] != nil {
		delete(clients[forumID], conn)
		log.Printf("Client disconnected from forum %d", forumID)
	}
	conn.Close()
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

// Отправка сообщения

func DeleteMessage(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		messageID, err := strconv.Atoi(vars["id"])
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
