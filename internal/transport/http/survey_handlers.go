package httptransport

import (
	"context"
	"errors"
	"log/slog"
	"mymodule/internal/domains"
	"mymodule/internal/httpx"
	"mymodule/internal/service"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type SurveyHandlers struct {
	service SurveyServices
}

type SurveyServices interface {
	CreateSurvey(ctx context.Context, survey domains.SurveyCreate, userId int) (domains.SurveyCreateResult, error)
	GetAllSurveysByUser(ctx context.Context, userId int) ([]domains.Survey, error)
	GetSurveyById(ctx context.Context, userId int, surveyId int) (domains.Survey, error)
	AccessSurveyByToken(ctx context.Context, token string) (domains.SurveyAccess, error)
}

func NewSurveyHandlers(service SurveyServices) *SurveyHandlers {
	return &SurveyHandlers{service: service}
}

func (h *SurveyHandlers) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	surveyData, err := httpx.ReadBody[domains.SurveyCreate](*r)
	if err != nil {
		slog.Error("CreateSurvey read survey err", "err", err)
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if surveyData.TemplateID == 0 {
		httpx.Error(w, http.StatusBadRequest, "template_id is required")
		return
	}

	created, err := h.service.CreateSurvey(r.Context(), surveyData, user)
	if err != nil {
		slog.Error("CreateSurvey failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "Не удалось создать анкету")
		return
	}

	httpx.JSON(w, http.StatusCreated, created)
}

func (h *SurveyHandlers) GetAllSurveysByUser(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	surveys, err := h.service.GetAllSurveysByUser(r.Context(), user)
	if err != nil {
		slog.Error("GetAllSurveysByUser failed", "err", err, "user", user)
		httpx.Error(w, http.StatusInternalServerError, "Не удалось получить анкеты")
		return
	}

	httpx.JSON(w, http.StatusOK, surveys)
}

func (h *SurveyHandlers) GetSurveyById(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	surveyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "Некорректный идентификатор анкеты")
		return
	}

	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	survey, err := h.service.GetSurveyById(r.Context(), user, int(surveyID))
	if err != nil {
		slog.Error("GetSurveyById failed", "err", err, "user", user, "survey", surveyID)
		httpx.Error(w, http.StatusInternalServerError, "Не удалось получить анкету")
		return
	}

	httpx.JSON(w, http.StatusOK, survey)
}

func (h *SurveyHandlers) AccessSurveyByToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" && r.Method != http.MethodGet {
		request, err := httpx.ReadBody[domains.SurveyAccessRequest](*r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		token = request.Token
	}
	if token == "" {
		httpx.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	access, err := h.service.AccessSurveyByToken(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSurveyTokenInvalid):
			httpx.Error(w, http.StatusUnauthorized, "Недействительный токен")
		case errors.Is(err, service.ErrSurveyTokenExpired):
			httpx.Error(w, http.StatusGone, "Срок действия токена истек")
		case errors.Is(err, service.ErrSurveyTokenUsed):
			httpx.Error(w, http.StatusForbidden, "Лимит использования токена исчерпан")
		default:
			slog.Error("AccessSurveyByToken failed", "err", err)
			httpx.Error(w, http.StatusInternalServerError, "Не удалось проверить токен")
		}
		return
	}

	httpx.JSON(w, http.StatusOK, access)
}
