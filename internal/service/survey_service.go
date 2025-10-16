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
	GetAllSurveysByUser(ctx context.Context, userId int64) ([]domains.Survey, error)
	GetSurveyByID(ctx context.Context, ownerID int64, surveyID int64) (domains.Survey, error)
	GetSurveyAccessByHash(ctx context.Context, hash []byte) (domains.SurveyAccess, error)
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

func (h *SurveyService) GetAllSurveysByUser(ctx context.Context, userId int) ([]domains.Survey, error) {
	surveys, err := h.provider.GetAllSurveysByUser(ctx, int64(userId))
	if err != nil {
		slog.Error("GetAllSurveysByUser failed", "err", err, "user_id", userId)
		return nil, err
	}
	return surveys, nil
}

func (h *SurveyService) GetSurveyById(ctx context.Context, userId int, surveyId int) (domains.Survey, error) {
	survey, err := h.provider.GetSurveyByID(ctx, int64(userId), int64(surveyId))
	if err != nil {
		slog.Error("GetSurveyById failed", "err", err, "user_id", userId, "survey_id", surveyId)
		return domains.Survey{}, err
	}
	return survey, nil
}

func (h *SurveyService) AccessSurveyByToken(ctx context.Context, token string) (domains.SurveyAccess, error) {
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
		slog.Warn("AccessSurveyByToken parse", "err", err)
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
	access, err := h.provider.GetSurveyAccessByHash(ctx, hash[:])
	if err != nil {
		slog.Warn("AccessSurveyByToken storage", "err", err)
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	if access.Enrollment.ID != enrollmentID {
		slog.Warn("AccessSurveyByToken enrollment mismatch", "expected", enrollmentID, "actual", access.Enrollment.ID)
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}
	if access.Survey.ID != surveyID {
		slog.Warn("AccessSurveyByToken survey mismatch", "expected", surveyID, "actual", access.Survey.ID)
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	if access.Enrollment.TokenExpiresAt != nil && time.Now().After(*access.Enrollment.TokenExpiresAt) {
		return domains.SurveyAccess{}, ErrSurveyTokenExpired
	}
	if access.Enrollment.UseLimit > 0 && access.Enrollment.UsedCount >= access.Enrollment.UseLimit {
		return domains.SurveyAccess{}, ErrSurveyTokenUsed
	}
	switch access.Enrollment.State {
	case "invited", "pending", "approved", "active":
		// allowed states
	default:
		return domains.SurveyAccess{}, ErrSurveyTokenInvalid
	}

	return access, nil
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
