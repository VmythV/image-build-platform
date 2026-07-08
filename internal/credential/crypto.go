package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptionAlgorithm = "AES-256-GCM"

var ErrInvalidSecret = errors.New("credential secret key must be at least 32 characters")

type Encryptor struct {
	key []byte
}

type encryptedPayload struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func NewEncryptor(secret string) (Encryptor, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return Encryptor{}, ErrInvalidSecret
	}

	sum := sha256.Sum256([]byte("image-build-platform:credentials:v1:" + secret))
	return Encryptor{key: sum[:]}, nil
}

func (e Encryptor) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte("ibp-credential-v1"))
	payload := encryptedPayload{
		Version:    EncryptionVersion,
		Algorithm:  encryptionAlgorithm,
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode encrypted payload: %w", err)
	}
	return string(data), nil
}

func (e Encryptor) Decrypt(value string) ([]byte, error) {
	var payload encryptedPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return nil, fmt.Errorf("decode encrypted payload: %w", err)
	}
	if payload.Version != EncryptionVersion || payload.Algorithm != encryptionAlgorithm {
		return nil, fmt.Errorf("unsupported credential encryption metadata")
	}

	nonce, err := base64.RawStdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte("ibp-credential-v1"))
	if err != nil {
		return nil, fmt.Errorf("decrypt credential: %w", err)
	}
	return plaintext, nil
}

func Fingerprint(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))[:16]
}
