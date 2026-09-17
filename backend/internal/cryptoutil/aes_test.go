package cryptoutil

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	Init("unit-test-material")
	enc := EncryptStored("s3cret--password")
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
	if enc := EncryptStored(""); enc != "" {
		t.Fatalf("empty should stay empty, got %q", enc)
	}
}

func TestWrongKeyFails(t *testing.T) {
	Init("material-a")
	enc := EncryptStored("data")
	Init("material-b") // 换密钥后应解密失败
	if _, err := DecryptStored(enc); err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}
