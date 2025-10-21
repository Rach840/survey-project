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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

	row, err := tx.Query(ctx, insertSurvey,
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
	if err != nil {
		return domains.Survey{}, nil, err
	}
	defer row.Close()
	created, err := pgx.CollectOneRow(row, pgx.RowToStructByName[domains.Survey])
	if err != nil {
		return domains.Survey{}, nil, fmt.Errorf("insert survey: %w", err)
	}

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
			SurveyID:       created.ID,
			EnrollmentID:   enrollmentID,
			OwnerID:        created.OwnerID,
			Enrollment:     participant,
			SurveyStartsAt: created.StartsAt,
			SurveyEndsAt:   created.EndsAt,
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

func (s SurveyProvider) AddEnrollments(ctx context.Context, surveyID, ownerID int64, participant domains.EnrollmentCreate, generator domains.EnrollmentTokenGenerator) (domains.EnrollmentInvitation, error) {

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domains.EnrollmentInvitation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	surveyWindow := struct {
		StartsAt *time.Time
		EndsAt   *time.Time
	}{}
	if err := tx.QueryRow(ctx,
		`SELECT starts_at, ends_at FROM surveys WHERE id = $1 AND owner_id = $2`,
		surveyID,
		ownerID,
	).Scan(&surveyWindow.StartsAt, &surveyWindow.EndsAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domains.EnrollmentInvitation{}, fmt.Errorf("verify survey ownership: %w", storage.ErrNotFound)
		}
		return domains.EnrollmentInvitation{}, fmt.Errorf("verify survey ownership: %w", err)
	}

	const insertEnrollment = `
          INSERT INTO enrollments (
              survey_id, source, full_name, email, phone,
              telegram_chat_id, state, invited_by
          ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
          RETURNING id`

	var enrollmentID int64

	err = tx.QueryRow(ctx, insertEnrollment,
		surveyID,
		"admin",
		participant.FullName,
		participant.Email,
		participant.Phone,
		participant.TelegramChatID,
		"invited",
		ownerID,
	).Scan(&enrollmentID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domains.EnrollmentInvitation{}, fmt.Errorf("insert enrollment: %w", storage.ErrConflict)
		}
		return domains.EnrollmentInvitation{}, fmt.Errorf("insert enrollment: %w", err)
	}

	token, hash, expiresAt, err := generator(domains.EnrollmentTokenPayload{
		SurveyID:       surveyID,
		EnrollmentID:   enrollmentID,
		OwnerID:        ownerID,
		Enrollment:     participant,
		SurveyStartsAt: surveyWindow.StartsAt,
		SurveyEndsAt:   surveyWindow.EndsAt,
	})
	if err != nil {
		return domains.EnrollmentInvitation{}, fmt.Errorf("generate token: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE enrollments SET token_hash = $1, token_expires_at = $2 WHERE id = $3`,
		hash, expiresAt, enrollmentID,
	); err != nil {
		return domains.EnrollmentInvitation{}, fmt.Errorf("update enrollment token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domains.EnrollmentInvitation{}, fmt.Errorf("commit: %w", err)
	}

	return domains.EnrollmentInvitation{
		EnrollmentID: enrollmentID,
		Token:        token,
		ExpiresAt:    expiresAt,
		FullName:     participant.FullName,
		Email:        participant.Email,
	}, nil
}

func (s SurveyProvider) UpdateSurvey(ctx context.Context, surveyID, ownerID int64, update domains.SurveyUpdate) (domains.Survey, error) {
	setClauses := make([]string, 0, 7)
	args := make([]interface{}, 0, 9)
	idx := 1

	if update.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", idx))
		args = append(args, *update.Title)
		idx++
	}
	if update.Mode != nil {
		setClauses = append(setClauses, fmt.Sprintf("mode = $%d", idx))
		args = append(args, *update.Mode)
		idx++
	}
	if update.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", idx))
		args = append(args, *update.Status)
		idx++
	}
	if update.MaxParticipants.Present {
		setClauses = append(setClauses, fmt.Sprintf("max_participants = $%d", idx))
		if update.MaxParticipants.Value != nil {
			args = append(args, *update.MaxParticipants.Value)
		} else {
			args = append(args, nil)
		}
		idx++
	}
	if update.PublicSlug.Present {
		setClauses = append(setClauses, fmt.Sprintf("public_slug = $%d", idx))
		if update.PublicSlug.Value != nil {
			args = append(args, *update.PublicSlug.Value)
		} else {
			args = append(args, nil)
		}
		idx++
	}
	if update.StartsAt.Present {
		setClauses = append(setClauses, fmt.Sprintf("starts_at = $%d", idx))
		if update.StartsAt.Value != nil {
			args = append(args, *update.StartsAt.Value)
		} else {
			args = append(args, nil)
		}
		idx++
	}
	if update.EndsAt.Present {
		setClauses = append(setClauses, fmt.Sprintf("ends_at = $%d", idx))
		if update.EndsAt.Value != nil {
			args = append(args, *update.EndsAt.Value)
		} else {
			args = append(args, nil)
		}
		idx++
	}

	if len(setClauses) == 0 {
		return s.GetSurveyByID(ctx, int(ownerID), int(surveyID))
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, surveyID, ownerID)
	query := fmt.Sprintf(`
		UPDATE surveys
		SET %s
		WHERE id = $%d AND owner_id = $%d
		RETURNING
			id, owner_id, template_id, snapshot_version,
			form_snapshot_json, title, mode, status,
			max_participants, public_slug, starts_at, ends_at, created_at`,
		strings.Join(setClauses, ", "), idx, idx+1,
	)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return domains.Survey{}, fmt.Errorf("update survey: %w", err)
	}
	defer rows.Close()

	updated, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domains.Survey])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domains.Survey{}, fmt.Errorf("update survey: %w", storage.ErrNotFound)
		}
		return domains.Survey{}, fmt.Errorf("update survey: %w", err)
	}

	return updated, nil
}

