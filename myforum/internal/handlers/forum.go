package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jaxxiy/myforum/internal/business"
	"github.com/jaxxiy/myforum/internal/repository"
)

var templates = template.Must(template.ParseGlob("C:/Users/Soulless/Desktop/myforum/templates/*.html"))

func RegisterForumHandlers(r *mux.Router, repo *repository.ForumsRepo) {
	// Обработчик для корня
	//r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	//	w.Write([]byte("Сервер работает!"))
	//})

	// Можно добавить PUT, DELETE по необходимости
	api := r.PathPrefix("/api").Subrouter()

	// CRUD для форумов
	api.HandleFunc("/forums", ListForums(repo)).Methods("GET")
	api.HandleFunc("/forums/new", NewForumForm()).Methods("GET")
	api.HandleFunc("/forums", CreateForum(repo)).Methods("POST")               // Создание
	api.HandleFunc("/forums/{id}", GetForum(repo)).Methods("GET")              // Получение по ID
	api.HandleFunc("/forums/{id}", UpdateForum(repo)).Methods("PUT")           // Обновление
	api.HandleFunc("/forums/{id}", DeleteForum(repo)).Methods("DELETE")        //Удаление темы
	api.HandleFunc("/forums/{id}/messages", GetMessages(repo)).Methods("GET")  //Просмотр всех сообщений в теме
	api.HandleFunc("/forums/{id}/messages", PostMessage(repo)).Methods("POST") //Отправка сообщения в нужную тему по ID
	api.HandleFunc("/forums/{id}/messages/{id}", DeleteMessage(repo)).Methods("DELETE")

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

		id, err := repo.Create(business.Forum{
			Title:       title,
			Description: description,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/api/forums/"+strconv.Itoa(id), http.StatusSeeOther)
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
func PostMessage(repo *repository.ForumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		forumID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.Error(w, "Invalid forum ID", http.StatusBadRequest)
			return
		}

		// Парсим форму
		err = r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Создаем сообщение
		message := business.Message{
			ForumID: forumID,
			Author:  r.FormValue("author"),
			Content: r.FormValue("content"),
		}

		// Сохраняем в БД
		_, err = repo.CreateMessage(message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Перенаправляем обратно к списку сообщений
		http.Redirect(w, r, "/api/forums/"+vars["id"]+"/messages", http.StatusSeeOther)
	}
}

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
