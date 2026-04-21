package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Kelhai/ani/client"
	"github.com/Kelhai/ani/client/config"
	"github.com/Kelhai/ani/client/controllers"
	"github.com/Kelhai/ani/common"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Background(lipgloss.Color("236")).
			Padding(0, 1).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	myMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114"))

	otherMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	inputBarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("238")).
			PaddingTop(0)
)

// Fade tick message for animation
type fadeTickMsg time.Time

// Colors for the tilde fade — bright to dark over 10 steps (500ms each)
var fadeColors = []string{
	"212", // 0.0s — bright pink
	"211", // 0.5s
	"175", // 1.0s
	"139", // 1.5s
	"103", // 2.0s
	"67",  // 2.5s
	"246", // 3.0s
	"243", // 3.5s
	"240", // 4.0s
	"237", // 4.5s — nearly invisible
}

type model struct {
	screen client.Screen

	// auth flow
	authChoice string
	username   string
	password   string

	// session
	token  uuid.UUID
	myId   uuid.UUID

	// conversations
	conversations []common.ConversationWithUsernames
	conversationChats map[uuid.UUID][]common.ShortMessage
	cursor        int

	// active chat
	convId        uuid.UUID
	chatLines     []client.ChatLine
	lastMessageId *uuid.UUID
	fadingActive  bool

	// widgets
	textInput textinput.Model
	viewport  viewport.Model
	vpReady   bool

	// window
	width  int
	height int

	// feedback
	status  string
	errStr  string
	loading bool
}

func initialModel() model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 60

	return model{
		screen:        client.ScreenAuthChoice,
		textInput:     ti,
	}
}

func cmdFadeTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return fadeTickMsg(t)
	})
}

// renderChatContent builds viewport content with fading tilde indicators
func (m model) renderChatContent() string {
	var lines []string
	now := time.Now()

	for _, cl := range m.chatLines {
		if cl.FromPoll && !cl.ArrivedAt.IsZero() {
			elapsed := now.Sub(cl.ArrivedAt)
			if elapsed < 5*time.Second {
				step := int(elapsed.Milliseconds() / 500)
				if step >= len(fadeColors) {
					step = len(fadeColors) - 1
				}
				tilde := lipgloss.NewStyle().
					Foreground(lipgloss.Color(fadeColors[step])).
					Render("~ ")
				lines = append(lines, tilde+cl.Text)
				continue
			}
		}
		// no tilde — faded out or own message
		lines = append(lines, cl.Text)
	}
	return strings.Join(lines, "\n")
}

// hasFading checks if any chat lines still have an active fade
func (m model) hasFading() bool {
	now := time.Now()
	for _, cl := range m.chatLines {
		if cl.FromPoll && !cl.ArrivedAt.IsZero() && now.Sub(cl.ArrivedAt) < 5*time.Second {
			return true
		}
	}
	return false
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpH := m.height - 5 // header + input + help + borders
		vpH = max(vpH, 1)
		if !m.vpReady {
			m.viewport = viewport.New(m.width, vpH)
			m.viewport.SetContent("")
			m.vpReady = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpH
		}

	// async states
	case client.RegisterResultMsg:
		m.loading = false
		if msg.Err != nil {
			m.errStr = msg.Err.Error()
			m.screen = client.ScreenPassword
			return m, nil
		}
		m.status = "Registered ✔ — logging in…"
		m.loading = true
		return m, controllers.Login(m.username, m.password)

	case client.LoginResultMsg:
		m.loading = false
		if msg.Err != nil {
			m.errStr = msg.Err.Error()
			m.screen = client.ScreenPassword
			return m, nil
		}
		m.status = "Loading conversations…"
		m.loading = true
		return m, controllers.CmdLoadConversations()

	case client.ConversationsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errStr = msg.Err.Error()
		} else {
			m.conversations = msg.Conversations
			m.errStr = ""

			if m.conversationChats == nil {
				m.conversationChats = make(map[uuid.UUID][]common.ShortMessage)
			}
		}
		if m.cursor >= len(m.conversations) {
			m.cursor = max(0, len(m.conversations)-1)
		}
		m.screen = client.ScreenConversations
		return m, nil

	case client.ConversationCreatedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errStr = msg.Err.Error()
			m.screen = client.ScreenConversations
			return m, nil
		}
		m.enterChat(msg.Id)
		return m, tea.Batch(
			controllers.CmdPollMessages(m.convId, nil),
			textinput.Blink,
		)

	case client.MessagesLoadedMsg:
		if msg.Err == nil {
			if m.conversationChats == nil {
				m.conversationChats = make(map[uuid.UUID][]common.ShortMessage)
			}
			m.conversationChats[m.convId] = append(m.conversationChats[m.convId], msg.Messages...)
			if msg.LastMessageId != nil {
				m.lastMessageId = msg.LastMessageId
			}

			now := time.Now()
			for _, rm := range msg.Messages {
				var line string
				if rm.Sender == m.username {
					line = myMsgStyle.Render(rm.Sender+": ") + rm.Content
				} else {
					line = otherMsgStyle.Render(rm.Sender+": ") + rm.Content
				}
				m.chatLines = append(m.chatLines, client.ChatLine{
					Text:      line,
					ArrivedAt: now,
					FromPoll:  true,
				})
			}

			if len(msg.Messages) > 0 && m.vpReady {
				m.viewport.SetContent(m.renderChatContent())
				m.viewport.GotoBottom()
			}

			// start fade tick loop if we got new messages and it's not already running
			if len(msg.Messages) > 0 && !m.fadingActive {
				m.fadingActive = true
				if m.screen == client.ScreenChat {
					return m, tea.Batch(controllers.CmdPollTick(), cmdFadeTick())
				}
			}
		}
		if m.screen == client.ScreenChat {
			return m, controllers.CmdPollTick()
		}
		return m, nil

	case fadeTickMsg:
		if m.screen != client.ScreenChat {
			m.fadingActive = false
			return m, nil
		}
		if m.hasFading() {
			if m.vpReady {
				m.viewport.SetContent(m.renderChatContent())
			}
			return m, cmdFadeTick()
		}
		// all fades done — one final render without tildes
		m.fadingActive = false
		if m.vpReady {
			m.viewport.SetContent(m.renderChatContent())
		}
		return m, nil

	case client.MessageSentMsg:
		if msg.Err != nil {
			m.errStr = msg.Err.Error()
		} else {
			m.lastMessageId = &msg.MessageId
		}
		return m, nil

	case client.PollTickMsg:
		if m.screen == client.ScreenChat {
			return m, controllers.CmdPollMessages(m.convId, m.lastMessageId)
		}
		return m, nil
	}

	// per-screen handlers
	switch m.screen {
	case client.ScreenAuthChoice:
		return m.updateAuthChoice(msg)
	case client.ScreenUsername:
		return m.updateUsername(msg)
	case client.ScreenPassword:
		return m.updatePassword(msg)
	case client.ScreenConversations:
		return m.updateConversations(msg)
	case client.ScreenNewChat:
		return m.updateNewChat(msg)
	case client.ScreenChat:
		return m.updateChat(msg)
	}

	return m, nil
}

