// Package cryptoutil 提供实例凭据的 AES-GCM 加解密。
// 密钥来源:优先 HAPROXY_WEBUI_ENCRYPTION_KEY;未设置时从 JWT secret 派生(SHA-256)。
// 兼容历史明文:解密失败时原样返回,保存时会自动迁移为密文。
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

var key []byte

// Init 设置用于派生 AES 密钥的主密钥材料(在应用启动时调用一次)。
func Init(secretMaterial string) {
	h := sha256.Sum256([]byte("haproxy-webui:" + secretMaterial))
	key = h[:]
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

func DecryptStored(stored string) (string, error) {
	if !strings.HasPrefix(stored, prefix) {
		// 历史明文,直接返回
		return stored, nil
	}
	if key == nil {
		return "", errors.New("cryptoutil not initialised")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
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

// EncryptStored 加密明文凭据;空串原样返回。
func EncryptStored(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	enc, err := encrypt(plaintext)
	if err != nil {
		// 加密失败不应阻断业务,退回明文(与历史行为一致)
		return plaintext
	}
	return enc
}

// DecryptStoredOrDefault 解密,失败时回退原值(兼容历史明文数据)。
func DecryptStoredOrDefault(stored string) string {
	v, err := DecryptStored(stored)
	if err != nil {
		return stored
	}
	return v
}
