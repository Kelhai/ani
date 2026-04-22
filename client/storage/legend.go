package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type KeyTag string

const (
	KeyTagIdentity KeyTag = "identity"
	KeyTagKem KeyTag = "kem"
	KeyTagSenderKey KeyTag = "sender_key"
	KeyTagRatchet   KeyTag = "ratchet"
)

type LegendEntry struct {
	Tag     KeyTag    `json:"tag"`
	Type    string    `json:"type"`
	Created time.Time `json:"created"`
}

type Legend struct {
	Version string                 `json:"version"`
	Keys    map[string]LegendEntry `json:"keys"`
}

const legendVersion = "v0.1.0"

func legendPath(username string) (string, error) {
	dir, err := userDir(username)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".legend"), nil
}

func groupLegendPath(groupId uuid.UUID) (string, error) {
	dir, err := groupDir(groupId)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".legend"), nil
}

func loadLegend(username string) (*Legend, error) {
	path, err := legendPath(username)
	if err != nil {
		return nil, err
	}

	encrypted, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Legend{
				Version: legendVersion,
				Keys:    make(map[string]LegendEntry),
			}, nil
		}

		return nil, fmt.Errorf("failed to read legend: %w", err)
	}

	raw, err := decrypt(encrypted, MasterKey, []byte(username))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt legend: %w", err)
	}

	var legend Legend
	if err := json.Unmarshal(raw, &legend); err != nil {
		return nil, fmt.Errorf("failed to unmarshal legend: %w", err)
	}

	return &legend, nil
}

func saveLegend(username string, legend *Legend) error {
	path, err := legendPath(username)
	if err != nil {
		return err
	}

	raw, err := json.Marshal(legend)
	if err != nil {
		return fmt.Errorf("failed to marshal legend: %w", err)
	}

	encrypted, err := encrypt(raw, MasterKey, []byte(username))
	if err != nil {
		return fmt.Errorf("failed to encrypt legend: %w", err)
	}

	return os.WriteFile(path, encrypted, 0600)
}

func AddLegendEntry(username string, keyId uuid.UUID, entry LegendEntry) error {
	legend, err := loadLegend(username)
	if err != nil {
		return err
	}
	legend.Keys[keyId.String()] = entry
	return saveLegend(username, legend)
}

func GetLegendEntry(username string, keyId uuid.UUID) (*LegendEntry, error) {
	legend, err := loadLegend(username)
	if err != nil {
		return nil, err
	}
	entry, ok := legend.Keys[keyId.String()]
	if !ok {
		return nil, fmt.Errorf("key %s not found in legend", keyId)
	}
	return &entry, nil
}

func RemoveLegendEntry(username string, keyId uuid.UUID) error {
	legend, err := loadLegend(username)
	if err != nil {
		return err
	}
	delete(legend.Keys, keyId.String())
	return saveLegend(username, legend)
}

func FindKeyByTag(username string, tag KeyTag) (uuid.UUID, *LegendEntry, error) {
	legend, err := loadLegend(username)
	if err != nil {
		return uuid.Nil, nil, err
	}
	for idStr, entry := range legend.Keys {
		if entry.Tag == tag {
			id, err := uuid.Parse(idStr)
			if err != nil {
				return uuid.Nil, nil, fmt.Errorf("invalid UUID in legend: %w", err)
			}
			e := entry
			return id, &e, nil
		}
	}
	return uuid.Nil, nil, fmt.Errorf("no key with tag %s found", tag)
}

func FindKeyByPeer(username string, tag KeyTag, peer string) (uuid.UUID, *LegendEntry, error) {
	legend, err := loadLegend(username)
	if err != nil {
		return uuid.Nil, nil, err
	}
	for idStr, entry := range legend.Keys {
		if entry.Tag == tag && entry.Type == peer {
			id, err := uuid.Parse(idStr)
			if err != nil {
				return uuid.Nil, nil, fmt.Errorf("invalid UUID in legend: %w", err)
			}
			e := entry
			return id, &e, nil
		}
	}
	return uuid.Nil, nil, fmt.Errorf("no key with tag %s for peer %s found", tag, peer)
}
