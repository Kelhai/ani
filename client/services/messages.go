package services

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Kelhai/ani/client"
	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
)

type MessageService struct{}

func (_ MessageService) GetConversations() ([]common.ConversationWithUsernames, error) {
	statusCode, conversationsBytes, err := apiService.GET("/messages/conversations", nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNoContent {
		return []common.ConversationWithUsernames{}, nil
	} else if statusCode != http.StatusOK {
		return nil, client.ErrUnknownErr
	}

	conversations := new([]common.ConversationWithUsernames)

	err = json.Unmarshal(conversationsBytes, &conversations)
	if err != nil {
		return nil, client.ErrJsonUnmarshal
	}
	
	return *conversations, nil
}

func (_ MessageService) CreateConversation(usernames []string) (*uuid.UUID, error) {
	usernameBody := map[string][]string{
		"usernames": usernames,
	}

	statusCode, conversation, err := apiService.POST("/messages/conversation", usernameBody)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusCreated {
		return nil, client.ErrUnknownErr
	}

	response := new(struct{
		ConversationId uuid.UUID `json:"conversationId"`
	})

	err = json.Unmarshal(conversation, response)
	if err != nil {
		return nil, client.ErrJsonUnmarshal
	}
	
	return &response.ConversationId, nil
}

func (_ MessageService) GetMessageFromConversation(conversationId uuid.UUID, lastMessage *uuid.UUID) ([]common.ShortMessage, error) {
	path := fmt.Sprintf("/messages/conversation/%s", conversationId)
	if lastMessage != nil {
		path += "/" + lastMessage.String()
	}
	statusCode, messagesJson, err := apiService.GET(path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		if statusCode == http.StatusNoContent {
			return []common.ShortMessage{}, nil
		}
		return nil, client.ErrUnknownErr
	}

	messages := new([]common.ShortMessage)

	err = json.Unmarshal(messagesJson, messages)
	if err != nil {
		return nil, client.ErrJsonUnmarshal
	}

	return *messages, nil
}

func (_ MessageService) SendMessage(conversationId uuid.UUID, text string) (*uuid.UUID, error) {
	body := map[string]string{
		"message": text,
	}
	statusCode, messageJson, err := apiService.POST("/messages/m/" + conversationId.String(), body)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusCreated {
		return nil, client.ErrUnknownErr
	}

	messageResponse := new(struct {
		MessageId uuid.UUID `json:"message_id"`
	})
	
	err = json.Unmarshal(messageJson, messageResponse)
	if err != nil {
		return nil, client.ErrJsonUnmarshal
	}

	return &messageResponse.MessageId, nil
}

