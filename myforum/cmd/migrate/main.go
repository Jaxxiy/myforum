package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("Переменная окружения DB_DSN не установлена")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе: %v", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("Ошибка инициализации драйвера: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://../../migrations", // путь к миграциям
		"postgres",
		driver,
	)
	if err != nil {
		log.Fatalf("Ошибка при создании миграции: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Ошибка миграции: %v", err)
	}
	log.Println("Миграции успешно применены")
}
