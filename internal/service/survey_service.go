package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"mymodule/internal/domains"
	"mymodule/internal/storage"

	"github.com/dgrijalva/jwt-go"
)

type SurveyService struct {
	provider      SurveyProvider
	templates     TemplateProvider
	secret        string
	invitationTTL time.Duration
}

type SurveyProvider interface {
	SaveSurvey(ctx context.Context, survey domains.SurveyToSave, generator domains.EnrollmentTokenGenerator) (domains.Survey, []domains.EnrollmentInvitation, error)
	GetAllSurveysByUser(ctx context.Context, userId int64) ([]domains.SurveySummary, error)
	GetSurveyByID(ctx context.Context, ownerID, surveyID int) (domains.Survey, error)
	GetSurveyAccessByHash(ctx context.Context, id int) (domains.SurveyAccess, error)
	ListEnrollmentsBySurveyID(ctx context.Context, ownerID int64, surveyID int64) ([]domains.Enrollment, error)
	UpdateEnrollmentToken(ctx context.Context, enrollmentID int64, hash []byte, expiresAt time.Time) error
	AddEnrollments(ctx context.Context, surveyID, ownerID int64, participants []domains.EnrollmentCreate, generator domains.EnrollmentTokenGenerator) ([]domains.EnrollmentInvitation, error)
	RemoveEnrollment(ctx context.Context, surveyID, ownerID, enrollmentID int64) error
	SubmitSurveyResponse(ctx context.Context, payload domains.SurveyResponseToSave) (domains.SurveyResponseResult, error)
	GetSurveyResultByEnrollmentID(ctx context.Context, enrollmentID int64) (domains.SurveyResponseResult, error)
	ListSurveyResults(ctx context.Context, ownerID int64, surveyID int64) ([]domains.SurveyResult, error)
	GetSurveyStatistics(ctx context.Context, ownerID int64, surveyID int64) (domains.SurveyStatisticsCounts, error)
	GetEnrollmentByID(ctx context.Context, enrollmentID int) (domains.Enrollment, error)
}

func NewSurveyService(provider SurveyProvider, templates TemplateProvider, secret string) *SurveyService {
	return &SurveyService{
		provider:      provider,
		templates:     templates,
		secret:        secret,
		invitationTTL: 15 * 24 * time.Hour,
	}
}

func (h *SurveyService) CreateSurvey(ctx context.Context, payload domains.SurveyCreate, userId int) (domains.SurveyCreateResult, error) {
	slog.Info("CreateSurvey", "payload", payload)
	template, err := h.templates.GetTemplateById(ctx, userId, int(payload.TemplateID))
	if err != nil {
		return domains.SurveyCreateResult{}, fmt.Errorf("load template: %w", err)
	}

	schema := template.PublishedSchemaJSON
	if len(schema) == 0 {
		schema = template.DraftSchemaJSON
	}
	if len(schema) == 0 {
		return domains.SurveyCreateResult{}, errors.New("template has no schema to snapshot")
	}

	mode := payload.Mode
	if mode == "" {
		mode = "admin"
	}
	status := payload.Status
	if status == "" {
		status = "draft"
	}
	title := payload.Title
	if title == "" {
		title = template.Title
	}

	toSave := domains.SurveyToSave{
		OwnerID:          int64(userId),
		TemplateID:       template.ID,
		SnapshotVersion:  template.Version,
		FormSnapshotJSON: schema,
		Title:            title,
		Mode:             mode,
		Status:           status,
		MaxParticipants:  payload.MaxParticipants,
		PublicSlug:       nil,
		StartsAt:         payload.StartsAt,
		EndsAt:           payload.EndsAt,
		Enrollments:      payload.Participants,
	}

	survey, invites, err := h.provider.SaveSurvey(ctx, toSave, h.buildToken)
	if err != nil {
		slog.Error("Save survey error", "err", err)
		return domains.SurveyCreateResult{}, err
	}

	return domains.SurveyCreateResult{Survey: survey, Invitations: invites}, nil
}

