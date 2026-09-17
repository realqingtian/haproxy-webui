// Package cryptoutil 提供实例凭据的 AES-GCM 加解密。
//
// 密钥派生(v0.6 起):
//   - HAPROXY_WEBUI_ENCRYPTION_KEY 非空:密钥仅由该独立密钥派生,换 JWT secret 不再影响凭据;
//     旧密钥(派生自 JWT secret)保留在内存中,仅供启动时的密文迁移使用。
//   - HAPROXY_WEBUI_ENCRYPTION_KEY 为空(仅限本地开发):沿用旧派生(从 JWT secret),启动时有告警,
//     STRICT 模式拒绝启动。
//   - 历史明文(无 enc: 前缀):解密原样返回,保存时自动迁移为密文。
package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const prefix = "enc:"

const legacyPrefix = "haproxy-webui:"

var (
	key       []byte // 当前加密/解密使用的密钥
	legacyKey []byte // 独立密钥模式下保留的旧派生密钥(仅迁移用),否则为 nil
)

// KeyMaterial 是启动时提供的密钥材料。
type KeyMaterial struct {
	JWTSecret     string
	EncryptionKey string // 独立凭据加密密钥,非空时优先生效
}

// Setup 在应用启动时初始化密钥(调用一次)。EncryptionKey 非空时密钥仅由它派生,
// 并保留旧派生密钥供密文迁移。
func Setup(m KeyMaterial) {
	legacyMaterial := m.JWTSecret + ":"
	if m.EncryptionKey != "" {
		key = derive("enc:" + m.EncryptionKey)
		legacyKey = derive(legacyMaterial)
		return
	}
	key = derive(legacyMaterial)
	legacyKey = nil
}

// Init 以旧派生方案(JWT secret 材料)初始化密钥。保留给测试与本地开发路径使用。
func Init(secretMaterial string) {
	key = derive(secretMaterial)
	legacyKey = nil
}

func derive(material string) []byte {
	h := sha256.Sum256([]byte(legacyPrefix + material))
	return h[:]
}

func encrypt(plaintext string) (string, error) {
	if key == nil {
		return "", errors.New("cryptoutil not initialised")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(ct), nil
}

// decryptWith 用指定密钥解密 enc: 密文。
func decryptWith(k []byte, stored string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

func DecryptStored(stored string) (string, error) {
	if !strings.HasPrefix(stored, prefix) {
		// 历史明文,直接返回
		return stored, nil
	}
	if key == nil {
		return "", errors.New("cryptoutil not initialised")
	}
	return decryptWith(key, stored)
}

// EncryptStored 加密明文凭据;空串原样返回。加密失败返回错误,由调用方决定如何响应
// (绝不静默降级为明文落库)。
func EncryptStored(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return encrypt(plaintext)
}

// DecryptStoredOrDefault 解密,失败时回退原值(兼容历史明文数据)。
func DecryptStoredOrDefault(stored string) string {
	v, err := DecryptStored(stored)
	if err != nil {
		return stored
	}
	return v
}

// TryMigrateCiphertext 尝试把旧密钥(派生自 JWT secret)加密的密文迁移为当前密钥密文。
// 返回 (新密文, 是否需要更新, 错误):
//   - 非 enc: 前缀(历史明文)或已可用当前密钥解密 → ("", false, nil)
//   - 旧密钥能解开 → 用当前密钥重新加密,返回新密文
//   - 两把钥匙都解不开 → 错误(调用方应大声记录,该凭据需手工重录)
func TryMigrateCiphertext(stored string) (string, bool, error) {
	if !strings.HasPrefix(stored, prefix) {
		return "", false, nil
	}
	if _, err := DecryptStored(stored); err == nil {
		return "", false, nil
	}
	if legacyKey == nil {
		return "", false, errors.New("ciphertext cannot be decrypted with current key and no legacy key available")
	}
	plain, err := decryptWith(legacyKey, stored)
	if err != nil {
		return "", false, fmt.Errorf("legacy key also failed: %w", err)
	}
	newStored, err := encrypt(plain)
	if err != nil {
		return "", false, err
	}
	return newStored, true, nil
}
