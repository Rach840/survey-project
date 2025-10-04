package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var migrationPath, database_url string
	flag.StringVar(&database_url, "database_url", "", "Path to database URL")
	flag.StringVar(&migrationPath, "migration-path", "", "Path to store the migrations")
	flag.Parse()

	if database_url == "" {
		panic("database URL is required")
	}
	if migrationPath == "" {
		panic("migrationPath is required")
	}

	m, err := migrate.New("file://"+migrationPath, fmt.Sprintf("postgres://%s", database_url))

	if err != nil {
		panic(err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("no migrations to apply")

			return
		}

		panic(err)
	}

	fmt.Println("Migrations applied")
}
