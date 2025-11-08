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

type UserHandlers struct {
	service UserServices
}
type UserServices interface {
	GetAllUsers(ctx context.Context) ([]domains.User, error)
	Create(ctx context.Context, user domains.Questioner) error
}

func NewUserHandlers(services UserServices) *UserHandlers {
	return &UserHandlers{
		service: services,
	}
}
func (srv UserHandlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("RegisterQuestioner called")
	userData, err := httpx.ReadBody[domains.Questioner](*r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	ctx := r.Context()

	if err := srv.service.Create(ctx, userData); err != nil {
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

func (h UserHandlers) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.GetAllUsers(r.Context())
	if err != nil {
		http.Error(w, "Ошибка", http.StatusInternalServerError)
		return
	}
	slog.Info("get all templates by user", users)
	httpx.JSON(w, http.StatusOK, users)
	return

}