func (h *SurveyService) AddSurveyParticipants(ctx context.Context, ownerID int, surveyID int, participants []domains.EnrollmentCreate) ([]domains.EnrollmentInvitation, error) {
	if len(participants) == 0 {
		return []domains.EnrollmentInvitation{}, nil
	}

	slog.Info("AddSurveyParticipants", "owner_id", ownerID, "survey_id", surveyID, "count", len(participants))

	survey, err := h.provider.GetSurveyByID(ctx, ownerID, surveyID)
	if err != nil {
		slog.Error("GetSurveyByID failed", "err", err, "owner_id", ownerID, "survey_id", surveyID)
		return nil, err
	}

	invitations, err := h.provider.AddEnrollments(ctx, survey.ID, survey.OwnerID, participants, h.buildToken)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrConflict):
			return nil, ErrSurveyParticipantExists
		case errors.Is(err, storage.ErrNotFound):
			return nil, storage.ErrNotFound
		default:
			slog.Error("AddEnrollments failed", "err", err, "owner_id", ownerID, "survey_id", surveyID)
			return nil, err
		}
	}

	return invitations, nil
}

func (h *SurveyService) RemoveSurveyParticipant(ctx context.Context, ownerID int, surveyID int, enrollmentID int) error {
	slog.Info("RemoveSurveyParticipant", "owner_id", ownerID, "survey_id", surveyID, "enrollment_id", enrollmentID)

	if _, err := h.provider.GetSurveyByID(ctx, ownerID, surveyID); err != nil {
		slog.Error("GetSurveyByID failed", "err", err, "owner_id", ownerID, "survey_id", surveyID)
		return err
	}

	if err := h.provider.RemoveEnrollment(ctx, int64(surveyID), int64(ownerID), int64(enrollmentID)); err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return ErrSurveyParticipantNotFound
		default:
			slog.Error("RemoveEnrollment failed", "err", err, "owner_id", ownerID, "survey_id", surveyID, "enrollment_id", enrollmentID)
			return err
		}
	}

	return nil
}

func (h *SurveyService) GetAllSurveysByUser(ctx context.Context, userId int) ([]domains.SurveySummary, error) {
	surveys, err := h.provider.GetAllSurveysByUser(ctx, int64(userId))
	if err != nil {
		slog.Error("GetAllSurveysByUser failed", "err", err, "user_id", userId)
		return nil, err
	}
	return surveys, nil
}

func (h *SurveyService) GetSurveyById(ctx context.Context, userId int, surveyId int) (domains.SurveyDetails, error) {
	survey, err := h.provider.GetSurveyByID(ctx, userId, surveyId)
	if err != nil {
		slog.Error("GetSurveyById failed", "err", err, "user_id", userId, "survey_id", surveyId)
		return domains.SurveyDetails{}, err
	}

	enrollments, err := h.provider.ListEnrollmentsBySurveyID(ctx, int64(userId), int64(surveyId))

	if err != nil {
		slog.Error("ListEnrollmentsBySurveyID failed", "err", err, "user_id", userId, "survey_id", surveyId)
		return domains.SurveyDetails{}, err
	}
	statsCounts, err := h.provider.GetSurveyStatistics(ctx, int64(userId), int64(surveyId))
	if err != nil {
		slog.Error("GetSurveyStatistics failed", "err", err, "user_id", userId, "survey_id", surveyId)
		return domains.SurveyDetails{}, err
	}
	invitations := make([]domains.EnrollmentInvitation, 0, len(enrollments))
	for _, enrollment := range enrollments {
		if !isEnrollmentTokenAllowed(enrollment.State) {
			continue
		}

		payload := domains.EnrollmentTokenPayload{
			SurveyID:     survey.ID,
			EnrollmentID: enrollment.ID,
			OwnerID:      survey.OwnerID,
			Enrollment: domains.EnrollmentCreate{
				FullName:       enrollment.FullName,
				Email:          enrollment.Email,
				Phone:          enrollment.Phone,
				TelegramChatID: enrollment.TelegramChatID,
			},
		}
		token, hash, expiresAt, err := h.buildToken(payload)
		if err != nil {
			slog.Error("buildToken failed", "err", err, "enrollment_id", enrollment.ID)
			return domains.SurveyDetails{}, err
		}

		if err := h.provider.UpdateEnrollmentToken(ctx, enrollment.ID, hash, expiresAt); err != nil {
			slog.Error("UpdateEnrollmentToken failed", "err", err, "enrollment_id", enrollment.ID)
			return domains.SurveyDetails{}, err
		}

		invitations = append(invitations, domains.EnrollmentInvitation{
			EnrollmentID: enrollment.ID,
			Token:        token,
			ExpiresAt:    expiresAt,
			FullName:     enrollment.FullName,
			Email:        enrollment.Email,
		})
	}

	return domains.SurveyDetails{
		Survey:      survey,
		Invitations: invitations,
		Statistics:  statsCounts.ToSurveyStatistics(),
	}, nil
}

