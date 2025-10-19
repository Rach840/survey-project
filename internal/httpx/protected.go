package httpx

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
)

type contextKey string

const userIDContextKey contextKey = "userID"

func Protected(jwtSecret string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			slog.Info(authHeader)
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				Error(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				log.Println(tokenString)
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				Error(w, http.StatusUnauthorized, "Unauthorized")
				log.Println(err)
				return
			}
			claims, ok := token.Claims.(jwt.MapClaims)
			slog.Info("claims ", claims)
			if !ok {
				return
			}

			subStr, ok := claims["sub"].(string)
			if !ok {
				return
			}

			ctx := WithUserID(r.Context(), subStr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDContextKey, id)
}
func UserIdFromContext(ctx context.Context) (int, bool) {
	val := ctx.Value(userIDContextKey)
	slog.Info("context value", val)

	sub, ok := val.(string)
	if !ok {
		return 0, false
	}

	uid, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		slog.Error("parse error", "err", err)
		return 0, false
	}
	slog.Info("context value", int(uid))
	return int(uid), true
}
