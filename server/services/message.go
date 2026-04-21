package services

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/Kelhai/ani/common"
	"github.com/Kelhai/ani/server/storage"
	"github.com/google/uuid"
)

type MessageService struct{}

func SetupMessageService() MessageService {
	return MessageService{}
}

func (ms MessageService) GetMessage(msgId uuid.UUID) (*storage.Message, error) {
	return pgStorage.GetMessageById(msgId)
}

func (ms MessageService) GetMessagesSince(since uuid.UUID) ([]common.ShortMessage, error) {
	messages, err := pgStorage.GetMessagesAfter(since)
	if err != nil {
		return nil, err
	}

	userIds := make([]uuid.UUID, len(messages))
	for i, m := range messages {
		userIds[i] = m.SenderId
	}

	userMap, err := pgStorage.GetUsersByIds(userIds)
	if err != nil {
		return nil, err
	}

	out := make([]common.ShortMessage, len(messages))
	for i, m := range messages {
		out[i] = common.ShortMessage{
			Id:      m.Id,
			Sender:  userMap[m.SenderId],
			Content: m.Message,
		}
	}

	return out, nil
}

func (ms MessageService) GetMessages(conversationId uuid.UUID) ([]common.ShortMessage, error) {
	messages, err := pgStorage.GetMessagesByConversationId(conversationId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []common.ShortMessage{}, nil
		}
		return nil, err
	}

	outputMessages := make([]common.ShortMessage, len(messages))
	userIds := []uuid.UUID{}

	for _, message := range messages {
		userIds = append(userIds, message.SenderId)
	}

	userMap, err := pgStorage.GetUsersByIds(userIds)
	if err != nil {
		return nil, err
	}

	for i, message := range messages {
		outputMessages[i] = common.ShortMessage{
			Id:       message.Id,
			Sender:   userMap[message.SenderId],
			Content:  message.Message,
		}
	}

	return outputMessages, nil
}

func (ms MessageService) CheckConversationMember(userId, conversationId uuid.UUID) error {
	conversationMembers, err := pgStorage.GetMembersByConversationIds([]uuid.UUID{conversationId})
	if err != nil {
		log.Printf("Failed to get members: %s", err.Error())
		return fmt.Errorf("Failed to get members: %w", err)
	}

	for _, conversations := range conversationMembers {
		if conversations.UserId == userId {
			return nil
		}
	}

	return common.ErrNotFound
}

func (ms MessageService) GetOrCreateConversation(usernames []string) (*uuid.UUID, error) {
	slices.Sort(usernames)

	idMap, err := pgStorage.GetUserIdsByUsernames(usernames)
	if err != nil {
		log.Printf("Failed to get user ids: %s", err.Error())
		return nil, fmt.Errorf("Failed to get user ids: %w", err)
	}

	values := make([]uuid.UUID, 0, len(idMap))
	for _, username := range usernames {
		values = append(values, idMap[username])
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

	result := make([]common.Conversation, 0, len(conversationMap))
	for conversationId, members := range conversationMap {
		result = append(result, common.Conversation{
			Id:      conversationId,
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
		Id:             id,
		Message:        messageBody,
		ConversationId: conversation,
		SenderId:       sender,
	}

	err = pgStorage.InsertMessage(message)
	if err != nil {
		return nil, errors.Join(common.ErrPgInsertFailed, err)
	}

	return &message.Id, nil
}
