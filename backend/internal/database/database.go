package database

import (
	"log"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"haproxy-webui/backend/internal/model"
)

func Open(path string) (*gorm.DB, error) {
	// WAL + busy_timeout:降低并发写下偶发 "database is locked" 的概率
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	return gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&model.User{}, &model.Instance{}, &model.AuditLog{}, &model.ConfigRevision{})
}

// SeedAdmin 仅在用户表为空时创建首个管理员,已有用户则不动。
func SeedAdmin(db *gorm.DB, username, password string) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	log.Printf("seeded admin user %q (initial password), change it after first login", username)
	return db.Create(&model.User{Username: username, PasswordHash: string(hash), Role: model.RoleAdmin}).Error
}
