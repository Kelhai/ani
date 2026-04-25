package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

func SaveSenderKey(groupId uuid.UUID, sender string, keyId uuid.UUID, key []byte) error {
	dir := groupDir(groupId)
	if err := os.MkdirAll(filepath.Join(dir, "sender_keys"), 0700); err != nil {
		return err
	}
	encrypted, err := encrypt(key, MasterKey, keyId[:])
	if err != nil {
		return err
	}
	// filename: <sender>_<keyId> so we can look up by sender later
	name := sender + "_" + keyId.String()
	return os.WriteFile(filepath.Join(dir, "sender_keys", name), encrypted, 0600)
}

func LoadSenderKey(groupId uuid.UUID, sender string, keyId uuid.UUID) ([]byte, error) {
	dir := groupDir(groupId)
	name := sender + "_" + keyId.String()
	encrypted, err := os.ReadFile(filepath.Join(dir, "sender_keys", name))
	if err != nil {
		return nil, fmt.Errorf("failed to read sender key: %w", err)
	}
	return decrypt(encrypted, MasterKey, keyId[:])
}
