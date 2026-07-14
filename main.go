package main

import (
	"log"

	"xfile/internal/config"
	"xfile/internal/database"
	"xfile/internal/server"
	"xfile/internal/store"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	appStore := store.New(db, cfg)
	app := server.New(cfg, appStore)

	log.Printf("xfile listening on %s", cfg.Addr)
	if err := app.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
