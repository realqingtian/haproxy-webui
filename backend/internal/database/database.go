package database

import (
	"log"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"haproxy-webui/backend/internal/cryptoutil"
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
	return db.AutoMigrate(&model.User{}, &model.Instance{}, &model.AuditLog{}, &model.ConfigRevision{}, &model.Cluster{}, &model.Setting{}, &model.AlertChannel{})
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

// MigrateCredentialCiphertext 在配置了独立加密密钥(ENCRYPTION_KEY)后首次启动时,
// 把仍由旧派生密钥(JWT secret)加密的实例凭据自动重加密为当前密钥,幂等可重复执行。
// 无法解密的密文(两把钥匙都失败)逐条大声记录,相关凭据需手工重录。
func MigrateCredentialCiphertext(db *gorm.DB) error {
	var insts []model.Instance
	if err := db.Where("password LIKE ?", "enc:%").Find(&insts).Error; err != nil {
		return err
	}
	migrated, failed := 0, 0
	for _, inst := range insts {
		newStored, changed, err := cryptoutil.TryMigrateCiphertext(inst.Password)
		if err != nil {
			failed++
			log.Printf("credential migration: instance %q (id=%d) cannot be decrypted, please re-enter its password: %v",
				inst.Name, inst.ID, err)
			continue
		}
		if changed {
			if err := db.Model(&model.Instance{}).Where("id = ?", inst.ID).Update("password", newStored).Error; err != nil {
				return err
			}
			migrated++
		}
	}
	switch {
	case migrated > 0 && failed > 0:
		log.Printf("credential migration: %d re-encrypted, %d failed (see logs above)", migrated, failed)
	case migrated > 0:
		log.Printf("credential migration: %d credential(s) re-encrypted with the dedicated encryption key", migrated)
	case failed > 0:
		log.Printf("credential migration: %d credential(s) failed, please re-enter their passwords", failed)
	}
	return nil
}
