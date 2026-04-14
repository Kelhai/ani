package controllers

import tea "github.com/charmbracelet/bubbletea"

func Register(username, password string) tea.Cmd {
	return func() tea.Msg {
		resp, err := apiAuth("/auth/register", username, password)
		if err != nil {
			return registerResultMsg{err: err}
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return registerResultMsg{err: fmt.Errorf("registration failed (status %d)", resp.StatusCode)}
		}
		return registerResultMsg{}
	}
}
