package main

import (
	"context"
	"errors"
	"log"
	"mymodule/internal/config"
	"mymodule/internal/server"
	"mymodule/internal/storage"
	httptransport "mymodule/internal/transport/http"
	"net/http"
)

func main() {
	cfg := config.MustLoad()

	db, err := storage.InitDB(cfg.DatabaseUrl)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	//if err := storage.Migrate(db); err != nil {
	//	log.Fatalf("failed to apply migrations: %v", err)
	//}

	router := httptransport.Router(db, cfg)

	addr := ":" + cfg.Server.Port
	log.Printf("listening on %s", addr)
	if err := server.Start(context.Background(), addr, router); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
