package domains

import (
	"bytes"
	"encoding/json"
	"time"
)

type Survey struct {
	ID               int64           `json:"id"`
	OwnerID          int64           `json:"owner_id"`
	TemplateID       *int64          `json:"template_id,omitempty"`
	SnapshotVersion  int             `json:"snapshot_version"`
	FormSnapshotJSON json.RawMessage `json:"form_snapshot_json"`
	Title            string          `json:"title"`
	Mode             string          `json:"mode"`
	Status           string          `json:"status"`
	MaxParticipants  *int            `json:"max_participants,omitempty"`
	PublicSlug       *string         `json:"public_slug,omitempty"`
	StartsAt         *time.Time      `json:"starts_at,omitempty"`
	EndsAt           *time.Time      `json:"ends_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

type SurveyCreate struct {
	TemplateID      int64              `json:"template_id"`
	Title           string             `json:"title"`
	Mode            string             `json:"invitationMode"`
	Status          string             `json:"status"`
	MaxParticipants *int               `json:"max_participants,omitempty"`
	PublicSlug      *string            `json:"public_slug,omitempty"`
	StartsAt        *time.Time         `json:"starts_at,omitempty"`
	EndsAt          *time.Time         `json:"ends_at,omitempty"`
	Participants    []EnrollmentCreate `json:"participants"`
}

type SurveyUpdate struct {
	Title           *string     `json:"title,omitempty"`
	Mode            *string     `json:"invitationMode,omitempty"`
	Status          *string     `json:"status,omitempty"`
	MaxParticipants OptionalInt `json:"max_participants"`
	PublicSlug      *string     `json:"public_slug"`
	StartsAt        *time.Time  `json:"starts_at"`
	EndsAt          *time.Time  `json:"ends_at"`
}

func (u SurveyUpdate) HasChanges() bool {
	return u.Title != nil || u.Mode != nil || u.Status != nil || u.MaxParticipants.Present || u.PublicSlug != nil || u.StartsAt != nil || u.EndsAt != nil
}

type SurveyToSave struct {
	OwnerID          int64
	TemplateID       int64
	SnapshotVersion  int
	FormSnapshotJSON json.RawMessage
	Title            string
	Mode             string
	Status           string
	MaxParticipants  *int
	PublicSlug       *string
	StartsAt         *time.Time
	EndsAt           *time.Time
	Enrollments      []EnrollmentCreate
}

type EnrollmentCreate struct {
	FullName       string  `json:"full_name"`
	Email          *string `json:"email,omitempty"`
	Phone          *string `json:"phone,omitempty"`
	TelegramChatID *int64  `json:"telegram_chat_id,omitempty"`
}

type EnrollmentInvitation struct {
	EnrollmentID int64     `json:"enrollment_id"`
	Token        string    `json:"token"`
	ExpiresAt    time.Time `json:"expires_at"`
	FullName     string    `json:"full_name"`
	Email        *string   `json:"email,omitempty"`
}

type SurveyCreateResult struct {
	Survey      Survey                 `json:"survey"`
	Invitations []EnrollmentInvitation `json:"invitations"`
}

type EnrollmentTokenPayload struct {
	SurveyID       int64
	EnrollmentID   int64
	OwnerID        int64
	Enrollment     EnrollmentCreate
	SurveyStartsAt *time.Time
	SurveyEndsAt   *time.Time
}

type EnrollmentTokenUpdate struct {
	EnrollmentID int64     `json:"enrollmentId"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type EnrollmentTokenUpdateResult struct {
	EnrollmentID int64      `json:"enrollment_id"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

type OptionalInt struct {
	Present bool
	Value   *int
}

func (o *OptionalInt) UnmarshalJSON(data []byte) error {
	o.Present = true
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		o.Value = nil
		return nil
	}
	var parsed int
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return err
	}
	o.Value = &parsed
	return nil
}

type EnrollmentTokenGenerator func(payload EnrollmentTokenPayload) (token string, hash []byte, expiresAt time.Time, err error)

type SurveyAccessRequest struct {
	Token string `json:"token"`
}

type SurveyStartRequest struct {
	Token   string `json:"token"`
	Channel string `json:"channel,omitempty"`
}

type Enrollment struct {
	ID             int64      `json:"id"`
	SurveyID       int64      `json:"survey_id"`
	FullName       string     `json:"full_name"`
	Email          *string    `json:"email,omitempty"`
	Phone          *string    `json:"phone,omitempty"`
	TelegramChatID *int64     `json:"telegram_chat_id,omitempty"`
	State          string     `json:"state"`
	TokenExpiresAt *time.Time `json:"token_expires_at,omitempty"`
	UseLimit       int        `json:"use_limit"`
	UsedCount      int        `json:"used_count"`
	TokenHash      []byte     `json:"-"`
}

type SurveyAccess struct {
	Survey     Survey     `json:"survey"`
	Enrollment Enrollment `json:"enrollment"`
}
