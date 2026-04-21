package controllers

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/Kelhai/ani/client"
)

func CmdLoadConversations() tea.Cmd {
	return func() tea.Msg {
		conversations, err := messageService.GetConversations()
		if err != nil {
			return client.ConversationsLoadedMsg{Err: err}
		}

		return client.ConversationsLoadedMsg{Conversations: conversations}
	}
}

func CmdCreateConversation(usernames []string) tea.Cmd {
	return func() tea.Msg {
		conversationId, err := messageService.CreateConversation(usernames)
		if err != nil {
			return client.ConversationCreatedMsg{Err: err}
		}

		return client.ConversationCreatedMsg{Id: *conversationId}
	}
}

func CmdPollMessages(
	convId uuid.UUID,
	lastMessageId *uuid.UUID,
) tea.Cmd {
	return func() tea.Msg {
		messages, err := messageService.GetMessageFromConversation(convId, lastMessageId)
		if err != nil {
			return client.MessagesLoadedMsg{Err: err}
		}

		var lastMessage *uuid.UUID
		if len(messages) > 0 {
			lastMessage = &messages[len(messages)-1].Id
		}

		return client.MessagesLoadedMsg{
			Messages:      messages,
			LastMessageId: lastMessage,
		}
	}
}

func CmdSendMessage(convId uuid.UUID, text string) tea.Cmd {
	return func() tea.Msg {
		messageId, err := messageService.SendMessage(convId, text)
		if err != nil {
			return client.MessageSentMsg{Err: err}
		}

		return client.MessageSentMsg{ MessageId: *messageId }
	}
}

func CmdPollTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return client.PollTickMsg{}
	})
}

