package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"haproxy-webui/backend/internal/api"
	"haproxy-webui/backend/internal/config"
	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/database"
	"haproxy-webui/backend/internal/scheduler"
)

func main() {
	cfg := config.Load()
	// 密钥派生:ENCRYPTION_KEY 非空时仅由它派生(换 JWT secret 不影响凭据);
	// 为空时回落旧派生并告警,STRICT 模式拒绝启动
	cryptoutil.Setup(cryptoutil.KeyMaterial{JWTSecret: cfg.JWTSecret, EncryptionKey: cfg.EncryptionKey})
	if cfg.EncryptionKey == "" {
		log.Println("WARNING: HAPROXY_WEBUI_ENCRYPTION_KEY not set, deriving credential key from JWT secret")
	}

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
	if err := database.MigrateCredentialCiphertext(db); err != nil {
		log.Printf("credential ciphertext migration: %v", err)
	}
	if err := database.SeedAdmin(db, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("seed admin user: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := api.NewRouter(&cfg, db)
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		log.Printf("haproxy-webui backend listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("run server: %v", err)
		}
	}()

	// 定时任务随信号退出,再收尾 HTTP(给在途请求 5 秒)
	scheduler.New(db).Run(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
	log.Println("backend exited")
}
