package httptransport

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"mymodule/internal/domains"
	"mymodule/internal/httpx"
	"mymodule/internal/service"
	"mymodule/internal/storage"
	"net/http"
	"strings"
)

type AuthHandlers struct {
	service AuthServices
}
type AuthServices interface {
	Register(ctx context.Context, user domains.Questioner) error
	Login(ctx context.Context, email string, password string) (string, string, error)
	Refresh(ctx context.Context, token string) (string, string, error)
	Me(ctx context.Context, token string) (domains.Questioner, error)
}

func NewAuthHandlers(service AuthServices) *AuthHandlers {
	return &AuthHandlers{
		service: service,
	}
}

func (srv AuthHandlers) RegisterQuestioner(w http.ResponseWriter, r *http.Request) {
	slog.Info("RegisterQuestioner called")
	userData, err := httpx.ReadBody[domains.Questioner](*r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	ctx := r.Context()

	if err := srv.service.Register(ctx, userData); err != nil {
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

func (srv AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	slog.Info("Login called")
	loginData, err := httpx.ReadBody[LoginData](*r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	accessToken, refreshToken, err := srv.service.Login(r.Context(), loginData.Email, loginData.Password)

	if err != nil {
		if errors.Is(err, service.PasswordIncorrect) {
			httpx.Error(w, http.StatusBadRequest, "Пароль не верный")
		}
		httpx.Error(w, http.StatusBadRequest, "Ошибка сервера")
	}

	httpx.JSON(w, http.StatusOK, struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}{AccessToken: accessToken, RefreshToken: refreshToken})

	return
}
func (s AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	tokenByCookie, err := r.Cookie("refreshToken")
	if err != nil || tokenByCookie.Value == "" {
		httpx.Error(w, http.StatusBadRequest, "Refresh token is required")
		return
	}

	accessToken, refreshToken, err := s.service.Refresh(r.Context(), tokenByCookie.Value)

	if err != nil {
		if errors.Is(err, service.TokenIncorrect) {
			httpx.Error(w, http.StatusBadRequest, "Token is incorrect")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			httpx.Error(w, http.StatusUnauthorized, "User not found")
			return
		}
		slog.Error(err.Error())
		httpx.Error(w, http.StatusInternalServerError, "Failed to retrieve user")
		return

	}
	httpx.JSON(w, http.StatusOK, struct {
		accessToken  string
		refreshToken string
	}{accessToken, refreshToken})
	w.WriteHeader(http.StatusOK)
	return
}

func (srv AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	user, err := srv.service.Me(r.Context(), tokenString)

	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	httpx.JSON(w, http.StatusOK, user)
	return
}

//func (s *AuthHandlers) Auth(w http.ResponseWriter, r *http.Request) {
//		tokenStr := r.URL.Query().Get("token")
//		if tokenStr == "" {
//			httpx.Error(w, http.StatusBadRequest, "Token is required")
//			return
//		}
//
//		claims := &jwt.StandardClaims{}
//		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
//			return []byte(h.jwtSecret), nil
//		})
//		if err != nil || !token.Valid {
//			httpx.Error(w, http.StatusUnauthorized, "Invalid or expired token")
//			return
//		}
//
//		magicToken, err := h.authService.FetchMagicToken(tokenStr)
//		if err != nil {
//			if errors.Is(err, sql.ErrNoRows) {
//				httpx.Error(w, http.StatusUnauthorized, "Token not found or already used")
//				return
//			}
//			log.Printf("auth: failed to fetch magic token: %v", err)
//			httpx.Error(w, http.StatusInternalServerError, "Failed to process token")
//			return
//		}
//
//		if time.Now().After(magicToken.ExpiresAt) {
//			httpx.Error(w, http.StatusUnauthorized, "Token expired")
//			return
//		}
//
//		if err := h.authService.MarkMagicTokenUsed(magicToken.ID); err != nil {
//			log.Printf("auth: failed to mark token as used: %v", err)
//			httpx.Error(w, http.StatusInternalServerError, "Failed to process token")
//			return
//		}
//
//		user, err := h.authService.FindUserByID(magicToken.UserID)
//		if err != nil {
//			if errors.Is(err, sql.ErrNoRows) {
//				httpx.Error(w, http.StatusNotFound, "User not found")
//			} else {
//				log.Printf("auth: failed to fetch user: %v", err)
//				httpx.Error(w, http.StatusInternalServerError, "Failed to process user")
//			}
//			return
//		}
//
//		accessToken, refreshToken, err := h.authService.GenerateTokens(user)
//		if err != nil {
//			log.Printf("auth: failed to generate tokens: %v", err)
//			httpx.Error(w, http.StatusInternalServerError, "Failed to generate tokens")
//			return
//		}
//
//		http.SetCookie(w, &http.Cookie{
//			Name:     "accessToken",
//			Value:    accessToken,
//			Path:     "/",
//			Domain:   h.appDomain,
//			MaxAge:   60 * 15,
//			HttpOnly: true,
//		})
//		http.SetCookie(w, &http.Cookie{
//			Name:     "refreshToken",
//			Value:    refreshToken,
//			Path:     "/",
//			Domain:   h.appDomain,
//			MaxAge:   60 * 60 * 24 * 7,
//			HttpOnly: true,
//		})
//
//		http.Redirect(w, r, h.appURL+"/panel", http.StatusFound)
//	}
