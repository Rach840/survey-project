package httpx

import (
	"context"
	"mymodule/internal/storage/providers"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
)

type contextKey string

const userIDContextKey contextKey = "userID"

func Protected(authProvider providers.AuthProvider, jwtSecret string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				Error(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			type Claims struct {
				Type string `json:"type"`
				jwt.StandardClaims
				Subject uint `json:"sub"`
			}

			var claims Claims
			token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid || claims.Subject == 0 {
				Error(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			ctx := context.WithValue(r.Context(), userIDContextKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIdFromContext(ctx context.Context) (int, bool) {
	sub, ok := ctx.Value(userIDContextKey).(int)
	return sub, ok
}
