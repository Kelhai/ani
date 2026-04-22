package controllers

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/Kelhai/ani/client"
	"github.com/Kelhai/ani/common"
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
	username string,
	lastMessageId *uuid.UUID,
) tea.Cmd {
	return func() tea.Msg {
		messages, err := messageService.GetMessageFromConversation(convId, username, lastMessageId)
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

func CmdSendMessage(conversation common.ConversationWithUsernames, myUsername, text string) tea.Cmd {
	return func() tea.Msg {
		recipients := []string{}
		for _, u := range conversation.Members {
			if u != myUsername {
				recipients = append(recipients, u)
			}
		}

		messageId, err := messageService.SendMessage(conversation.Id, myUsername, recipients, text)
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

