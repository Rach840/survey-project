package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB(storagPath string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		return nil, err
	}

	log.Println("Подключение к PostgresSQL успешно")
	return pool, nil

}
