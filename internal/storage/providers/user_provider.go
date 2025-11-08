package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mymodule/internal/domains"
	"mymodule/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserProvider struct {
	db *pgxpool.Pool
}

func (u UserProvider) GetAllUser(ctx context.Context) ([]domains.User, error) {
	tx, err := u.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	userRows, err := tx.Query(ctx, `SELECT id ,email,full_name,role,created_at,disabled_at FROM accounts`)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	users, err := pgx.CollectRows(userRows, pgx.RowToStructByName[domains.User])

	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	return users, nil
}

func (s UserProvider) SaveUser(ctx context.Context, passHash string, User domains.Questioner) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO accounts ( full_name,email, role, passhash, created_at)
         VALUES ($1, $2,$3,$4, NOW())`, User.FullName, User.Email, "QUESTIONER", passHash)

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

func (u UserProvider) UpdateUser(ctx context.Context, user domains.Questioner) error {
	tx, err := u.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE INTO accounts ( full_name,email, role, passhash, created_at)
         VALUES ($1, $2,$3,$4, NOW())`)

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

func NewUserProvider(db *pgxpool.Pool) *UserProvider {
	return &UserProvider{
		db: db,
	}
}
