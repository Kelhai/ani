package controllers

import (
	"errors"
	"log"
	"net/http"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type conversationResponse struct {
	Id uuid.UUID `json:"id"`
	Members []string `json:"members"`
}

func setupMessageRoutes(e *echo.Echo) {
	g := e.Group("/messages")

	g.POST("/conversation", createConversation)
	g.POST("/m/:conversationId", sendMessageToConversation)
	g.GET("/conversations", getConversations)
	g.GET("/conversation/:conversationId", getMessagesFromConversation)
	g.GET("/conversation/:conversationId/:messageId", getMessagesFromConversationSince)
}

func getMessagesFromConversation(c *echo.Context) error {
	return nil
}

func getMessagesFromConversationSince(c *echo.Context) error {
	return nil
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

	return c.JSON(http.StatusCreated, struct{
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

	var body struct {
		Message string `json:"message"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	senderId := c.Get("userID").(uuid.UUID)

	messageId, err := messageService.SendMessageToConversation(body.Message, senderId, conversationId)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to send message")
	}

	return c.JSON(http.StatusCreated, struct{
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

	response := make([]conversationResponse, len(conversations))
	for _, conversation := range conversations {
		response = append(response, conversationResponse{
			Id: conversation.Id,
		})

		members := []string{}
		for _, member := range conversation.Members {
			members = append(members, usernameMap[member])
		}
		response[len(response)-1].Members = members
	}

	return c.JSON(http.StatusOK, response)
}

