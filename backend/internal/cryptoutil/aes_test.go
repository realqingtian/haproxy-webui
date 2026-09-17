package cryptoutil

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	Init("unit-test-material")
	enc, err := EncryptStored("s3cret--password")
	if err != nil {
		t.Fatal(err)
	}
	if enc == "s3cret--password" {
		t.Fatal("ciphertext should differ from plaintext")
	}
	got, err := DecryptStored(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != "s3cret--password" {
		t.Fatalf("round trip mismatch: %q", got)
	}
}

func TestDecryptStoredLegacyPlaintext(t *testing.T) {
	Init("unit-test-material")
	// 历史明文(无 enc: 前缀)应原样返回
	got, err := DecryptStored("legacy-plain")
	if err != nil || got != "legacy-plain" {
		t.Fatalf("legacy plaintext: got %q err %v", got, err)
	}
	if DecryptStoredOrDefault("legacy-plain") != "legacy-plain" {
		t.Fatal("default should fall back to original value")
	}
}

func TestEncryptStoredEmpty(t *testing.T) {
	Init("unit-test-material")
	enc, err := EncryptStored("")
	if err != nil || enc != "" {
		t.Fatalf("empty should stay empty, got %q err %v", enc, err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	Init("material-a")
	enc, _ := EncryptStored("data")
	Init("material-b") // 换密钥后应解密失败
	if _, err := DecryptStored(enc); err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

// v0.6 技术债核心行为:独立密钥生效后,轮换 JWT secret 不影响凭据解密。
func TestIndependentKeySurvivesJWTRotation(t *testing.T) {
	Setup(KeyMaterial{JWTSecret: "jwt-v1", EncryptionKey: "enc-key-1"})
	enc, err := EncryptStored("node-password")
	if err != nil {
		t.Fatal(err)
	}

	// 轮换 JWT secret(独立密钥不变):凭据仍可解
	Setup(KeyMaterial{JWTSecret: "jwt-v2", EncryptionKey: "enc-key-1"})
	if got, err := DecryptStored(enc); err != nil || got != "node-password" {
		t.Fatalf("after jwt rotation: got %q err %v", got, err)
	}

	// 独立密钥也换了解不开(需要重新录入或旧钥迁移)
	Setup(KeyMaterial{JWTSecret: "jwt-v2", EncryptionKey: "enc-key-2"})
	if _, err := DecryptStored(enc); err == nil {
		t.Fatal("different encryption key should not decrypt")
	}
}

// 启动迁移:旧派生密钥(JWT)的密文,在配置独立密钥后首次启动应能自动重加密。
func TestTryMigrateCiphertext(t *testing.T) {
	// 旧时代:仅有 JWT secret 派生密钥
	Init("jwt-v1:")
	legacyEnc, err := EncryptStored("node-password")
	if err != nil {
		t.Fatal(err)
	}

	// 新时代:配置了独立密钥
	Setup(KeyMaterial{JWTSecret: "jwt-v1", EncryptionKey: "independent-key"})
	if _, err := DecryptStored(legacyEnc); err == nil {
		t.Fatal("legacy ciphertext should not decrypt before migration")
	}

	newStored, changed, err := TryMigrateCiphertext(legacyEnc)
	if err != nil || !changed {
		t.Fatalf("migrate: changed=%v err=%v", changed, err)
	}
	if got, err := DecryptStored(newStored); err != nil || got != "node-password" {
		t.Fatalf("after migration: got %q err %v", got, err)
	}

	// 已是当前密钥的密文:无需再迁移
	if _, changed, err := TryMigrateCiphertext(newStored); changed || err != nil {
		t.Fatalf("current ciphertext should not migrate: changed=%v err=%v", changed, err)
	}

	// 彻底无解的密文应报错
	Setup(KeyMaterial{JWTSecret: "jwt-other", EncryptionKey: "enc-other"})
	if _, _, err := TryMigrateCiphertext(legacyEnc); err == nil {
		t.Fatal("undecryptable ciphertext should error")
	}
}
