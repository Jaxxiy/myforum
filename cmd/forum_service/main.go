package main

import (
	"log"

	"github.com/jaxxiy/myforum/internal/app"
)

func main() {
	srv := app.NewServer()
	if err := srv.Run(); err != nil {
		log.Fatalf("Ошибка запуска: %v", err)
	}
}
