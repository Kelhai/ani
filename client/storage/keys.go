package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
	"github.com/google/uuid"
)

var MasterKey []byte

func DeriveMasterKey(password, username string) []byte {
	return argon2.IDKey([]byte(password), []byte(username), 1, 64*1024, 4, 32)
}

func SaveKeyPair(username string, keyId uuid.UUID, pub, priv []byte) error {
	dir, err := userDir(username)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create user dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, keyId.String()+".pub"), pub, 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}
	encrypted, err := encrypt(priv, MasterKey, keyId[:])
	if err != nil {
		return fmt.Errorf("failed to encrypt private key: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, keyId.String()), encrypted, 0600)
}

func LoadPubKey(username string, keyId uuid.UUID) ([]byte, error) {
	dir, err := userDir(username)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, keyId.String()+".pub"))
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}
	return data, nil
}

func LoadPrivKey(username string, keyId uuid.UUID) ([]byte, error) {
	dir, err := userDir(username)
	if err != nil {
		return nil, err
	}
	encrypted, err := os.ReadFile(filepath.Join(dir, keyId.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}
	return decrypt(encrypted, MasterKey, keyId[:])
}

func SaveSymmetricKey(username string, keyId uuid.UUID, key []byte) error {
	dir, err := userDir(username)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	encrypted, err := encrypt(key, MasterKey, keyId[:])
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, keyId.String()), encrypted, 0600)
}

func LoadSymmetricKey(username string, keyId uuid.UUID) ([]byte, error) {
	dir, err := userDir(username)
	if err != nil {
		return nil, err
	}
	encrypted, err := os.ReadFile(filepath.Join(dir, keyId.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to read symmetric key: %w", err)
	}
	return decrypt(encrypted, MasterKey, keyId[:])
}
