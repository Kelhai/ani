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

func marshalRatchetState(s RatchetState) ([]byte, error) {
	fields := [][]byte{
		s.SendChainKey,
		s.RecvChainKey,
		s.TheirKEMPubKey,
		s.OurKEMPubKey,
		s.OurKEMPrivKey,
	}
	var out []byte
	for _, f := range fields {
		length := uint16(len(f))
		out = append(out, byte(length>>8), byte(length))
		out = append(out, f...)
	}
	return out, nil
}

func unmarshalRatchetState(data []byte) (*RatchetState, error) {
	fields := make([][]byte, 5)
	offset := 0
	for i := range fields {
		if offset+2 > len(data) {
			return nil, fmt.Errorf("ratchet state too short")
		}
		length := int(data[offset])<<8 | int(data[offset+1])
		offset += 2
		if offset+length > len(data) {
			return nil, fmt.Errorf("ratchet state field overrun")
		}
		fields[i] = make([]byte, length)
		copy(fields[i], data[offset:offset+length])
		offset += length
	}
	return &RatchetState{
		SendChainKey:   fields[0],
		RecvChainKey:   fields[1],
		TheirKEMPubKey: fields[2],
		OurKEMPubKey:   fields[3],
		OurKEMPrivKey:  fields[4],
	}, nil
}
