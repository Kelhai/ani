package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Kelhai/ani/client/config"
	"github.com/google/uuid"
)

type SavedSession struct {
	Username  string    `json:"username"`
	Token     uuid.UUID `json:"token"`
	UserId    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

func sessionPath() (string, error) {
	return filepath.Join(config.AniHome, ".session"), nil
}

func SaveSession(s SavedSession) error {
	if err := os.MkdirAll(config.AniHome, 0700); err != nil {
		return fmt.Errorf("failed to create ani dir: %w", err)
	}
	path, err := sessionPath()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func LoadSession() (*SavedSession, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	s := new(SavedSession)
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, err
	}
	return s, nil
}

func ClearSession() error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}
