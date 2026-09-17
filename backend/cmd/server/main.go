package main

import (
	"log"

	"haproxy-webui/backend/internal/api"
	"haproxy-webui/backend/internal/config"
	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/database"
)

func main() {
	cfg := config.Load()
	cryptoutil.Init(cfg.JWTSecret + ":" + cfg.EncryptionKey)

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	if err := database.SeedAdmin(db, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("seed admin user: %v", err)
	}

	r := api.NewRouter(&cfg, db)
	addr := ":" + cfg.Port
	log.Printf("haproxy-webui backend listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
