package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/jaxxiy/myforum/internal/handlers"
	"github.com/jaxxiy/myforum/internal/repository"
	"github.com/jaxxiy/myforum/internal/services"
	_ "github.com/lib/pq"
)

func main() {
	// Get database connection string from environment variable
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:Stas2005101010!@localhost:5432/forum?sslmode=disable"
	}

	// Connect to database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepo(db)

	// Initialize services
	authService := services.NewAuthService(userRepo)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService)

	// Initialize router
	r := mux.NewRouter()

	// Register routes
	auth := r.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/register", authHandler.RegisterPage).Methods("GET")
	auth.HandleFunc("/register", authHandler.Register).Methods("POST")
	auth.HandleFunc("/login", authHandler.LoginPage).Methods("GET")
	auth.HandleFunc("/login", authHandler.Login).Methods("POST")
	auth.HandleFunc("/validate", authHandler.ValidateToken).Methods("GET")

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000" // Default port for auth service
	}

	log.Printf("Auth service starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
