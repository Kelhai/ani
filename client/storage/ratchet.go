package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
)

type RatchetState struct {
	SendChainKey  []byte
	RecvChainKey  []byte
	PeerKemPk     []byte
	KemPk         []byte
	KemSk         []byte
	KemRatchetDue bool
	InitHeader    *common.RatchetHeader
}

func ratchetPath(username, peer string) (string, error) {
	dir, err := userDir(username)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ratchet_"+peer), nil
}

func SaveRatchetState(username, peer string, keyId uuid.UUID, state RatchetState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal ratchet state: %w", err)
	}
	encrypted, err := encrypt(raw, MasterKey, keyId[:])
	if err != nil {
		return err
	}
	path, err := ratchetPath(username, peer)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encrypted, 0600)
}

func LoadRatchetState(username, peer string, keyId uuid.UUID) (*RatchetState, error) {
	path, err := ratchetPath(username, peer)
	if err != nil {
		return nil, err
	}
	encrypted, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no session yet
		}
		return nil, fmt.Errorf("failed to read ratchet state: %w", err)
	}
	raw, err := decrypt(encrypted, MasterKey, keyId[:])
	if err != nil {
		return nil, err
	}

	state := new(RatchetState)
	err = json.Unmarshal(raw, state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ratchet state: %w", err)
	}

	return state, nil
}

