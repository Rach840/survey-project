package httptransport

import (
	"context"
	"errors"
	"log/slog"
	"mymodule/internal/domains"
	"mymodule/internal/httpx"
	"mymodule/internal/service"
	"mymodule/internal/storage"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type SurveyHandlers struct {
	service SurveyServices
}

type SurveyServices interface {
	CreateSurvey(ctx context.Context, survey domains.SurveyCreate, userId int) (domains.SurveyCreateResult, error)
	GetAllSurveysByUser(ctx context.Context, userId int) ([]domains.SurveySummary, error)
	GetSurveyById(ctx context.Context, userId int, surveyId int) (domains.SurveyDetails, error)
	GetSurveyResults(ctx context.Context, userId int, surveyId int) (domains.SurveyResultsSummary, error)
	AccessSurveyByToken(ctx context.Context, token string) (domains.SurveyAccess, error)
	SubmitSurveyResponse(ctx context.Context, submission domains.SurveySubmission) (domains.SurveyResult, error)
	GetSurveyResultByToken(ctx context.Context, token string) (domains.SurveyResult, error)
	GetEnrollmentResultByID(ctx context.Context, ownerID, enrollmentID, surveyID int) (domains.SurveyResult, error)
	AddSurveyParticipants(ctx context.Context, ownerID int, surveyID int, participants []domains.EnrollmentCreate) ([]domains.EnrollmentInvitation, error)
	RemoveSurveyParticipant(ctx context.Context, ownerID int, surveyID int, enrollmentID int) error
}

func NewSurveyHandlers(service SurveyServices) *SurveyHandlers {
	return &SurveyHandlers{service: service}
}

type addParticipantsRequest struct {
	Participants []domains.EnrollmentCreate `json:"participants"`
}

type addParticipantsResponse struct {
	Invitations []domains.EnrollmentInvitation `json:"invitations"`
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

func (h *SurveyHandlers) AddParticipants(w http.ResponseWriter, r *http.Request) {
	surveyID := httpx.GetId(w, r)
	if surveyID == 0 {
		return
	}

	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	req, err := httpx.ReadBody[addParticipantsRequest](*r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Participants) == 0 {
		httpx.Error(w, http.StatusBadRequest, "participants is required")
		return
	}

	invitations, err := h.service.AddSurveyParticipants(r.Context(), user, int(surveyID), req.Participants)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSurveyParticipantExists):
			httpx.Error(w, http.StatusConflict, "Участник уже добавлен")
		case errors.Is(err, storage.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "Анкета не найдена")
		default:
			slog.Error("AddSurveyParticipants failed", "err", err, "user", user, "survey", surveyID)
			httpx.Error(w, http.StatusInternalServerError, "Не удалось добавить участников")
		}
		return
	}

	response := addParticipantsResponse{Invitations: invitations}
	httpx.JSON(w, http.StatusCreated, response)
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
	slog.Info("surveys", surveys)
	httpx.JSON(w, http.StatusOK, surveys)
}

func (h *SurveyHandlers) GetSurveyById(w http.ResponseWriter, r *http.Request) {
	surveyID := httpx.GetId(w, r)

	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	details, err := h.service.GetSurveyById(r.Context(), user, int(surveyID))
	if err != nil {
		slog.Error("GetSurveyById failed", "err", err, "user", user, "survey", surveyID)
		httpx.Error(w, http.StatusInternalServerError, "Не удалось получить анкету")
		return
	}

	httpx.JSON(w, http.StatusOK, details)
}

