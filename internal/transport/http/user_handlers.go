package httptransport

import (
	"context"
	"log/slog"
	"mymodule/internal/domains"
	"mymodule/internal/httpx"
	"net/http"
)

type UserHandlers struct {
	service UserServices
}
type UserServices interface {
	GetAllUsers(ctx context.Context) ([]domains.Questioner, error)
}

func NewUserHandlers(services UserServices) *UserHandlers {
	return &UserHandlers{
		service: services,
	}
}

func (h *UserHandlers) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.GetAllUsers(r.Context())
	if err != nil {
		http.Error(w, "Ошибка", http.StatusInternalServerError)
		return
	}
	slog.Info("get all templates by user", users)
	httpx.JSON(w, http.StatusOK, users)
	return

}
