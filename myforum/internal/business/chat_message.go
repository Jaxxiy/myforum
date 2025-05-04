package business

import "time"

type IncomingChatMessage struct {
	Author  string `json:"author"`
	Message string `json:"message"`
}

type GlobalMessage struct {
	ID        int       `json:"id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