func (s SurveyProvider) GetAllSurveysByUser(ctx context.Context, userId int64) ([]domains.SurveySummary, error) {
	const query = `
		SELECT
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
			s.created_at,
			COALESCE(stats.total_enrollments, 0) AS total_enrollments,
			COALESCE(stats.responses_started, 0) AS responses_started,
			COALESCE(stats.responses_submitted, 0) AS responses_submitted,
			COALESCE(stats.responses_in_progress, 0) AS responses_in_progress,
			stats.avg_completion_seconds
		FROM surveys s
		LEFT JOIN LATERAL (
			SELECT
				COUNT(e.id) AS total_enrollments,
				COUNT(r.id) AS responses_started,
				COUNT(*) FILTER (WHERE r.state = 'submitted') AS responses_submitted,
				COUNT(*) FILTER (WHERE r.state = 'in_progress') AS responses_in_progress,
				AVG(EXTRACT(EPOCH FROM (r.submitted_at - r.started_at))) AS avg_completion_seconds
			FROM enrollments e
			LEFT JOIN responses r ON r.enrollment_id = e.id
			WHERE e.survey_id = s.id
		) AS stats ON true
		WHERE s.owner_id = $1
		ORDER BY s.created_at DESC`

	rows, err := s.db.Query(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("list surveys: %w", err)
	}
	defer rows.Close()

	var result []domains.SurveySummary

	for rows.Next() {
		var (
			survey          domains.Survey
			total           int
			started         int
			submitted       int
			inProgress      int
			templateID      sql.NullInt64
			formSnapshot    []byte
			maxParticipants sql.NullInt64
			publicSlug      sql.NullString
			startsAt        *time.Time
			endsAt          *time.Time
			avgSeconds      sql.NullFloat64
		)

		if err := rows.Scan(
			&survey.ID,
			&survey.OwnerID,
			&templateID,
			&survey.SnapshotVersion,
			&formSnapshot,
			&survey.Title,
			&survey.Mode,
			&survey.Status,
			&maxParticipants,
			&publicSlug,
			&startsAt,
			&endsAt,
			&survey.CreatedAt,
			&total,
			&started,
			&submitted,
			&inProgress,
			&avgSeconds,
		); err != nil {
			return nil, fmt.Errorf("scan survey summary: %w", err)
		}

		if templateID.Valid {
			value := templateID.Int64
			survey.TemplateID = &value
		} else {
			survey.TemplateID = nil
		}
		if len(formSnapshot) > 0 {
			data := make(json.RawMessage, len(formSnapshot))
			copy(data, formSnapshot)
			survey.FormSnapshotJSON = data
		} else {
			survey.FormSnapshotJSON = nil
		}
		if maxParticipants.Valid {
			value := int(maxParticipants.Int64)
			survey.MaxParticipants = &value
		} else {
			survey.MaxParticipants = nil
		}
		if publicSlug.Valid {
			slug := publicSlug.String
			survey.PublicSlug = &slug
		} else {
			survey.PublicSlug = nil
		}
		survey.StartsAt = startsAt
		survey.EndsAt = endsAt

		counts := domains.SurveyStatisticsCounts{
			TotalEnrollments:    total,
			ResponsesStarted:    started,
			ResponsesSubmitted:  submitted,
			ResponsesInProgress: inProgress,
		}
		if avgSeconds.Valid {
			value := avgSeconds.Float64
			counts.AverageCompletionSeconds = &value
		}

		result = append(result, domains.SurveySummary{
			Survey:     survey,
			Statistics: counts.ToSurveyStatistics(),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate surveys: %w", err)
	}

	return result, nil
}

func (s SurveyProvider) GetSurveyByID(ctx context.Context, ownerID int, surveyID int) (domains.Survey, error) {
	const query = `
		SELECT
			id, owner_id, template_id, snapshot_version,
			form_snapshot_json, title, mode, status,
			max_participants, public_slug, starts_at, ends_at, created_at
		FROM surveys
		WHERE owner_id = $1 AND id = $2`

	row, err := s.db.Query(ctx, query, ownerID, surveyID)
	survey, err := pgx.CollectOneRow(row, pgx.RowToStructByName[domains.Survey])
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

	enrollments, err := pgx.CollectRows(rows, pgx.RowToStructByName[domains.Enrollment])
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

func (s SurveyProvider) RemoveEnrollment(ctx context.Context, surveyID, ownerID, enrollmentID int64) error {
	const query = `
		UPDATE enrollments e
		SET state = 'removed',
		    token_hash = NULL,
		    token_expires_at = NULL
		FROM surveys s
		WHERE e.id = $1
		  AND e.survey_id = $2
		  AND s.id = e.survey_id
		  AND s.owner_id = $3
		RETURNING e.id`

	var updatedID int64
	if err := s.db.QueryRow(ctx, query, enrollmentID, surveyID, ownerID).Scan(&updatedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("remove enrollment: %w", storage.ErrNotFound)
		}
		return fmt.Errorf("remove enrollment: %w", err)
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

	row, _ := tx.Query(ctx, upsert,
		payload.SurveyID,
		payload.EnrollmentID,
		payload.State,
		channel,
		payload.SubmittedAt,
	)

	response, err := pgx.CollectOneRow(row, pgx.RowToStructByName[domains.SurveyResponse])

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

	row, err := s.db.Query(ctx, query, enrollmentID)
	response, err := pgx.CollectOneRow(row, pgx.RowToStructByName[domains.SurveyResponse])
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

func (s SurveyProvider) ListSurveyResults(ctx context.Context, ownerID int64, surveyID int64) ([]domains.SurveyResult, error) {
	const responsesQuery = `
		SELECT
			r.id,
			r.survey_id,
			r.enrollment_id,
			r.state,
			r.channel,
			r.started_at,
			r.submitted_at,
			e.id,
			e.survey_id,
			e.full_name,
			e.email,
			e.phone,
			e.telegram_chat_id,
			e.state,
			e.token_expires_at,
			e.use_limit,
			e.used_count
		FROM responses r
		JOIN enrollments e ON e.id = r.enrollment_id
		JOIN surveys s ON s.id = r.survey_id
		WHERE s.owner_id = $1 AND s.id = $2
		ORDER BY r.started_at`

	rows, err := s.db.Query(ctx, responsesQuery, ownerID, surveyID)
	if err != nil {
		return nil, fmt.Errorf("list survey responses: %w", err)
	}
	defer rows.Close()

	type responseItem struct {
		Response   domains.SurveyResponse
		Enrollment domains.Enrollment
	}

	items := make([]responseItem, 0)
	responseIDs := make([]int64, 0)

	for rows.Next() {
		var (
			item         responseItem
			channel      sql.NullString
			submittedAt  sql.NullTime
			email        sql.NullString
			phone        sql.NullString
			telegram     sql.NullInt64
			tokenExpires sql.NullTime
		)

		if err := rows.Scan(
			&item.Response.ID,
			&item.Response.SurveyID,
			&item.Response.EnrollmentID,
			&item.Response.State,
			&channel,
			&item.Response.StartedAt,
			&submittedAt,
			&item.Enrollment.ID,
			&item.Enrollment.SurveyID,
			&item.Enrollment.FullName,
			&email,
			&phone,
			&telegram,
			&item.Enrollment.State,
			&tokenExpires,
			&item.Enrollment.UseLimit,
			&item.Enrollment.UsedCount,
		); err != nil {
			return nil, fmt.Errorf("scan survey response: %w", err)
		}

		if channel.Valid {
			value := channel.String
			item.Response.Channel = &value
		}
		if submittedAt.Valid {
			t := submittedAt.Time
			item.Response.SubmittedAt = &t
		}
		if email.Valid {
			value := email.String
			item.Enrollment.Email = &value
		}
		if phone.Valid {
			value := phone.String
			item.Enrollment.Phone = &value
		}
		if telegram.Valid {
			id := telegram.Int64
			item.Enrollment.TelegramChatID = &id
		}
		if tokenExpires.Valid {
			t := tokenExpires.Time
			item.Enrollment.TokenExpiresAt = &t
		}

		items = append(items, item)
		responseIDs = append(responseIDs, item.Response.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate survey responses: %w", err)
	}

	if len(items) == 0 {
		return nil, nil
	}

	const answersQuery = `
		SELECT
			response_id,
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
		WHERE response_id = ANY($1)
		ORDER BY response_id, question_code, repeat_path`

	answerRows, err := s.db.Query(ctx, answersQuery, responseIDs)
	if err != nil {
		return nil, fmt.Errorf("list survey answers: %w", err)
	}
	defer answerRows.Close()

	answersByResponse := make(map[int64][]domains.SurveyAnswer, len(responseIDs))

	for answerRows.Next() {
		var (
			responseID    int64
			questionCode  string
			sectionCode   sql.NullString
			repeatPath    string
			valueText     sql.NullString
			valueNumber   sql.NullFloat64
			valueBool     sql.NullBool
			valueDate     sql.NullTime
			valueDateTime sql.NullTime
			valueJSON     []byte
		)

		if err := answerRows.Scan(
			&responseID,
			&questionCode,
			&sectionCode,
			&repeatPath,
			&valueText,
			&valueNumber,
			&valueBool,
			&valueDate,
			&valueDateTime,
			&valueJSON,
		); err != nil {
			return nil, fmt.Errorf("scan survey answer: %w", err)
		}

		answer := mapSurveyAnswer(questionCode, sectionCode, repeatPath, valueText, valueNumber, valueBool, valueDate, valueDateTime, valueJSON)
		answersByResponse[responseID] = append(answersByResponse[responseID], answer)
	}
	if err := answerRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate survey answers: %w", err)
	}

	results := make([]domains.SurveyResult, 0, len(items))
	for _, item := range items {
		results = append(results, domains.SurveyResult{
			Enrollment: item.Enrollment,
			Response:   item.Response,
			Answers:    answersByResponse[item.Response.ID],
		})
	}

	return results, nil
}

func (s SurveyProvider) GetSurveyStatistics(ctx context.Context, ownerID int64, surveyID int64) (domains.SurveyStatisticsCounts, error) {
	const query = `
		SELECT
			COUNT(e.id) AS total_enrollments,
			COUNT(r.id) AS responses_started,
			COUNT(*) FILTER (WHERE r.state = 'submitted') AS responses_submitted,
			COUNT(*) FILTER (WHERE r.state = 'in_progress') AS responses_in_progress,
			AVG(EXTRACT(EPOCH FROM (r.submitted_at - r.started_at))) FILTER (WHERE r.submitted_at IS NOT NULL) AS avg_completion_seconds
		FROM surveys s
		LEFT JOIN enrollments e ON e.survey_id = s.id
		LEFT JOIN responses r ON r.enrollment_id = e.id
		WHERE s.owner_id = $1 AND s.id = $2`

	row := s.db.QueryRow(ctx, query, ownerID, surveyID)

	var (
		stats domains.SurveyStatisticsCounts
		avg   sql.NullFloat64
	)

	if err := row.Scan(
		&stats.TotalEnrollments,
		&stats.ResponsesStarted,
		&stats.ResponsesSubmitted,
		&stats.ResponsesInProgress,
		&avg,
	); err != nil {
		return domains.SurveyStatisticsCounts{}, fmt.Errorf("get survey statistics: %w", err)
	}

	if avg.Valid {
		value := avg.Float64
		stats.AverageCompletionSeconds = &value
	}

	return stats, nil
}
func (s SurveyProvider) GetEnrollmentByID(ctx context.Context, enrollmentID int) (domains.Enrollment, error) {
	slog.Info("enrollmentID", enrollmentID)
	const query = `
		SELECT 	e.id,
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
		WHERE id = $1`
	rows, err := s.db.Query(ctx, query, enrollmentID)
	if err != nil {
		return domains.Enrollment{}, fmt.Errorf("enrollment: %w", err)
	}
	defer rows.Close()

	enrollment, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domains.Enrollment])
	if err := rows.Err(); err != nil {
		return domains.Enrollment{}, fmt.Errorf("iterate enrollments: %w", err)
	}
	return enrollment, nil
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

func scanAnswer(row pgx.Row) (domains.SurveyAnswer, error) {
	var (
		questionCode  string
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
		&questionCode,
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

	return mapSurveyAnswer(questionCode, sectionCode, repeatPath, valueText, valueNumber, valueBool, valueDate, valueDateTime, valueJSON), nil
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

func mapSurveyAnswer(
	questionCode string,
	sectionCode sql.NullString,
	repeatPath string,
	valueText sql.NullString,
	valueNumber sql.NullFloat64,
	valueBool sql.NullBool,
	valueDate sql.NullTime,
	valueDateTime sql.NullTime,
	valueJSON []byte,
) domains.SurveyAnswer {
	answer := domains.SurveyAnswer{
		QuestionCode: questionCode,
		RepeatPath:   repeatPath,
	}
	if sectionCode.Valid {
		answer.SectionCode = &sectionCode.String
	}
	if valueText.Valid {
		answer.ValueText = &valueText.String
	}
	if valueNumber.Valid {
		value := valueNumber.Float64
		answer.ValueNumber = &value
	}
	if valueBool.Valid {
		value := valueBool.Bool
		answer.ValueBool = &value
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

	return answer
}
