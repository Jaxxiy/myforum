package controllers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/jaxxiy/myforum/internal/business"
	"github.com/jaxxiy/myforum/internal/repository"
	"github.com/jaxxiy/myforum/pkg/jwt"

	"golang.org/x/crypto/bcrypt"
)

type AuthController struct {
	userRepo  *repository.UserRepo
	jwtSecret string
	templates *template.Template
}

func NewAuthController(repo *repository.UserRepo, secret string) *AuthController {
	templates := template.Must(template.ParseGlob("templates/*.html"))
	return &AuthController{
		userRepo:  repo,
		jwtSecret: secret,
		templates: templates,
	}
}

func (c *AuthController) RegisterPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.templates.ExecuteTemplate(w, "register.html", nil)
}

func (c *AuthController) LoginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.templates.ExecuteTemplate(w, "login.html", nil)
}

func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) {
	var req business.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user := business.User{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	}

	if _, err := c.userRepo.Create(user); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "user created"})
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	var req business.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user, err := c.userRepo.GetByUsername(req.Username)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := jwt.GenerateToken(user.ID, c.jwtSecret, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"token":    token,
		"user_id":  user.ID,
		"username": user.Username,
		"email":    user.Email,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
