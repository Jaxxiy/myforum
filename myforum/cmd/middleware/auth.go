package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/jaxxiy/myforum/pkg/jwt"
)

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := jwt.ParseToken(tokenString, secret)
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Добавляем user_id в контекст
			ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		}) // Была пропущена закрывающая скобка для HandlerFunc
	} // Была пропущена закрывающая скобка для возвращаемой функции
}
