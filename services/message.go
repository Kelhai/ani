package services

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"

	"github.com/Kelhai/ani/common"
	"github.com/Kelhai/ani/storage"
	"github.com/google/uuid"
)

type MessageService struct{}

func SetupMessageService() MessageService {
	return MessageService{}
}

func (ms MessageService) GetOrCreateConversation(usernames []string) (*uuid.UUID, error) {
	slices.Sort(usernames)

	idMap, err := pgStorage.GetUserIdsByUsernames(usernames)
	if err != nil {
		log.Printf("Failed to get user ids: %s", err.Error())
		return nil, fmt.Errorf("Failed to get user ids: %w", err)
	}

	values := make([]uuid.UUID, len(idMap))
	for _, id := range idMap {
		values = append(values, id)
	}

	conversation, err := pgStorage.GetOrCreateConversation(values)
	if err != nil {
		log.Printf("Failed to get or create conversation: %s", err.Error())
		return nil, fmt.Errorf("Failed to get or create conversation: %w", err)
	}

	return &conversation.Id, nil
}

func (ms MessageService) GetConversations(userId uuid.UUID) ([]common.Conversation, error) {
	conversations, err := pgStorage.GetConversationsByUserId(userId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, common.ErrNotFound
		}
		return nil, fmt.Errorf("Failed to get conversations: %w", err)
	}
	if len(conversations) == 0 {
		return []common.Conversation{}, nil
	}

	convIdList := []uuid.UUID{}
	for _, conv := range conversations {
		convIdList = append(convIdList, conv.Id)
	}
	conversationMembers, err := pgStorage.GetMembersByConversationIds(convIdList)

	conversationMap := map[uuid.UUID][]uuid.UUID{}
	for _, conversationMember := range conversationMembers {
		conversationMap[conversationMember.ConversationId] = append(conversationMap[conversationMember.ConversationId], conversationMember.UserId)
	}

	result := make([]common.Conversation, len(conversationMap))
	for conversationId, members := range conversationMap {
		result = append(result, common.Conversation{
			Id: conversationId,
			Members: members,
		})
	}

	return result, nil
}

func (ms MessageService) SendMessageToConversation(messageBody string, sender uuid.UUID, conversation uuid.UUID) (*uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, common.ErrUuidFailed
	}
	message := storage.Message{
		Id: id,
		Message: messageBody,
		ConversationId: conversation,
		SenderId: sender,
	}

	err = pgStorage.InsertMessage(message)
	if err != nil {
		return nil, err
	}

	return &message.Id, nil
}