func (h *SurveyHandlers) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	surveyID := httpx.GetId(w, r)
	if surveyID == 0 {
		return
	}

	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	participantIDStr, ok := vars["participantId"]
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "participant_id is required")
		return
	}

	participantID, err := strconv.Atoi(participantIDStr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "Некорректный идентификатор участника")
		return
	}

	if err := h.service.RemoveSurveyParticipant(r.Context(), user, int(surveyID), participantID); err != nil {
		switch {
		case errors.Is(err, service.ErrSurveyParticipantNotFound), errors.Is(err, storage.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "Участник не найден")
		default:
			slog.Error("RemoveSurveyParticipant failed", "err", err, "user", user, "survey", surveyID, "participant", participantID)
			httpx.Error(w, http.StatusInternalServerError, "Не удалось удалить участника")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SurveyHandlers) GetSurveyResults(w http.ResponseWriter, r *http.Request) {
	slog.Info("surveyResults", r.URL.Query())
	surveyID := httpx.GetId(w, r)

	user, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	results, err := h.service.GetSurveyResults(r.Context(), user, int(surveyID))
	if err != nil {
		slog.Error("GetSurveyResults failed", "err", err, "user", user, "survey", surveyID)
		httpx.Error(w, http.StatusInternalServerError, "Не удалось получить результаты анкеты")
		return
	}

	httpx.JSON(w, http.StatusOK, results)
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
	slog.Info("AccessSurveyByToken", "token", token)
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

func (h *SurveyHandlers) SubmitSurveyResponse(w http.ResponseWriter, r *http.Request) {
	submission, err := httpx.ReadBody[domains.SurveySubmission](*r)
	if err != nil {

		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if submission.Token == "" {
		httpx.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	result, err := h.service.SubmitSurveyResponse(r.Context(), submission)
	if err != nil {
		slog.Error("SubmitSurveyResponse failed", "err", err, "submission", submission)
		switch {
		case errors.Is(err, service.ErrSurveyTokenInvalid):
			httpx.Error(w, http.StatusUnauthorized, "Недействительный токен")
		case errors.Is(err, service.ErrSurveyTokenExpired):
			httpx.Error(w, http.StatusGone, "Срок действия токена истек")
		case errors.Is(err, service.ErrSurveyTokenUsed):
			httpx.Error(w, http.StatusForbidden, "Лимит использования токена исчерпан")
		default:
			slog.Error("SubmitSurveyResponse failed", "err", err)
			httpx.Error(w, http.StatusInternalServerError, "Не удалось сохранить ответы")
		}
		return
	}

	httpx.JSON(w, http.StatusOK, result)
}

func (h *SurveyHandlers) GetSurveyResult(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	//if token == "" && r.Method != http.MethodGet {
	//	request, err := httpx.ReadBody[domains.SurveyAccessRequest](*r)
	//	if err != nil {
	//		httpx.Error(w, http.StatusBadRequest, err.Error())
	//		return
	//	}
	//	token = request.Token
	//}
	if token == "" {
		httpx.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	result, err := h.service.GetSurveyResultByToken(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSurveyTokenInvalid):
			httpx.Error(w, http.StatusUnauthorized, "Недействительный токен")
		case errors.Is(err, service.ErrSurveyTokenExpired):
			httpx.Error(w, http.StatusGone, "Срок действия токена истек")
		case errors.Is(err, service.ErrSurveyResponseNotFound):
			httpx.Error(w, http.StatusNotFound, "Результат не найден")
		default:
			slog.Error("GetSurveyResult failed", "err", err)
			httpx.Error(w, http.StatusInternalServerError, "Не удалось получить результат анкеты")
		}
		return
	}

	httpx.JSON(w, http.StatusOK, result)
}
func (h *SurveyHandlers) GetEnrollmentResultByID(w http.ResponseWriter, r *http.Request) {
	enrollmentID := r.URL.Query().Get("enrollment")
	surveyID := r.URL.Query().Get("survey")
	userID, ok := httpx.UserIdFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "не авторизован")
	}

	if enrollmentID == "" || surveyID == "" {
		httpx.Error(w, http.StatusBadRequest, "enrollmentID и surveyID обязательны")
	}
	enrollmentIDInt, err := strconv.Atoi(enrollmentID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "enrollmentID не валиден")
	}
	surveyIDInt, err := strconv.Atoi(surveyID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "surveyID не валиден")
	}
	slog.Info("GetEnrollmentResultByID", "enrollmentID", enrollmentIDInt, "surveyID", surveyIDInt)

	result, err := h.service.GetEnrollmentResultByID(r.Context(), userID, enrollmentIDInt, surveyIDInt)

	httpx.JSON(w, http.StatusOK, result)
}