func (h *SurveyService) GetSurveyResults(ctx context.Context, userId int, surveyId int) (domains.SurveyResultsSummary, error) {
	survey, err := h.provider.GetSurveyByID(ctx, userId, surveyId)
	if err != nil {
		slog.Error("GetSurveyResults get survey failed", "err", err, "user_id", userId, "survey_id", surveyId)
		return domains.SurveyResultsSummary{}, err
	}

	results, err := h.provider.ListSurveyResults(ctx, int64(userId), int64(surveyId))
	if err != nil {
		slog.Error("ListSurveyResults failed", "err", err, "user_id", userId, "survey_id", surveyId)
		return domains.SurveyResultsSummary{}, err
	}

	statsCounts, err := h.provider.GetSurveyStatistics(ctx, int64(userId), int64(surveyId))
	if err != nil {
		slog.Error("GetSurveyStatistics failed", "err", err, "user_id", userId, "survey_id", surveyId)
		return domains.SurveyResultsSummary{}, err
	}
	// TODO посмотреть как он себя ведет с результатами
	//for i := range results {
	//	results[i].Survey = survey
	//}

	return domains.SurveyResultsSummary{
		Survey:     survey,
		Results:    results,
		Statistics: statsCounts.ToSurveyStatistics(),
	}, nil
}

func (h *SurveyService) SubmitSurveyResponse(ctx context.Context, submission domains.SurveySubmission) (domains.SurveyResult, error) {
	if submission.Token == "" {
		return domains.SurveyResult{}, ErrSurveyTokenInvalid
	}

	access, err := h.fetchSurveyAccess(ctx, submission.Token)
	if err != nil {
		return domains.SurveyResult{}, err
	}
	if err := ensureTokenUsable(access); err != nil {
		return domains.SurveyResult{}, err
	}

	channel := submission.Channel
	if channel == "" {
		channel = "api"
	}

	answers := make([]domains.SurveyAnswerToSave, 0, len(submission.Answers))
	for _, answer := range submission.Answers {
		if answer.QuestionCode == "" {
			continue
		}
		repeatPath := answer.RepeatPath
		answers = append(answers, domains.SurveyAnswerToSave{
			QuestionCode:  answer.QuestionCode,
			SectionCode:   answer.SectionCode,
			RepeatPath:    repeatPath,
			ValueText:     answer.ValueText,
			ValueNumber:   answer.ValueNumber,
			ValueBool:     answer.ValueBool,
			ValueDate:     answer.ValueDate,
			ValueDateTime: answer.ValueDateTime,
			ValueJSON:     answer.ValueJSON,
		})
	}

	payload := domains.SurveyResponseToSave{
		SurveyID:       access.Survey.ID,
		EnrollmentID:   access.Enrollment.ID,
		Channel:        channel,
		State:          "submitted",
		SubmittedAt:    time.Now().UTC(),
		Answers:        answers,
		IncrementUsage: true,
	}

	saved, err := h.provider.SubmitSurveyResponse(ctx, payload)
	if err != nil {
		slog.Error("SubmitSurveyResponse failed", "err", err, "enrollment_id", access.Enrollment.ID)
		return domains.SurveyResult{}, err
	}

	access.Enrollment.UsedCount++
	if access.Enrollment.State == "invited" || access.Enrollment.State == "pending" {
		access.Enrollment.State = "approved"
	}

	return domains.SurveyResult{
		Survey:     access.Survey,
		Enrollment: access.Enrollment,
		Response:   saved.Response,
		Answers:    saved.Answers,
	}, nil
}

func (h *SurveyService) GetSurveyResultByToken(ctx context.Context, token string) (domains.SurveyResult, error) {
	access, err := h.fetchSurveyAccess(ctx, token)
	if err != nil {
		return domains.SurveyResult{}, err
	}

	result, err := h.provider.GetSurveyResultByEnrollmentID(ctx, access.Enrollment.ID)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return domains.SurveyResult{}, ErrSurveyResponseNotFound
		default:
			return domains.SurveyResult{}, err
		}
	}

	return domains.SurveyResult{
		Survey:     access.Survey,
		Enrollment: access.Enrollment,
		Response:   result.Response,
		Answers:    result.Answers,
	}, nil
}

