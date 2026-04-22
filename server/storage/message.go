package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func createMessageSchema(db *bun.DB) {
	ctx := context.Background()

	if _, err := db.NewCreateTable().Model((*Conversation)(nil)).IfNotExists().Exec(ctx); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.NewCreateTable().Model((*ConversationMember)(nil)).IfNotExists().Exec(ctx); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.NewCreateTable().Model((*common.Message)(nil)).IfNotExists().Exec(ctx); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}
	_, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'conversation_members_unique'
			) THEN
				ALTER TABLE conversation_members 
					ADD CONSTRAINT conversation_members_unique 
					UNIQUE (conversation_id, user_id);
			END IF;
		END$$
	`)
	if err != nil {
		log.Fatalf("failed to add unique constraint: %v", err)
	}
}

type Conversation struct {
	bun.BaseModel `bun:"table:conversations"`

	Id   uuid.UUID `bun:"id,pk,type:uuid"`
	Key  string    `bun:"key,unique,notnull"`
	Xmax int       `bun:"xmax,scanonly"`
}

type ConversationMember struct {
	bun.BaseModel `bun:"table:conversation_members"`

	ConversationId uuid.UUID `bun:"conversation_id,type:uuid,notnull"`
	UserId         uuid.UUID `bun:"user_id,type:uuid,notnull"`
}

func conversationKey(members []uuid.UUID) string {
	strs := make([]string, len(members))
	for i, id := range members {
		strs[i] = id.String()
	}
	slices.Sort(strs)
	joined := strings.Join(strs, "")
	hash := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(hash[:])
}

func (pgs PgStorage) GetConversationByMembers(members []uuid.UUID) (*Conversation, error) {
	key := conversationKey(members)
	conversation := Conversation{}

	err := pgs.db.NewSelect().
		Model(&conversation).
		Where("key = ?", key).
		Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		log.Printf("Failed to get conversation: %s", err.Error())
		return nil, fmt.Errorf("Failed to get conversation: %w", err)
	}

	return &conversation, nil
}

func (pgs PgStorage) GetOrCreateConversation(members []uuid.UUID) (*Conversation, error) {
	if len(members) == 0 {
		return nil, errors.New("must have members to get or create conversation")
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, common.ErrUuidFailed
	}

	newConversation := &Conversation{
		Id:  id,
		Key: conversationKey(members),
	}

	_, err = pgs.db.NewInsert().
		Model(newConversation).
		On("CONFLICT (key) DO UPDATE SET key = EXCLUDED.key").
		Returning("id, key, xmax").
		Exec(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get or create conversation: %w", err)
	}

	// pg native
	isNew := newConversation.Xmax == 0
	if isNew {
		conversationMembers := make([]ConversationMember, len(members))
		for i, member := range members {
			conversationMembers[i] = ConversationMember{
				ConversationId: newConversation.Id,
				UserId:         member,
			}
		}
		_, err = pgs.db.NewInsert().
			Model(&conversationMembers).
			Exec(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to insert conversation members: %w", err)
		}
	}

	return newConversation, nil
}

func (pgs PgStorage) GetConversationsByUserId(userId uuid.UUID) ([]Conversation, error) {
	var conversations []Conversation
	err := pgs.db.NewSelect().
		Model(&conversations).
		Join("JOIN conversation_members cm ON cm.conversation_id = conversation.id").
		Where("cm.user_id = ?", userId).
		OrderExpr("conversation.id DESC").
		Scan(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get conversations: %w", err)
	}
	return conversations, nil
}

func (pgs PgStorage) GetMessagesByConversationId(conversationId uuid.UUID) ([]common.Message, error) {
	var messages []common.Message
	err := pgs.db.NewSelect().
		Model(&messages).
		Where("conversation_id = ?", conversationId).
		OrderExpr("id ASC").
		Scan(context.Background())
	if err != nil {
		log.Printf("Failed to get messages: %s", err.Error())
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	return messages, nil
}

func (pgs PgStorage) GetMessageById(msgId uuid.UUID) (*common.Message, error) {
	var message common.Message
	err := pgs.db.NewSelect().
		Model(&message).
		Where("id = ?", msgId).
		Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("Message does not exist: %w", err)
		}
		log.Printf("Failed to get message: %s", err.Error())
		return nil, fmt.Errorf("Failed to get message: %w", err)
	}

	return &message, nil
}

func (pgs PgStorage) GetMessagesAfter(after uuid.UUID) ([]common.Message, error) {
	var messages []common.Message
	err := pgs.db.NewSelect().
		Model(&messages).
		Where("conversation_id = (SELECT conversation_id FROM messages WHERE id = ?)", after).
		Where("id > ?", after).
		OrderExpr("id ASC").
		Scan(context.Background())
	if err != nil {
		log.Printf("Failed to get messages: %s", err.Error())
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	return messages, nil
}

func (pgs PgStorage) InsertMessage(message common.Message) error {
	_, err := pgs.db.NewInsert().Model(&message).Exec(context.Background())
	if err != nil {
		log.Printf("Failed to insert new message: %s", err.Error())
		return fmt.Errorf("Failed to insert message: %w", err)
	}

	return nil
}

func (pgs PgStorage) GetMembersByConversationIds(ids []uuid.UUID) ([]ConversationMember, error) {
	var members []ConversationMember
	err := pgs.db.NewSelect().
		Model(&members).
		Where("conversation_id IN (?)", bun.List(ids)).
		Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("Failed to get conversation members: %w", err)
	}

	return members, nil
}
