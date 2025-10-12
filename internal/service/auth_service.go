package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"mymodule/internal/domains"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	provider AuthProvider
	secret   string
}

type AuthProvider interface {
	SaveUser(ctx context.Context, passHash string, userData domains.Questioner) error
	GetUserByEmail(ctx context.Context, email string) (domains.Questioner, error)
	GetUserByID(ctx context.Context, ID int) (domains.Questioner, error)
}

func NewAuthService(provider AuthProvider, secret string) *AuthService {
	return &AuthService{
		provider: provider,
		secret:   secret,
	}
}

func (s *AuthService) Login(ctx context.Context, email string, password string) (string, string, error) {
	user, err := s.provider.GetUserByEmail(ctx, email)
	if err != nil {
		slog.Error("Fetch user error %v", err.Error())
		return "", "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))

	if err != nil {
		return "", "", PasswordIncorrect
	}

	accessToken, refreshToken, err := s.GenerateTokens(user)
	if err != nil {
		slog.Error("auth: failed to generate tokens: %v", err)
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (s *AuthService) GenerateTokens(user domains.Questioner) (accessToken string, refreshToken string, err error) {
	accessExpiration := time.Now().Add(15 * time.Minute)
	refreshExpiration := time.Now().Add(7 * 24 * time.Hour)
	slog.Info(s.secret)
	accessClaims := jwt.MapClaims{
		"sub":  user.Id,
		"exp":  accessExpiration.Unix(),
		"type": "access",
	}
	accessJWT := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessJWT.SignedString([]byte(s.secret))
	if err != nil {
		return "", "", err
	}

	refreshClaims := jwt.MapClaims{
		"sub":  user.Id,
		"exp":  refreshExpiration.Unix(),
		"type": "refresh",
	}
	refreshJWT := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshJWT.SignedString([]byte(s.secret))
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *AuthService) Register(ctx context.Context, userData domains.Questioner) error {
	passHash, err := bcrypt.GenerateFromPassword([]byte(userData.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Create hash pass error", err.Error())
		return err
	}

	err = s.provider.SaveUser(ctx, string(passHash), userData)
	if err != nil {
		slog.Error("Save user error", err.Error())
		return err
	}

	return nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	sub, claims, err := s.validateAndGetSubByToken(refreshToken)
	slog.Info("uid ", sub)
	if claims["type"] != "refresh" || claims["subject"] == 0 {
		return "", "", TokenIncorrect
	}

	user, err := s.provider.GetUserByID(ctx, int(sub))
	if err != nil {
		return "", "", err
	}

	accessToken, newRefreshToken, err := s.GenerateTokens(user)
	if err != nil {
		return "", "", err
	}

	return accessToken, newRefreshToken, nil

}
func (s *AuthService) Me(ctx context.Context, token string) (domains.Questioner, error) {
	sub, _, err := s.validateAndGetSubByToken(token)
	if err != nil {
		return domains.Questioner{}, err
	}
	user, err := s.provider.GetUserByID(ctx, sub)
	if err != nil {
		return domains.Questioner{}, err
	}
	return user, nil
}

func (s *AuthService) validateAndGetSubByToken(initToken string) (int, jwt.MapClaims, error) {
	token, err := jwt.Parse(initToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		log.Println(initToken)
		return []byte(s.secret), nil
	})
	if err != nil || !token.Valid {
		log.Println(err)
		return 0, nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	slog.Info("claims ", claims)
	if !ok {
		return 0, nil, errors.New("invalid claims")
	}

	subStr, ok := claims["sub"].(string)
	if !ok {
		return 0, nil, errors.New("subject missing")
	}

	uid, err := strconv.ParseInt(subStr, 10, 64)

	return int(uid), claims, nil
}