func (m *model) enterChat(convId uuid.UUID) {
	m.convId = convId
	m.chatLines = nil
	m.lastMessageId = nil
	if chat, ok := m.conversationChats[convId]; ok {
		if len(chat) > 0 {
			m.lastMessageId = &chat[len(chat)-1].Id
		}
	}
	m.errStr = ""
	m.fadingActive = false
	m.screen = client.ScreenChat
	m.textInput.SetValue("")
	m.textInput.Placeholder = "Type a message…"
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Focus()
	if m.vpReady {
		m.viewport.SetContent("")
		m.viewport.GotoBottom()
	}
}

// per-screen update handlers
func (m model) updateAuthChoice(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "l", "r":
			m.authChoice = key.String()
			m.errStr = ""
			m.screen = client.ScreenUsername
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Username"
			m.textInput.EchoMode = textinput.EchoNormal
			m.textInput.Focus()
			return m, textinput.Blink
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) updateUsername(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := strings.TrimSpace(m.textInput.Value())
			if val == "" {
				return m, nil
			}
			m.username = val
			m.errStr = ""
			m.screen = client.ScreenPassword
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Password"
			m.textInput.EchoMode = textinput.EchoPassword
			m.textInput.Focus()
			return m, textinput.Blink
		case "esc":
			m.screen = client.ScreenAuthChoice
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) updatePassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := strings.TrimSpace(m.textInput.Value())
			if val == "" {
				return m, nil
			}
			m.password = val
			m.errStr = ""
			m.loading = true
			if m.authChoice == "r" {
				m.status = "Registering…"
				return m, controllers.Register(m.username, m.password)
			}
			m.status = "Logging in…"
			return m, controllers.Login(m.username, m.password)
		case "esc":
			m.screen = client.ScreenUsername
			m.textInput.SetValue(m.username)
			m.textInput.Placeholder = "Username"
			m.textInput.EchoMode = textinput.EchoNormal
			m.textInput.Focus()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) updateConversations(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.conversations)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.conversations) > 0 {
				c := m.conversations[m.cursor]
				m.enterChat(c.Id)
				return m, tea.Batch(
					controllers.CmdPollMessages(m.convId, m.lastMessageId),
					textinput.Blink,
				)
			}
		case "n":
			m.screen = client.ScreenNewChat
			m.errStr = ""
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Usernames (comma-separated)"
			m.textInput.EchoMode = textinput.EchoNormal
			m.textInput.Focus()
			return m, textinput.Blink
		case "r":
			m.loading = true
			m.status = "Refreshing…"
			return m, controllers.CmdLoadConversations()
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) updateNewChat(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := strings.TrimSpace(m.textInput.Value())
			if val == "" {
				return m, nil
			}
			usernames := []string{m.username}
			for u := range strings.SplitSeq(val, ",") {
				u = strings.TrimSpace(u)
				if u != "" && u != m.username {
					usernames = append(usernames, u)
				}
			}
			m.loading = true
			m.errStr = ""
			m.status = "Creating conversation…"
			return m, controllers.CmdCreateConversation(usernames)
		case "esc":
			m.screen = client.ScreenConversations
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) updateChat(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.screen = client.ScreenConversations
			m.fadingActive = false
			m.loading = true
			m.status = "Loading conversations…"
			return m, controllers.CmdLoadConversations()
		case "enter":
			text := strings.TrimSpace(m.textInput.Value())
			if text == "" {
				return m, nil
			}
			m.textInput.SetValue("")

			// optimistic — own messages never get a tilde
			line := myMsgStyle.Render(m.username+": ") + text
			m.chatLines = append(m.chatLines, client.ChatLine{
				Text:     line,
				FromPoll: false,
			})
			if m.vpReady {
				m.viewport.SetContent(m.renderChatContent())
				m.viewport.GotoBottom()
			}
			return m, controllers.CmdSendMessage(m.convId, text)
		}
	}

	var cmds []tea.Cmd

	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	cmds = append(cmds, tiCmd)

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.loading {
		return m.frame(statusStyle.Render("⏳ "+m.status) + "\n")
	}
	switch m.screen {
	case client.ScreenAuthChoice:
		return m.viewAuthChoice()
	case client.ScreenUsername:
		return m.viewUsername()
	case client.ScreenPassword:
		return m.viewPassword()
	case client.ScreenConversations:
		return m.viewConversations()
	case client.ScreenNewChat:
		return m.viewNewChat()
	case client.ScreenChat:
		return m.viewChat()
	}
	return ""
}

