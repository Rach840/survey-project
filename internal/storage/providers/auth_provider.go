package providers

import (
	"context"
	"fmt"
	"mymodule/internal/domains"

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

func (s *AuthProvider) GetUserByID(ctx context.Context, id int) (domains.Questioner, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domains.Questioner{}, fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	var user domains.Questioner
	err = tx.QueryRow(ctx,
		`SELECT id, full_name, email,passhash,role
         FROM accounts
         WHERE id = $1`,
		int64(id),
	).Scan(
		&user.Id,
		&user.FullName,
		&user.Email,
		&user.Password,
		&user.Role,
	)
	if err != nil {
		return domains.Questioner{}, err
	}
	return user, nil
}

func (s *AuthProvider) GetUserByEmail(ctx context.Context, email string) (domains.Questioner, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domains.Questioner{}, fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	var user domains.Questioner
	err = tx.QueryRow(ctx,
		`SELECT id, full_name, email,passhash
         FROM accounts
         WHERE email = $1`,
		email,
	).Scan(
		&user.Id,
		&user.FullName,
		&user.Email,
		&user.Password,
	)
	if err != nil {
		return domains.Questioner{}, err
	}
	return user, nil
}
