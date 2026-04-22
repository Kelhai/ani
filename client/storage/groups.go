package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

func SaveSenderKey(groupId uuid.UUID, username string, keyId uuid.UUID, key []byte) error {
	dir, err := groupDir(groupId)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "sender_keys"), 0700); err != nil {
		return err
	}
	encrypted, err := encrypt(key, MasterKey, keyId[:])
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "sender_keys", keyId.String())
	return os.WriteFile(path, encrypted, 0600)
}

func LoadSenderKey(groupId uuid.UUID, keyId uuid.UUID) ([]byte, error) {
	dir, err := groupDir(groupId)
	if err != nil {
		return nil, err
	}
	encrypted, err := os.ReadFile(filepath.Join(dir, "sender_keys", keyId.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to read sender key: %w", err)
	}
	return decrypt(encrypted, MasterKey, keyId[:])
}

