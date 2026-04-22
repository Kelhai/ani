package storage

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"golang.org/x/crypto/chacha20poly1305"
)

func aniDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home dir: %w", err)
	}
	return filepath.Join(home, ".ani"), nil
}

func userDir(username string) (string, error) {
	base, err := aniDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, username), nil
}

func groupDir(groupId uuid.UUID) (string, error) {
	base, err := aniDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "groups", groupId.String()), nil
}

func encrypt(plaintext, key, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plaintext, aad), nil
}

func decrypt(ciphertext, key, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return aead.Open(nil, nonce, ciphertext, aad)
}

func DeriveKey(secret []byte, info string) []byte {
	key, err := hkdf.Key(sha256.New, secret, nil, info, 32)
	if err != nil {
		log.Fatal("hkdf.Key failed")
	}

	return key
}
