package httptransport

import (
	"mymodule/internal/config"
	"mymodule/internal/service"
	"mymodule/internal/storage/providers"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Router(db *pgxpool.Pool, cfg *config.Config) *mux.Router {
	router := mux.NewRouter()

	allProviders := providers.New(db)
	authService := service.NewAuthService(allProviders.AuthProvider, cfg.JWT.Secret)
	authHandler := NewAuthHandlers(authService)

	api := router.PathPrefix("/api").Subrouter()

	auth := api.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost)
	auth.HandleFunc("/register", authHandler.RegisterQuestioner).Methods(http.MethodPost)
	auth.HandleFunc("/refresh", authHandler.Refresh).Methods(http.MethodPost)
	auth.HandleFunc("/me", authHandler.Me).Methods(http.MethodGet)

	return router
}
