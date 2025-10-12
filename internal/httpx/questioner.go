package httpx

import (
	"context"
	"mymodule/internal/domains"
	"mymodule/internal/storage/providers"
	"net/http"

	"github.com/gorilla/mux"
)

const questionerContextKey contextKey = "questioner"

func Questioner(provider providers.AuthProvider) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sub, ok := UserIdFromContext(r.Context())
			if !ok {
				Error(w, http.StatusUnauthorized, "Unauthorized")
				return
			}
			user, err := provider.GetUserByID(r.Context(), sub)
			if err != nil {
				Error(w, http.StatusUnauthorized, "Unauthorized")
				return
			}
			ctx := context.WithValue(r.Context(), userIDContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func QuestionerFromContext(ctx context.Context) (domains.Questioner, bool) {
	sub, ok := ctx.Value(questionerContextKey).(domains.Questioner)
	return sub, ok
}
