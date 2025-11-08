package service

import (
	"context"
	"log/slog"
	"mymodule/internal/domains"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	provider UserProvider
}

func (u UserService) GetAllUsers(ctx context.Context) ([]domains.User, error) {

	users, err := u.provider.GetAllUser(ctx)
	if err != nil {
		return nil, err
	}
	return users, nil
}
func (u UserService) Create(ctx context.Context, userData domains.Questioner) error {
	passHash, err := bcrypt.GenerateFromPassword([]byte(userData.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Create hash pass error", err.Error())
		return err
	}

	err = u.provider.SaveUser(ctx, string(passHash), userData)
	if err != nil {
		slog.Error("Save user error", err.Error())
		return err
	}

	return nil
}

type UserProvider interface {
	GetAllUser(ctx context.Context) ([]domains.User, error)
	SaveUser(ctx context.Context, passHash string, userData domains.Questioner) error
}

func NewUserService(provider UserProvider) *UserService {
	return &UserService{
		provider: provider,
	}
}