func (m model) frame(body string) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("💬 Ani Chat") + "\n\n")
	b.WriteString(body)
	if m.errStr != "" {
		b.WriteString("\n" + errorStyle.Render("Error: "+m.errStr) + "\n")
	}
	return b.String()
}

func (m model) viewAuthChoice() string {
	body := fmt.Sprintf(
		"%s\n\n  %s  Login\n  %s  Register\n\n%s\n",
		"Welcome! Choose an option:",
		selectedStyle.Render("[L]"),
		selectedStyle.Render("[R]"),
		helpStyle.Render("Press l / r • q to quit"),
	)
	return m.frame(body)
}

func (m model) viewUsername() string {
	action := "Login"
	if m.authChoice == "r" {
		action = "Register"
	}
	body := fmt.Sprintf(
		"%s — enter your username:\n\n  %s\n\n%s\n",
		action,
		m.textInput.View(),
		helpStyle.Render("enter to continue • esc to go back"),
	)
	return m.frame(body)
}

func (m model) viewPassword() string {
	action := "Login"
	if m.authChoice == "r" {
		action = "Register"
	}
	body := fmt.Sprintf(
		"%s as %s — enter your password:\n\n  %s\n\n%s\n",
		action,
		selectedStyle.Render(m.username),
		m.textInput.View(),
		helpStyle.Render("enter to submit • esc to go back"),
	)
	return m.frame(body)
}

func (m model) viewConversations() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Logged in as %s\n\n", selectedStyle.Render(m.username))

	if len(m.conversations) == 0 {
		b.WriteString(statusStyle.Render("  No conversations yet.") + "\n")
	} else {
		b.WriteString("Conversations:\n\n")
		for i, c := range m.conversations {
			cursor := "  "
			style := normalItemStyle
			if i == m.cursor {
				cursor = "▸ "
				style = selectedStyle
			}
			members := ""
			for _, member := range c.Members {
				members = members + member + ", "
			}
			line := fmt.Sprintf("%s%s", cursor, style.Render(members))
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", min(m.width, 50))) + "\n")
	b.WriteString(helpStyle.Render("↑/↓ navigate • enter open • n new chat • r refresh • q quit") + "\n")

	return m.frame(b.String())
}

func (m model) viewNewChat() string {
	body := fmt.Sprintf(
		"New conversation — who do you want to chat with?\n\n  %s\n\n%s\n",
		m.textInput.View(),
		helpStyle.Render("enter comma-separated usernames • esc to cancel"),
	)
	return m.frame(body)
}

func (m model) viewChat() string {
	var b strings.Builder

	header := titleStyle.Render(fmt.Sprintf("💬 Conversation %s", m.convId.String()[:8]))
	b.WriteString(header + "\n")

	if m.vpReady {
		b.WriteString(m.viewport.View())
		b.WriteString("\n")
	} else {
		b.WriteString(statusStyle.Render("Initializing…") + "\n")
	}

	b.WriteString(inputBarStyle.Render(m.textInput.View()) + "\n")

	if m.errStr != "" {
		b.WriteString(errorStyle.Render("Error: "+m.errStr) + "\n")
	}

	b.WriteString(helpStyle.Render("enter send • esc back to conversations"))

	return b.String()
}

func main() {
	err := config.SetupConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %s", err.Error())
		os.Exit(1)
	}

	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

