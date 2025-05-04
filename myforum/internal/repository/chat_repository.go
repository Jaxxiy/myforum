package repository

import (
	"database/sql"
	"time"

	"github.com/jaxxiy/myforum/internal/business"
)

type ChatRepository struct {
	db *sql.DB
}

func NewChatRepository(db *sql.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) SaveMessage(msg business.GlobalChatMessage) (int, error) {
	var id int
	err := r.db.QueryRow(
		`INSERT INTO global_chat (author, message, created_at) 
         VALUES ($1, $2, $3) RETURNING id`,
		msg.Author, msg.Message, time.Now(),
	).Scan(&id)
	return id, err
}

func (r *ChatRepository) GetRecentMessages(limit int) ([]business.GlobalChatMessage, error) {
	rows, err := r.db.Query(
		`SELECT id, author, message, created_at 
         FROM global_chat 
         ORDER BY created_at DESC 
         LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []business.GlobalChatMessage
	for rows.Next() {
		var msg business.GlobalChatMessage
		if err := rows.Scan(&msg.ID, &msg.Author, &msg.Message, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
