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
	SendChainKey  []byte               `json:"send_chain_key"`
	RecvChainKey  []byte               `json:"recv_chain_key"`
	PeerKemPk     []byte               `json:"peer_kem_pk"`
	KemPk         []byte               `json:"kem_pk"`
	KemSk         []byte               `json:"kem_sk"`
	KemRatchetDue bool                 `json:"kem_ratchet_due"`
	InitHeader    *common.RatchetHeader `json:"init_header,omitempty"`
}

func ratchetPath(username, peer string) string {
	return filepath.Join(userDir(username), "ratchet_"+peer)
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
	path := ratchetPath(username, peer)

	return os.WriteFile(path, encrypted, 0600)
}

func LoadRatchetState(username, peer string, keyId uuid.UUID) (*RatchetState, error) {
	path := ratchetPath(username, peer)
	encrypted, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read ratchet state: %w", err)
	}
	raw, err := decrypt(encrypted, MasterKey, keyId[:])
	if err != nil {
		return nil, err
	}
	state := new(RatchetState)
	if err := json.Unmarshal(raw, state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ratchet state: %w", err)
	}
	return state, nil
}
