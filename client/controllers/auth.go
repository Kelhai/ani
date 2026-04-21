package controllers

import (
	"errors"
	"fmt"

	"github.com/Kelhai/ani/client"
	tea "github.com/charmbracelet/bubbletea"
)

func Register(username, password string) tea.Cmd {
	return func() tea.Msg {
		if len(username) < 4 {
			return client.RegisterResultMsg{Err: fmt.Errorf("Username must be at least 4 characters")}
		}
		if len(password) < 4 {
			return client.RegisterResultMsg{Err: fmt.Errorf("Password must be at least 4 characters")}
		}

		err := authService.Register(username, password)
		if err != nil {
			if errors.Is(err, client.ErrUsernameTaken) {
				return client.RegisterResultMsg{Err: fmt.Errorf("Username taken, please try again")}
			}
			return client.RegisterResultMsg{Err: fmt.Errorf("Failed to register, please try again")}
		}

		return client.RegisterResultMsg{}
	}
}

func Login(username, password string) tea.Cmd {
	return func() tea.Msg {
		err := authService.Login(username, password)
		if err != nil {
			return client.LoginResultMsg{Err: err}
		}
		return client.LoginResultMsg{Username: username}
	}
}
