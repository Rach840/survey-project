package httptransport

import (
	"context"
	"errors"
	"log/slog"
	"mymodule/internal/domains"
	"mymodule/internal/httpx"
	"mymodule/internal/storage"
	"net/http"
)

type TemplateHandlers struct {
	service TemplateServices
}
type TemplateServices interface {
	CreateTemplate(ctx context.Context, template domains.TemplateCreate, userId int) error
	GetAllTemplatesByUser(ctx context.Context, userId int) ([]domains.Template, error)
}

func NewTemplateHandlers(service TemplateServices) *TemplateHandlers {
	return &TemplateHandlers{
		service: service,
	}
}

func (h *TemplateHandlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {

	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	templateData, err := httpx.ReadBody[domains.TemplateCreate](*r)
	if err != nil {
		slog.Error("CreateTemplate read template err", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	ctx := r.Context()

	if err := h.service.CreateTemplate(ctx, templateData, user); err != nil {
		if errors.Is(err, storage.ErrUserExist) {
			http.Error(w, "Ошибка", http.StatusConflict)
			return
		}
		http.Error(w, "Ошибка", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	return
}
func (h *TemplateHandlers) GetAllTemplatesByUser(w http.ResponseWriter, r *http.Request) {

	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	ctx := r.Context()
	templates, err := h.service.GetAllTemplatesByUser(ctx, user)
	if err != nil {
		http.Error(w, "Ошибка", http.StatusInternalServerError)
		return
	}
	slog.Info("get all templates by user", user, templates)
	httpx.JSON(w, http.StatusOK, templates)
	return
}
