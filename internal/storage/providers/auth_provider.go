package providers

import (
	"context"
	"errors"
	"fmt"
	"mymodule/internal/domains"
	"mymodule/internal/storage"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthProvider struct {
	db *pgxpool.Pool
}

func NewAuthProvider(pg *pgxpool.Pool) *AuthProvider {
	return &AuthProvider{
		db: pg,
	}
}

func (s *AuthProvider) SaveUser(ctx context.Context, passHash string, User domains.Questioner) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO accounts ( full_name,email, role, passhash, created_at)
         VALUES ($1, $2,$3,$4, NOW())`, User.FullName, User.Email, passHash, "QUESTIONER")
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return storage.ErrUserExist
		}
		return fmt.Errorf("Ошибка транзакции")
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Ошибка базы данных")

	}
	return nil
}
