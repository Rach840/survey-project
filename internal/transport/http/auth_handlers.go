package http

import (
	"context"
	"errors"
	"mymodule/internal/domains"
	"mymodule/internal/httpx"
	"mymodule/internal/storage"
	"net/http"
)

type AuthHandlers struct {
	service AuthServices
}
type AuthServices interface {
	RegisterQuestioner(ctx context.Context, user domains.Questioner) error
	Login(ctx context.Context, user LoginData) (string, error)
}

func NewAuthHandlers(service AuthServices) *AuthHandlers {
	return &AuthHandlers{
		service: service,
	}
}

func (srv AuthHandlers) RegisterQuestioner(w http.ResponseWriter, r http.Request) {
	userData, err := httpx.ReadBody[domains.Questioner](r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	ctx := context.Background()

	if err := srv.service.RegisterQuestioner(ctx, userData); err != nil {
		if errors.As(err, &storage.ErrUserExist) {
			http.Error(w, "Ошибка", http.StatusConflict)
			return
		}
		http.Error(w, "Ошибка", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	return
}

func (srv AuthHandlers) Login(w http.ResponseWriter, r http.Request) {
	loginData, err := httpx.ReadBody[LoginData](r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	ctx := context.Background()
	token, err := srv.service.Login(ctx, loginData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	httpx.JSON(w, http.StatusOK, token)

	return
}
