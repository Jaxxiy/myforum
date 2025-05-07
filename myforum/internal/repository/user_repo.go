package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jaxxiy/myforum/internal/business"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(user business.User) (int, error) {
	query := `
		INSERT INTO users (username, email, password, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	var id int
	err := r.db.QueryRow(
		query,
		user.Username,
		user.Email,
		user.Password,
		user.Role,
		time.Now(),
		time.Now(),
	).Scan(&id)

	if err != nil {
		return 0, err
	}

	return id, nil
}

func (r *UserRepo) GetByUsername(username string) (*business.User, error) {
	query := `
		SELECT id, username, email, password, role, created_at, updated_at
		FROM users
		WHERE username = $1`

	user := &business.User{}
	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) GetByEmail(email string) (*business.User, error) {
	query := `
		SELECT id, username, email, password, role, created_at, updated_at
		FROM users
		WHERE email = $1`

	user := &business.User{}
	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) GetUserByID(userID int) (*business.User, error) {
	query := `
        SELECT id, username, email, role, created_at, updated_at
        FROM users
        WHERE id = $1`

	user := &business.User{}
	err := r.db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) UpdatePassword(userID int, hashedPassword string) error {
	query := `
		UPDATE users
		SET password = $1, updated_at = $2
		WHERE id = $3`

	_, err := r.db.Exec(query, hashedPassword, time.Now(), userID)
	return err
}
