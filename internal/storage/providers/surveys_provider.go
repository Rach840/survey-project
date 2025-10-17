package providers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mymodule/internal/domains"
	"mymodule/internal/storage"
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

func (s SurveyProvider) ListEnrollmentsBySurveyID(ctx context.Context, ownerID int64, surveyID int64) ([]domains.Enrollment, error) {
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
			e.used_count
		FROM enrollments e
		JOIN surveys s ON s.id = e.survey_id
		WHERE s.owner_id = $1 AND e.survey_id = $2
		ORDER BY e.id`

	rows, err := s.db.Query(ctx, query, ownerID, surveyID)
	if err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}
	defer rows.Close()

	var enrollments []domains.Enrollment
	for rows.Next() {
		enrollment, err := scanEnrollment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan enrollment: %w", err)
		}
		enrollments = append(enrollments, enrollment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enrollments: %w", err)
	}
	return enrollments, nil
}

func (s SurveyProvider) UpdateEnrollmentToken(ctx context.Context, enrollmentID int64, hash []byte, expiresAt time.Time) error {
	const query = `
		UPDATE enrollments
		SET token_hash = $1, token_expires_at = $2
		WHERE id = $3`

	if _, err := s.db.Exec(ctx, query, hash, expiresAt, enrollmentID); err != nil {
		return fmt.Errorf("update enrollment token: %w", err)
	}
	return nil
}

func (s SurveyProvider) SubmitSurveyResponse(ctx context.Context, payload domains.SurveyResponseToSave) (domains.SurveyResponseResult, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domains.SurveyResponseResult{}, fmt.Errorf("begin response tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var channel interface{}
	if payload.Channel != "" {
		channel = payload.Channel
	}

	const upsert = `
		INSERT INTO responses (survey_id, enrollment_id, state, channel, submitted_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (survey_id, enrollment_id) DO UPDATE
		SET state = EXCLUDED.state,
		    channel = COALESCE(EXCLUDED.channel, responses.channel),
		    submitted_at = EXCLUDED.submitted_at
		RETURNING id, survey_id, enrollment_id, state, channel, started_at, submitted_at`

	row := tx.QueryRow(ctx, upsert,
		payload.SurveyID,
		payload.EnrollmentID,
		payload.State,
		channel,
		payload.SubmittedAt,
	)

	response, err := scanResponse(row)
	if err != nil {
		return domains.SurveyResponseResult{}, fmt.Errorf("upsert response: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM answers WHERE response_id = $1`, response.ID); err != nil {
		return domains.SurveyResponseResult{}, fmt.Errorf("clear answers: %w", err)
	}

	const insertAnswer = `
		INSERT INTO answers (
		    response_id, question_code, section_code, repeat_path,
		    value_text, value_number, value_bool, value_date,
		    value_datetime, value_json
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	answers := make([]domains.SurveyAnswer, 0, len(payload.Answers))
	for _, answer := range payload.Answers {
		var valueJSON interface{}
		if len(answer.ValueJSON) > 0 {
			valueJSON = answer.ValueJSON
		}
		if _, err := tx.Exec(ctx, insertAnswer,
			response.ID,
			answer.QuestionCode,
			answer.SectionCode,
			answer.RepeatPath,
			answer.ValueText,
			answer.ValueNumber,
			answer.ValueBool,
			answer.ValueDate,
			answer.ValueDateTime,
			valueJSON,
		); err != nil {
			return domains.SurveyResponseResult{}, fmt.Errorf("insert answer: %w", err)
		}
		answers = append(answers, toSurveyAnswer(answer))
	}

	if payload.IncrementUsage {
		const updateUsage = `
			UPDATE enrollments
			SET used_count = used_count + 1,
			    state = CASE
			        WHEN state IN ('invited','pending') THEN 'approved'
			        WHEN state = 'approved' THEN 'active'
			        ELSE state
			    END
			WHERE id = $1`
		if _, err := tx.Exec(ctx, updateUsage, payload.EnrollmentID); err != nil {
			return domains.SurveyResponseResult{}, fmt.Errorf("increment enrollment usage: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domains.SurveyResponseResult{}, fmt.Errorf("commit response: %w", err)
	}

	return domains.SurveyResponseResult{Response: response, Answers: answers}, nil
}

func (s SurveyProvider) GetSurveyResultByEnrollmentID(ctx context.Context, enrollmentID int64) (domains.SurveyResponseResult, error) {
	const query = `
		SELECT id, survey_id, enrollment_id, state, channel, started_at, submitted_at
		FROM responses
		WHERE enrollment_id = $1
		LIMIT 1`

	row := s.db.QueryRow(ctx, query, enrollmentID)
	response, err := scanResponse(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domains.SurveyResponseResult{}, fmt.Errorf("get survey response: %w", storage.ErrNotFound)
		}
		return domains.SurveyResponseResult{}, fmt.Errorf("get survey response: %w", err)
	}

	const answersQuery = `
		SELECT
			question_code,
			section_code,
			repeat_path,
			value_text,
			value_number,
			value_bool,
			value_date,
			value_datetime,
			value_json
		FROM answers
		WHERE response_id = $1
		ORDER BY question_code, repeat_path`

	rows, err := s.db.Query(ctx, answersQuery, response.ID)
	if err != nil {
		return domains.SurveyResponseResult{}, fmt.Errorf("list answers: %w", err)
	}
	defer rows.Close()

	var answers []domains.SurveyAnswer
	for rows.Next() {
		answer, err := scanAnswer(rows)
		if err != nil {
			return domains.SurveyResponseResult{}, fmt.Errorf("scan answer: %w", err)
		}
		answers = append(answers, answer)
	}
	if err := rows.Err(); err != nil {
		return domains.SurveyResponseResult{}, fmt.Errorf("iterate answers: %w", err)
	}

	return domains.SurveyResponseResult{Response: response, Answers: answers}, nil
}

func (s SurveyProvider) GetSurveyAccessByHash(ctx context.Context, id int) (domains.SurveyAccess, error) {

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
		WHERE e.id = $1
		LIMIT 1`

	row := s.db.QueryRow(ctx, query, id)

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
		slog.Error(err.Error())
		if errors.Is(err, pgx.ErrNoRows) {
			return domains.SurveyAccess{}, fmt.Errorf("get survey access: %w", storage.ErrNotFound)
		}
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

func scanResponse(row pgx.Row) (domains.SurveyResponse, error) {
	var (
		response    domains.SurveyResponse
		channel     sql.NullString
		submittedAt sql.NullTime
	)

	if err := row.Scan(
		&response.ID,
		&response.SurveyID,
		&response.EnrollmentID,
		&response.State,
		&channel,
		&response.StartedAt,
		&submittedAt,
	); err != nil {
		return domains.SurveyResponse{}, err
	}

	if channel.Valid {
		response.Channel = &channel.String
	}
	if submittedAt.Valid {
		t := submittedAt.Time
		response.SubmittedAt = &t
	}

	return response, nil
}

func scanAnswer(row pgx.Row) (domains.SurveyAnswer, error) {
	var (
		answer        domains.SurveyAnswer
		sectionCode   sql.NullString
		repeatPath    string
		valueText     sql.NullString
		valueNumber   sql.NullFloat64
		valueBool     sql.NullBool
		valueDate     sql.NullTime
		valueDateTime sql.NullTime
		valueJSON     []byte
	)

	if err := row.Scan(
		&answer.QuestionCode,
		&sectionCode,
		&repeatPath,
		&valueText,
		&valueNumber,
		&valueBool,
		&valueDate,
		&valueDateTime,
		&valueJSON,
	); err != nil {
		return domains.SurveyAnswer{}, err
	}

	if sectionCode.Valid {
		answer.SectionCode = &sectionCode.String
	}
	answer.RepeatPath = repeatPath
	if valueText.Valid {
		answer.ValueText = &valueText.String
	}
	if valueNumber.Valid {
		val := valueNumber.Float64
		answer.ValueNumber = &val
	}
	if valueBool.Valid {
		val := valueBool.Bool
		answer.ValueBool = &val
	}
	if valueDate.Valid {
		value := valueDate.Time
		answer.ValueDate = &value
	}
	if valueDateTime.Valid {
		value := valueDateTime.Time
		answer.ValueDateTime = &value
	}
	if len(valueJSON) > 0 {
		data := make(json.RawMessage, len(valueJSON))
		copy(data, valueJSON)
		answer.ValueJSON = data
	}

	return answer, nil
}

func toSurveyAnswer(src domains.SurveyAnswerToSave) domains.SurveyAnswer {
	result := domains.SurveyAnswer{
		QuestionCode:  src.QuestionCode,
		SectionCode:   src.SectionCode,
		RepeatPath:    src.RepeatPath,
		ValueText:     src.ValueText,
		ValueNumber:   src.ValueNumber,
		ValueBool:     src.ValueBool,
		ValueDate:     src.ValueDate,
		ValueDateTime: src.ValueDateTime,
	}
	if len(src.ValueJSON) > 0 {
		data := make(json.RawMessage, len(src.ValueJSON))
		copy(data, src.ValueJSON)
		result.ValueJSON = data
	}
	return result
}

func scanEnrollment(row pgx.Row) (domains.Enrollment, error) {
	var (
		enrollment domains.Enrollment
		email      sql.NullString
		phone      sql.NullString
		telegram   sql.NullInt64
		expires    sql.NullTime
		useLimit   int32
		usedCount  int32
	)

	if err := row.Scan(
		&enrollment.ID,
		&enrollment.SurveyID,
		&enrollment.FullName,
		&email,
		&phone,
		&telegram,
		&enrollment.State,
		&enrollment.TokenHash,
		&expires,
		&useLimit,
		&usedCount,
	); err != nil {
		return domains.Enrollment{}, err
	}

	if email.Valid {
		enrollment.Email = &email.String
	}
	if phone.Valid {
		enrollment.Phone = &phone.String
	}
	if telegram.Valid {
		value := telegram.Int64
		enrollment.TelegramChatID = &value
	}
	if expires.Valid {
		value := expires.Time
		enrollment.TokenExpiresAt = &value
	}
	enrollment.UseLimit = int(useLimit)
	enrollment.UsedCount = int(usedCount)

	return enrollment, nil
}
