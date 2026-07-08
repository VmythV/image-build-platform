package credential

import (
	"strings"
	"testing"
)

func TestEncryptorRoundTripAndDoesNotStorePlaintext(t *testing.T) {
	encryptor, err := NewEncryptor("test-secret-key-for-credentials-1234")
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}

	plaintext := []byte(`{"username":"robot","password":"registry-secret"}`)
	encrypted, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if strings.Contains(encrypted, "registry-secret") {
		t.Fatalf("encrypted payload contains plaintext")
	}

	decrypted, err := encryptor.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestNewEncryptorRejectsShortSecret(t *testing.T) {
	if _, err := NewEncryptor("short"); err == nil {
		t.Fatalf("expected short secret to fail")
	}
}
