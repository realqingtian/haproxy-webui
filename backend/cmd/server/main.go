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

	// 严格生产模式:未自定义密钥时拒绝启动,避免带弱密钥上线
	if cfg.Strict {
		if cfg.JWTSecret == "dev-insecure-secret" {
			log.Fatal("STRICT: HAPROXY_WEBUI_JWT_SECRET 未设置,拒绝以默认密钥启动")
		}
		if cfg.EncryptionKey == "" {
			log.Fatal("STRICT: HAPROXY_WEBUI_ENCRYPTION_KEY 未设置,拒绝以派生密钥启动")
		}
	}

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
