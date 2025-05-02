package business

import "time"

type Message struct {
	ID        int       `json:"id"`
	ForumID   int       `json:"forum_id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
