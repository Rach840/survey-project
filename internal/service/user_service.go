package service

import (
	"context"
	"mymodule/internal/domains"
)

type UserService struct {
	provider UserProvider
}

func (u UserService) GetAllUsers(ctx context.Context) ([]domains.Questioner, error) {

	users, err := u.provider.GetAllUser(ctx)
	if err != nil {
		return nil, err
	}
	return users, nil
}

type UserProvider interface {
	GetAllUser(ctx context.Context) ([]domains.Questioner, error)
}

func NewUserService(provider UserProvider) *UserService {
	return &UserService{
		provider: provider,
	}
}
