package providers

import (
	"context"
	"fmt"
	"mymodule/internal/domains"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserProvider struct {
	db *pgxpool.Pool
}

func (u UserProvider) GetAllUser(ctx context.Context) ([]domains.Questioner, error) {
	tx, err := u.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Ошибка начала транзакции")
	}
	defer tx.Rollback(ctx)

	userRows, err := tx.Query(ctx, `SELECT *
         FROM accounts`)

	users, err := pgx.CollectRows(userRows, pgx.RowToStructByName[domains.Questioner])

	if err != nil {
		return nil, err
	}
	return users, nil
}

func NewUserProvider(db *pgxpool.Pool) *UserProvider {
	return &UserProvider{
		db: db,
	}
}
