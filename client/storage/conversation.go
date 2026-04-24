package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type StoredMessage struct {
	Id         uuid.UUID `json:"id"`
	Sender     string    `json:"sender"`
	Ciphertext []byte    `json:"ciphertext"`
	Header     []byte    `json:"header"`
	Signature  []byte    `json:"signature"`
}

type ConversationStore struct {
	ConversationId uuid.UUID       `json:"conversation_id"`
	Messages       []StoredMessage `json:"messages"`
	LastMessageId  *uuid.UUID      `json:"last_message_id"`
}

func convPath(username string, conversationId uuid.UUID) (string, error) {
	dir, err := userDir(username)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "conv_"+conversationId.String()), nil
}

func convKey(conversationId uuid.UUID) []byte {
	return DeriveKey(MasterKey, "conv:"+conversationId.String())
}

func LoadConversationStore(username string, conversationId uuid.UUID) (*ConversationStore, error) {
	path, err := convPath(username, conversationId)
	if err != nil {
		return nil, err
	}

	encrypted, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ConversationStore{
				ConversationId: conversationId,
				Messages:       []StoredMessage{},
				LastMessageId:  nil,
			}, nil
		}
		return nil, fmt.Errorf("failed to read conversation store: %w", err)
	}

	raw, err := decrypt(encrypted, convKey(conversationId), conversationId[:])
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt conversation store: %w", err)
	}

	store := new(ConversationStore)
	if err := json.Unmarshal(raw, store); err != nil {
		return nil, fmt.Errorf("failed to unmarshal conversation store: %w", err)
	}

	return store, nil
}

func SaveConversationStore(username string, store *ConversationStore) error {
	path, err := convPath(username, store.ConversationId)
	if err != nil {
		return err
	}

	raw, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation store: %w", err)
	}

	encrypted, err := encrypt(raw, convKey(store.ConversationId), store.ConversationId[:])
	if err != nil {
		return fmt.Errorf("failed to encrypt conversation store: %w", err)
	}

	return os.WriteFile(path, encrypted, 0600)
}

func AppendMessages(username string, conversationId uuid.UUID, messages []StoredMessage) error {
	store, err := LoadConversationStore(username, conversationId)
	if err != nil {
		return err
	}

	store.Messages = append(store.Messages, messages...)

	if len(messages) > 0 {
		last := messages[len(messages)-1].Id
		store.LastMessageId = &last
	}

	return SaveConversationStore(username, store)
}
