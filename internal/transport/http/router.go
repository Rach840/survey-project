package httptransport

import (
	"mymodule/internal/config"
	"mymodule/internal/httpx"
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
	templateService := service.NewTemplateService(allProviders.TemplateProvider)
	authHandler := NewAuthHandlers(authService)
	templateHandler := NewTemplateHandlers(templateService)

	api := router.PathPrefix("/api").Subrouter()

	auth := api.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost)
	auth.HandleFunc("/register", authHandler.RegisterQuestioner).Methods(http.MethodPost)
	auth.HandleFunc("/refresh", authHandler.Refresh).Methods(http.MethodPost)
	auth.HandleFunc("/me", authHandler.Me).Methods(http.MethodGet)

	template := api.PathPrefix("/template").Subrouter()
	template.Use(httpx.Protected(cfg.JWT.Secret))
	template.Use(httpx.Questioner(*allProviders.AuthProvider))
	template.HandleFunc("/create", templateHandler.CreateTemplate).Methods(http.MethodPost)
	template.HandleFunc("/", templateHandler.GetAllTemplatesByUser).Methods(http.MethodGet)
	template.HandleFunc("/{id}", templateHandler.GetTemplateById).Methods(http.MethodGet)
	template.HandleFunc("/{id}", templateHandler.UpdateTemplate).Methods(http.MethodPatch)

	return router
}
