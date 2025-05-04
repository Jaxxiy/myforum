package business

import "time"

type GlobalChatMessage struct {
	ID        int       `json:"id"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type IncomingChatMessage struct {
	Author  string `json:"author"`
	Message string `json:"message"`
}
