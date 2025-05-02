package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jaxxiy/myforum/internal/business"
)

func RegisterForumHandlers(r *mux.Router) {
	r.HandleFunc("/forums", CreateForum).Methods("POST")
	r.HandleFunc("/forums/{id}", GetForum).Methods("GET")
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Сервер работает!"))
	})
}

func CreateForum(w http.ResponseWriter, r *http.Request) {
	var forum business.Forum
	if err := json.NewDecoder(r.Body).Decode(&forum); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Тут вызов бизнес-логики
	json.NewEncoder(w).Encode(forum)
}

func GetForum(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	// Тут вызов бизнес-логики
	json.NewEncoder(w).Encode(map[string]string{"id": id, "name": "Пример форума"})
}
