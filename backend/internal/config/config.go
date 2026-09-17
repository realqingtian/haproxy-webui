package config

import (
	"log"
	"os"
	"strings"
)

// Config 是后端运行配置,全部来自环境变量,均有开发用默认值。
type Config struct {
	Port          string // 监听端口
	DBPath        string // SQLite 文件路径
	JWTSecret     string // JWT 签名密钥,生产必须覆盖
	AdminUsername string // 首次启动种子管理员
	AdminPassword string
	EncryptionKey string // 实例凭据加密密钥;缺省从 JWT secret 派生
	Strict        bool   // 严格生产模式:未自定义 JWT secret / 加密密钥时拒绝启动
}

func Load() Config {
	cfg := Config{
		Port:          env("HAPROXY_WEBUI_PORT", "8080"),
		DBPath:        env("HAPROXY_WEBUI_DB", "./data/haproxy-webui.db"),
		JWTSecret:     env("HAPROXY_WEBUI_JWT_SECRET", "dev-insecure-secret"),
		AdminUsername: env("HAPROXY_WEBUI_ADMIN_USER", "admin"),
		AdminPassword: env("HAPROXY_WEBUI_ADMIN_PASSWORD", "admin123"),
		EncryptionKey: env("HAPROXY_WEBUI_ENCRYPTION_KEY", ""),
		Strict:        envBool("HAPROXY_WEBUI_STRICT"),
	}
	if cfg.JWTSecret == "dev-insecure-secret" && !cfg.Strict {
		log.Println("WARNING: using default JWT secret, set HAPROXY_WEBUI_JWT_SECRET in production")
	}
	if cfg.EncryptionKey == "" {
		log.Println("WARNING: HAPROXY_WEBUI_ENCRYPTION_KEY not set, deriving credential key from JWT secret")
	}
	return cfg
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBool 判断布尔型环境变量是否为真值(1/true/yes,大小写不敏感)。
func envBool(key string) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes":
		return true
	}
	return false
}
