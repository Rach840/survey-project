package service

import (
	"context"
	"log/slog"
	"mymodule/internal/domains"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	provider AuthProvider
}

type AuthProvider interface {
	Register(ctx context.Context, userData domains.Questioner) error
	Login(ctx context.Context, email string) (domains.Questioner, error)
}

func NewAuthService(provider AuthProvider) *AuthService {
	return &AuthService{
		provider: provider,
	}
}

func (s *AuthService) Register(ctx context.Context, userData domains.Questioner) error {
	passHash, err := bcrypt.GenerateFromPassword([]byte(userData.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Create hash pass error", err.Error())
		return err
	}

}