func (h *SurveyService) GetEnrollmentResultByID(ctx context.Context, userId, enrollmentID, surveyID int) (domains.SurveyResult, error) {
	survey, err := h.provider.GetSurveyByID(ctx, userId, surveyID)
	enrollment, err := h.provider.GetEnrollmentByID(ctx, enrollmentID)
	slog.Info("survey", survey, "enrollment", enrollment)
	if err != nil {
		return domains.SurveyResult{}, err
	}

	result, err := h.provider.GetSurveyResultByEnrollmentID(ctx, int64(enrollmentID))
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return domains.SurveyResult{}, ErrSurveyResponseNotFound
		default:
			return domains.SurveyResult{}, err
		}
	}

	return domains.SurveyResult{
		Survey:     survey,
		Enrollment: enrollment,
		Response:   result.Response,
		Answers:    result.Answers,
	}, nil
}

func (h *SurveyService) AccessSurveyByToken(ctx context.Context, token string) (domains.SurveyAccess, error) {
	access, err := h.fetchSurveyAccess(ctx, token)
	if err != nil {
		return domains.SurveyAccess{}, err
	}
	if err := ensureTokenUsable(access); err != nil {
		return domains.SurveyAccess{}, err
	}
	return access, nil
}

func (h *SurveyService) fetchSurveyAccess(ctx context.Context, token string) (domains.SurveyAccess, error) {
	if token == "" {
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	claims := jwt.MapClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(h.secret), nil
	})
	if err != nil || !parsedToken.Valid {
		slog.Warn("fetchSurveyAccess parse", "err", err)
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}
	enrollmentID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	surveyID, err := claimToInt64(claims["survey_id"])
	if err != nil {
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	hash := sha256.Sum256([]byte(token))
	slog.Info(token, hash)
	access, err := h.provider.GetSurveyAccessByHash(ctx, int(enrollmentID))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domains.SurveyAccess{}, ErrSurveyTokenInvalid
		}
		slog.Warn("fetchSurveyAccess storage", "err", err)
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	if access.Enrollment.ID != enrollmentID {
		slog.Warn("fetchSurveyAccess enrollment mismatch", "expected", enrollmentID, "actual", access.Enrollment.ID)
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}
	if access.Survey.ID != surveyID {
		slog.Warn("fetchSurveyAccess survey mismatch", "expected", surveyID, "actual", access.Survey.ID)
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}
	if access.Enrollment.TokenExpiresAt != nil && time.Now().After(*access.Enrollment.TokenExpiresAt) {
		return domains.SurveyAccess{}, ErrSurveyTokenExpired
	}
	if !isEnrollmentTokenAllowed(access.Enrollment.State) {
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	return access, nil
}

func ensureTokenUsable(access domains.SurveyAccess) error {
	if access.Enrollment.UseLimit > 0 && access.Enrollment.UsedCount >= access.Enrollment.UseLimit {
		return ErrSurveyTokenUsed
	}
	return nil
}

func isEnrollmentTokenAllowed(state string) bool {
	switch state {
	case "invited", "pending", "approved", "active":
		return true
	default:
		return false
	}
}

func (h *SurveyService) buildToken(payload domains.EnrollmentTokenPayload) (string, []byte, time.Time, error) {
	expiresAt := time.Now().UTC().Add(h.invitationTTL)

	claims := jwt.MapClaims{
		"sub":       strconv.FormatInt(payload.EnrollmentID, 10),
		"survey_id": payload.SurveyID,
		"owner_id":  payload.OwnerID,
		"type":      "survey_invitation",
		"exp":       expiresAt.Unix(),
		"issued_at": time.Now().UTC().Unix(),
	}
	if payload.Enrollment.FullName != "" {
		claims["name"] = payload.Enrollment.FullName
	}
	if payload.Enrollment.Email != nil && *payload.Enrollment.Email != "" {
		claims["email"] = *payload.Enrollment.Email
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.secret))
	if err != nil {
		return "", nil, expiresAt, err
	}

	sum := sha256.Sum256([]byte(signed))
	return signed, sum[:], expiresAt, nil
}

func claimToInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case nil:
		return 0, errors.New("missing claim")
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case json.Number:
		return v.Int64()
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported claim type %T", value)
	}
}
