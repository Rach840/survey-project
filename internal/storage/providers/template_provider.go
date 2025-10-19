package providers

import (
	"context"
	"errors"
	"fmt"
	"mymodule/internal/domains"
	"mymodule/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TemplateProvider struct {
	db *pgxpool.Pool
}

func NewTemplateProvider(pg *pgxpool.Pool) *TemplateProvider {
	return &TemplateProvider{
		db: pg,
	}
}

func (s *TemplateProvider) SaveTemplate(ctx context.Context, template domains.TemplateCreate, userId int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO form_templates ( owner_id,title, description, version, status,draft_schema_json,published_schema_json,published_at)
         VALUES ($1, $2,$3,$4,$5,$6,$7, NOW())`, userId, template.Title, template.Description, template.Version, "draft", template.Section, template.Section)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return storage.ErrUserExist
		}
		return fmt.Errorf("Ошибка транзакции", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Ошибка базы данных")

	}
	return nil
}

func (s *TemplateProvider) GetAllTemplatesByUser(ctx context.Context, userId int) ([]domains.Template, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	templatesRows, err := tx.Query(ctx, `SELECT *
         FROM form_templates
         WHERE owner_id = $1`,
		int64(userId))

	templates, err := pgx.CollectRows(templatesRows, pgx.RowToStructByName[domains.Template])

	if err != nil {
		return nil, err
	}
	return templates, nil
}

func (s *TemplateProvider) GetTemplateById(ctx context.Context, userId int, templateId int) (domains.Template, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domains.Template{}, fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)
	var template domains.Template
	row, err := tx.Query(ctx, `
        SELECT *
        FROM form_templates
        WHERE id = $1 AND owner_id = $2
    `, templateId, userId)
	if err != nil {
		return domains.Template{}, err
	}
	defer row.Close()
	template, err = pgx.CollectOneRow(row, pgx.RowToStructByName[domains.Template])
	if err != nil {
		return domains.Template{}, err
	}
	return template, nil
}

func (s *TemplateProvider) UpdateTemplate(ctx context.Context, templateID int, template domains.TemplateCreate, userId int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)
	var templateId int
	err = tx.QueryRow(ctx, `SELECT id
         FROM form_templates
         WHERE id = $1 AND owner_id = $2`,
		templateID, userId).Scan(&templateId)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
    UPDATE form_templates
    SET
        title = $1,
        description = $2,
        version = $3,
        status = $4,
        draft_schema_json = $5,
        published_schema_json = $6,
        published_at = NOW()
    WHERE id = $7 AND owner_id = $8
`, template.Title, template.Description, template.Version+1, "draft", template.Section, template.Section, templateId, userId)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return storage.ErrUserExist
		}
		return fmt.Errorf("Ошибка транзакции", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Ошибка базы данных")

	}
	return nil
}
