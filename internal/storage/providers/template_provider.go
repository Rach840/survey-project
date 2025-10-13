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
