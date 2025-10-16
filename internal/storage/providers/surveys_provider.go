package providers

import (
	"context"
	"database/sql"
	"fmt"
	"mymodule/internal/domains"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SurveyProvider struct {
	db *pgxpool.Pool
}

func NewSurveyProvider(db *pgxpool.Pool) *SurveyProvider {
	return &SurveyProvider{
		db: db,
	}
}

func (s SurveyProvider) SaveSurvey(ctx context.Context, survey domains.SurveyToSave, generator domains.EnrollmentTokenGenerator) (domains.Survey, []domains.EnrollmentInvitation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domains.Survey{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	insertSurvey := `
          INSERT INTO surveys (
              owner_id, template_id, snapshot_version,
              form_snapshot_json, title, mode, status,
              max_participants, public_slug, starts_at, ends_at
          )
          VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
          RETURNING
              id, owner_id, template_id, snapshot_version,
              form_snapshot_json, title, mode, status,
              max_participants, public_slug, starts_at, ends_at, created_at`

	var created domains.Survey
	var templateID sql.NullInt64
	var maxParticipants sql.NullInt64
	var publicSlug sql.NullString
	var startsAt *time.Time
	var endsAt *time.Time

	row := tx.QueryRow(ctx, insertSurvey,
		survey.OwnerID,
		survey.TemplateID,
		survey.SnapshotVersion,
		survey.FormSnapshotJSON,
		survey.Title,
		survey.Mode,
		survey.Status,
		survey.MaxParticipants,
		survey.PublicSlug,
		survey.StartsAt,
		survey.EndsAt,
	)

	if err := row.Scan(
		&created.ID,
		&created.OwnerID,
		&templateID,
		&created.SnapshotVersion,
		&created.FormSnapshotJSON,
		&created.Title,
		&created.Mode,
		&created.Status,
		&maxParticipants,
		&publicSlug,
		&startsAt,
		&endsAt,
		&created.CreatedAt,
	); err != nil {
		return domains.Survey{}, nil, fmt.Errorf("insert survey: %w", err)
	}

	if templateID.Valid {
		created.TemplateID = &templateID.Int64
	}
	if maxParticipants.Valid {
		v := int(maxParticipants.Int64)
		created.MaxParticipants = &v
	}
	if publicSlug.Valid {
		slug := publicSlug.String
		created.PublicSlug = &slug
	}
	created.StartsAt = startsAt
	created.EndsAt = endsAt

	invitations := make([]domains.EnrollmentInvitation, 0, len(survey.Enrollments))
	insertEnrollment := `
          INSERT INTO enrollments (
              survey_id, source, full_name, email, phone,
              telegram_chat_id, state, invited_by
          ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
          RETURNING id`

	for _, participant := range survey.Enrollments {
		var enrollmentID int64
		if err := tx.QueryRow(ctx, insertEnrollment,
			created.ID,
			"admin",
			participant.FullName,
			participant.Email,
			participant.Phone,
			participant.TelegramChatID,
			"invited",
			survey.OwnerID,
		).Scan(&enrollmentID); err != nil {
			return domains.Survey{}, nil, fmt.Errorf("insert enrollment: %w", err)
		}

		token, hash, expiresAt, err := generator(domains.EnrollmentTokenPayload{
			SurveyID:     created.ID,
			EnrollmentID: enrollmentID,
			OwnerID:      created.OwnerID,
			Enrollment:   participant,
		})
		if err != nil {
			return domains.Survey{}, nil, fmt.Errorf("generate token: %w", err)
		}

		if _, err := tx.Exec(ctx,
			`UPDATE enrollments SET token_hash = $1, token_expires_at = $2 WHERE id = $3`,
			hash, expiresAt, enrollmentID,
		); err != nil {
			return domains.Survey{}, nil, fmt.Errorf("update enrollment token: %w", err)
		}

		invitations = append(invitations, domains.EnrollmentInvitation{
			EnrollmentID: enrollmentID,
			Token:        token,
			ExpiresAt:    expiresAt,
			FullName:     participant.FullName,
			Email:        participant.Email,
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return domains.Survey{}, nil, fmt.Errorf("commit: %w", err)
	}

	return created, invitations, nil
}

func (s SurveyProvider) GetAllSurveysByUser(ctx context.Context, userId int64) ([]domains.Survey, error) {
	const query = `
		SELECT
			id, owner_id, template_id, snapshot_version,
			form_snapshot_json, title, mode, status,
			max_participants, public_slug, starts_at, ends_at, created_at
		FROM surveys
		WHERE owner_id = $1
		ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("list surveys: %w", err)
	}
	defer rows.Close()

	var result []domains.Survey
	for rows.Next() {
		survey, err := scanSurvey(rows)
		if err != nil {
			return nil, fmt.Errorf("scan survey: %w", err)
		}
		result = append(result, survey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate surveys: %w", err)
	}
	return result, nil
}

func (s SurveyProvider) GetSurveyByID(ctx context.Context, ownerID int64, surveyID int64) (domains.Survey, error) {
	const query = `
		SELECT
			id, owner_id, template_id, snapshot_version,
			form_snapshot_json, title, mode, status,
			max_participants, public_slug, starts_at, ends_at, created_at
		FROM surveys
		WHERE owner_id = $1 AND id = $2`

	row := s.db.QueryRow(ctx, query, ownerID, surveyID)
	survey, err := scanSurvey(row)
	if err != nil {
		return domains.Survey{}, fmt.Errorf("get survey: %w", err)
	}
	return survey, nil
}

func (s SurveyProvider) GetSurveyAccessByHash(ctx context.Context, hash []byte) (domains.SurveyAccess, error) {
	const query = `
		SELECT
			e.id,
			e.survey_id,
			e.full_name,
			e.email,
			e.phone,
			e.telegram_chat_id,
			e.state,
			e.token_hash,
			e.token_expires_at,
			e.use_limit,
			e.used_count,
			s.id,
			s.owner_id,
			s.template_id,
			s.snapshot_version,
			s.form_snapshot_json,
			s.title,
			s.mode,
			s.status,
			s.max_participants,
			s.public_slug,
			s.starts_at,
			s.ends_at,
			s.created_at
		FROM enrollments e
		JOIN surveys s ON s.id = e.survey_id
		WHERE e.token_hash = $1
		LIMIT 1`

	row := s.db.QueryRow(ctx, query, hash)

	var (
		access          domains.SurveyAccess
		email           sql.NullString
		phone           sql.NullString
		telegramChat    sql.NullInt64
		templateID      sql.NullInt64
		maxParticipants sql.NullInt64
		publicSlug      sql.NullString
		tokenExpires    sql.NullTime
		surveyStarts    *time.Time
		surveyEnds      *time.Time
		useLimit        int32
		usedCount       int32
	)

	if err := row.Scan(
		&access.Enrollment.ID,
		&access.Enrollment.SurveyID,
		&access.Enrollment.FullName,
		&email,
		&phone,
		&telegramChat,
		&access.Enrollment.State,
		&access.Enrollment.TokenHash,
		&tokenExpires,
		&useLimit,
		&usedCount,
		&access.Survey.ID,
		&access.Survey.OwnerID,
		&templateID,
		&access.Survey.SnapshotVersion,
		&access.Survey.FormSnapshotJSON,
		&access.Survey.Title,
		&access.Survey.Mode,
		&access.Survey.Status,
		&maxParticipants,
		&publicSlug,
		&surveyStarts,
		&surveyEnds,
		&access.Survey.CreatedAt,
	); err != nil {
		return domains.SurveyAccess{}, fmt.Errorf("get survey access: %w", err)
	}

	if email.Valid {
		access.Enrollment.Email = &email.String
	}
	if phone.Valid {
		access.Enrollment.Phone = &phone.String
	}
	if telegramChat.Valid {
		id := telegramChat.Int64
		access.Enrollment.TelegramChatID = &id
	}
	if tokenExpires.Valid {
		t := tokenExpires.Time
		access.Enrollment.TokenExpiresAt = &t
	}
	access.Enrollment.UseLimit = int(useLimit)
	access.Enrollment.UsedCount = int(usedCount)

	if templateID.Valid {
		access.Survey.TemplateID = &templateID.Int64
	}
	if maxParticipants.Valid {
		value := int(maxParticipants.Int64)
		access.Survey.MaxParticipants = &value
	}
	if publicSlug.Valid {
		slug := publicSlug.String
		access.Survey.PublicSlug = &slug
	}
	access.Survey.StartsAt = surveyStarts
	access.Survey.EndsAt = surveyEnds

	return access, nil
}

func scanSurvey(row pgx.Row) (domains.Survey, error) {
	var (
		survey          domains.Survey
		templateID      sql.NullInt64
		maxParticipants sql.NullInt64
		publicSlug      sql.NullString
		startsAt        *time.Time
		endsAt          *time.Time
	)

	if err := row.Scan(
		&survey.ID,
		&survey.OwnerID,
		&templateID,
		&survey.SnapshotVersion,
		&survey.FormSnapshotJSON,
		&survey.Title,
		&survey.Mode,
		&survey.Status,
		&maxParticipants,
		&publicSlug,
		&startsAt,
		&endsAt,
		&survey.CreatedAt,
	); err != nil {
		return domains.Survey{}, err
	}

	if templateID.Valid {
		survey.TemplateID = &templateID.Int64
	}
	if maxParticipants.Valid {
		value := int(maxParticipants.Int64)
		survey.MaxParticipants = &value
	}
	if publicSlug.Valid {
		slug := publicSlug.String
		survey.PublicSlug = &slug
	}
	survey.StartsAt = startsAt
	survey.EndsAt = endsAt

	return survey, nil
}
