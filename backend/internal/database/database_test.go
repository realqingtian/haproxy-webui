package database

import (
	"path/filepath"
	"testing"

	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/model"
)

// 配置独立加密密钥后的启动迁移:旧派生密钥的密文被自动重加密,幂等。
func TestMigrateCredentialCiphertext(t *testing.T) {
	cryptoutil.Init("jwt-old:") // 旧派生方案
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	oldEnc, err := cryptoutil.EncryptStored("node-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Instance{Name: "legacy-node", BaseURL: "http://x:5555", Username: "u", Password: oldEnc, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}

	// 切换到独立密钥后迁移
	cryptoutil.Setup(cryptoutil.KeyMaterial{JWTSecret: "jwt-old", EncryptionKey: "dedicated-key"})
	if err := MigrateCredentialCiphertext(db); err != nil {
		t.Fatal(err)
	}

	var inst model.Instance
	if err := db.Where("name = ?", "legacy-node").First(&inst).Error; err != nil {
		t.Fatal(err)
	}
	if got, err := cryptoutil.DecryptStored(inst.Password); err != nil || got != "node-secret" {
		t.Fatalf("after migration decrypt: got %q err %v", got, err)
	}
	if inst.Password == oldEnc {
		t.Fatal("ciphertext should have been re-encrypted")
	}

	// 幂等:重复执行无副作用
	if err := MigrateCredentialCiphertext(db); err != nil {
		t.Fatal(err)
	}
	var inst2 model.Instance
	if err := db.Where("name = ?", "legacy-node").First(&inst2).Error; err != nil {
		t.Fatal(err)
	}
	if got, err := cryptoutil.DecryptStored(inst2.Password); err != nil || got != "node-secret" {
		t.Fatalf("after idempotent re-run: got %q err %v", got, err)
	}
}
