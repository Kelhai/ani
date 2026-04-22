package controllers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func setupMessageRoutes(e *echo.Echo) {
	g := e.Group("/messages", SessionMiddleware)

	g.POST("/conversation", createConversation)
	g.POST("/m/:conversationId", sendMessageToConversation)
	g.GET("/conversations", getConversations)
	g.GET("/conversation/:conversationId", getMessagesFromConversation)
	g.GET("/conversation/:conversationId/:messageId", getMessagesFromConversationSince)
}

func getMessagesFromConversation(c *echo.Context) error {
	userId := c.Get("userId").(uuid.UUID)
	conversationId, err := uuid.Parse(c.Param("conversationId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid conversation id")
	}

	err = messageService.CheckConversationMember(userId, conversationId)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return c.NoContent(http.StatusUnauthorized)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check conversation")
	}

	messages, err := messageService.GetMessages(conversationId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNoContent, []common.ShortMessage{})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get messages")
	}

	return c.JSON(http.StatusOK, messages)
}

func getMessagesFromConversationSince(c *echo.Context) error {
	userId := c.Get("userId").(uuid.UUID)
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid message id")
	}

	message, err := messageService.GetMessage(messageId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "no such message")
	}

	err = messageService.CheckConversationMember(userId, message.ConversationId)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return c.NoContent(http.StatusUnauthorized)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check conversation")
	}

	messages, err := messageService.GetMessagesSince(messageId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusOK, []common.Message{})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get messages")
	}

	return c.JSON(http.StatusOK, messages)
}

func createConversation(c *echo.Context) error {
	var body struct {
		Usernames []string `json:"usernames"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user list")
	}

	conversationId, err := messageService.GetOrCreateConversation(body.Usernames)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get or create conversation")
	}

	return c.JSON(http.StatusCreated, struct {
		ConversationId uuid.UUID `json:"conversationId"`
	}{
		ConversationId: *conversationId,
	})
}

func sendMessageToConversation(c *echo.Context) error {
	conversationId, err := uuid.Parse(c.Param("conversationId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid conversation id")
	}

	var body common.SendMessageRequest
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	senderId := c.Get("userId").(uuid.UUID)

	messageId, err := messageService.SendMessageToConversation(body.Ciphertext, body.Header, body.Signature, senderId, conversationId)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to send message")
	}

	return c.JSON(http.StatusCreated, struct {
		MessageId uuid.UUID `json:"message_id"`
	}{
		MessageId: *messageId,
	})
}

func getConversations(c *echo.Context) error {
	userId, ok := c.Get("userId").(uuid.UUID)
	if !ok {
		log.Println("Failed to parse userId from header")
		return c.NoContent(http.StatusInternalServerError)
	}

	conversations, err := messageService.GetConversations(userId)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return c.NoContent(http.StatusNoContent)
		}
		log.Printf("Failed to get conversations: %s", err.Error())
		return c.NoContent(http.StatusInternalServerError)
	}

	partnerIds := []uuid.UUID{}
	uuidMap := map[uuid.UUID]struct{}{}
	for _, conversation := range conversations {
		for _, member := range conversation.Members {
			_, ok := uuidMap[member]
			if !ok {
				partnerIds = append(partnerIds, member)
				uuidMap[member] = struct{}{}
			}
		}
	}

	usernameMap, err := authService.GetUsernamesByIds(partnerIds)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	response := make([]common.ConversationWithUsernames, 0, len(conversations))
	for _, conversation := range conversations {
		response = append(response, common.ConversationWithUsernames{
			Id: conversation.Id,
		})

		members := make([]string, len(conversation.Members))
		for i, member := range conversation.Members {
			members[i] = usernameMap[member]
		}
		response[len(response)-1].Members = members
	}

	return c.JSON(http.StatusOK, response)
}
