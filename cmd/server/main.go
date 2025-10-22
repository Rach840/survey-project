package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"mymodule/internal/config"
	"mymodule/internal/scheduler"
	"mymodule/internal/server"
	"mymodule/internal/storage"
	"mymodule/internal/storage/providers"
	httptransport "mymodule/internal/transport/http"
)

func main() {
	cfg := config.MustLoad()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := storage.InitDB(cfg.DatabaseUrl)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	//if err := storage.Migrate(db); err != nil {
	//	log.Fatalf("failed to apply migrations: %v", err)
	//}

	allProviders := providers.New(db)
	scheduler.NewSurveyScheduler(allProviders.SurveyProvider, time.Minute).Start(ctx)

	router := httptransport.Router(allProviders, cfg)

	addr := ":" + cfg.Server.Port
	log.Printf("listening on %s", addr)
	if err := server.Start(ctx, addr, router); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
