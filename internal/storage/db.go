package storage

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB(storagPath string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pgxCfg, err := pgxpool.ParseConfig(storagPath)
	if err != nil {
		log.Fatal(" Ошибка парсинга строки подключения:", err)
	}
	pgxCfg.MaxConns = 1
	pgxCfg.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err := pool.Ping(ctx); err != nil {
		fmt.Println("Ошибка подключения к базе данных ")
		slog.Error(err.Error())
		return nil, err
	}

	log.Println("Подключение к PostgresSQL успешно")
	return pool, nil

}
