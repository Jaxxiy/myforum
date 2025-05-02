// internal/repository/forums.go
package repository

import (
	"database/sql"
	"errors"

	"github.com/jaxxiy/myforum/internal/business"
)

type ForumsRepo struct {
	DB *sql.DB
}

func NewForumsRepo(db *sql.DB) *ForumsRepo {
	return &ForumsRepo{
		DB: db,
	}
}

func (r *ForumsRepo) Create(f business.Forum) (int, error) {
	var id int
	err := r.DB.QueryRow(`INSERT INTO forums (name, description) VALUES ($1, $2) RETURNING id`, f.Title, f.Description).Scan(&id)
	return id, err
}

func (r *ForumsRepo) GetAll() ([]business.Forum, error) {
	rows, err := r.DB.Query(`SELECT id, name, description FROM forums`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var forums []business.Forum
	for rows.Next() {
		var f business.Forum
		if err := rows.Scan(&f.ID, &f.Title, &f.Description); err != nil {
			return nil, err
		}
		forums = append(forums, f)
	}
	return forums, nil
}

func (r *ForumsRepo) GetByID(id int) (*business.Forum, error) {
	query := `SELECT id, name, description FROM forums WHERE id = $1`
	row := r.DB.QueryRow(query, id)

	var forum business.Forum
	err := row.Scan(&forum.ID, &forum.Title, &forum.Description)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("forum not found")
		}
		return nil, err
	}

	return &forum, nil
}

// Аналогичные методы для Update и Delete
func (r *ForumsRepo) Update(id int, f business.Forum) error {
	result, err := r.DB.Exec(
		`UPDATE forums SET name = $1, description = $2 WHERE id = $3`,
		f.Title, f.Description, id,
	)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("no forum found with the given ID")
	}
	return nil
}

// Delete (новый метод)
func (r *ForumsRepo) Delete(id int) error {
	result, err := r.DB.Exec(
		`DELETE FROM forums WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("no forum found with the given ID")
	}
	return nil
}

//Сообщения

func (r *ForumsRepo) CreateMessage(message business.Message) (int, error) {
	var id int
	err := r.DB.QueryRow(
		`INSERT INTO messages (forum_id, author, content) 
         VALUES ($1, $2, $3) RETURNING id`,
		message.ForumID, message.Author, message.Content,
	).Scan(&id)
	return id, err
}

func (r *ForumsRepo) GetMessages(forumID int) ([]business.Message, error) {
	rows, err := r.DB.Query(`
		SELECT id, forum_id, author, content, created_at 
		FROM messages 
		WHERE forum_id = $1 
		ORDER BY created_at DESC`, forumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []business.Message
	for rows.Next() {
		var m business.Message
		if err := rows.Scan(&m.ID, &m.ForumID, &m.Author, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// DeleteMessage удаляет сообщение по ID
func (r *ForumsRepo) DeleteMessage(id int) error {
	_, err := r.DB.Exec("DELETE FROM messages WHERE id = $1", id)
	return err
}
