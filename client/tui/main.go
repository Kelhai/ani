package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// ═══════════════════════════════════════════════════════════════════════════
// Screens
// ═══════════════════════════════════════════════════════════════════════════

type screen int

const (
	screenAuthChoice screen = iota
	screenUsername
	screenPassword
	screenConversations
	screenNewChat
	screenChat
)

// ═══════════════════════════════════════════════════════════════════════════
// Styles
// ═══════════════════════════════════════════════════════════════════════════

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

// ═══════════════════════════════════════════════════════════════════════════
// API helpers (same as original, but decoupled from global state)
// ═══════════════════════════════════════════════════════════════════════════

var (
	baseURL    = "http://localhost:52971"
	httpClient = &http.Client{Timeout: 10 * time.Second}
)

func init() {
	godotenv.Load()
	host := os.Getenv("SERVER_HOST")
	port := os.Getenv("SERVER_PORT")

	if host != "" && port != "" {
		baseURL = "http://" + host + ":" + port
	} else if host != "" {
		baseURL = "http://" + host + ":52971"
	} else if port != "" {
		baseURL = "http://localhost:" + port
	}
}

func apiAuth(path, username, password string) (*http.Response, error) {
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	req, _ := http.NewRequest("POST", baseURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return httpClient.Do(req)
}

func apiAuthed(method, path string, tok uuid.UUID, payload any) (*http.Response, error) {
	var buf []byte
	if payload != nil {
		buf, _ = json.Marshal(payload)
	}
	req, _ := http.NewRequest(method, baseURL+path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok.String())
	return httpClient.Do(req)
}

// ═══════════════════════════════════════════════════════════════════════════
// Data types
// ═══════════════════════════════════════════════════════════════════════════

type conversation struct {
	Id      uuid.UUID `json:"id"`
	Members []string  `json:"members"`
}

type rawMessage struct {
	Id       uuid.UUID `json:"id"`
	SenderId uuid.UUID `json:"senderId"`
	Message  string    `json:"message"`
}

type resolvedMessage struct {
	id       uuid.UUID
	sender   string
	senderId uuid.UUID
	text     string
}

// ═══════════════════════════════════════════════════════════════════════════
// Tea messages (results from async commands)
// ═══════════════════════════════════════════════════════════════════════════

type registerResultMsg struct{ err error }

type loginResultMsg struct {
	token    uuid.UUID
	id       uuid.UUID
	username string
	err      error
}

type conversationsLoadedMsg struct {
	convos []conversation
	err    error
}

type conversationCreatedMsg struct {
	id  uuid.UUID
	err error
}

type messagesLoadedMsg struct {
	messages      []resolvedMessage
	newUsernames  map[uuid.UUID]string
	lastMessageId *uuid.UUID
	err           error
}

type messageSentMsg struct{ err error }

type pollTickMsg struct{}

// ═══════════════════════════════════════════════════════════════════════════
// Async commands
// ═══════════════════════════════════════════════════════════════════════════

func cmdRegister(username, password string) tea.Cmd {
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

func cmdLogin(username, password string) tea.Cmd {
	return func() tea.Msg {
		resp, err := apiAuth("/auth/login", username, password)
		if err != nil {
			return loginResultMsg{err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return loginResultMsg{err: fmt.Errorf("login failed (status %d)", resp.StatusCode)}
		}
		var lr struct {
			Token    uuid.UUID `json:"token"`
			Id       uuid.UUID `json:"id"`
			Username string    `json:"username"`
		}
		json.NewDecoder(resp.Body).Decode(&lr)
		return loginResultMsg{token: lr.Token, id: lr.Id, username: lr.Username}
	}
}

func cmdLoadConversations(tok uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		resp, err := apiAuthed("GET", "/messages/conversations", tok, nil)
		if err != nil {
			return conversationsLoadedMsg{err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return conversationsLoadedMsg{err: fmt.Errorf("failed to load conversations (status %d)", resp.StatusCode)}
		}
		var convos []conversation
		json.NewDecoder(resp.Body).Decode(&convos)
		return conversationsLoadedMsg{convos: convos}
	}
}

func cmdCreateConversation(tok uuid.UUID, usernames []string) tea.Cmd {
	return func() tea.Msg {
		resp, err := apiAuthed("POST", "/messages/conversation", tok, map[string]any{
			"usernames": usernames,
		})
		if err != nil {
			return conversationCreatedMsg{err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return conversationCreatedMsg{err: fmt.Errorf("failed to create conversation (status %d)", resp.StatusCode)}
		}
		var cr struct {
			ConversationId uuid.UUID `json:"conversationId"`
		}
		json.NewDecoder(resp.Body).Decode(&cr)
		return conversationCreatedMsg{id: cr.ConversationId}
	}
}

func cmdPollMessages(
	tok uuid.UUID,
	convId uuid.UUID,
	lastId *uuid.UUID,
	myId uuid.UUID,
	cache map[uuid.UUID]string,
) tea.Cmd {
	// snapshot the cache so the goroutine is safe
	snap := make(map[uuid.UUID]string, len(cache))
	for k, v := range cache {
		snap[k] = v
	}
	isInitialLoad := lastId == nil

	return func() tea.Msg {
		path := fmt.Sprintf("/messages/conversation/%s", convId)
		if lastId != nil {
			path = fmt.Sprintf("/messages/conversation/%s/%s", convId, lastId)
		}

		resp, err := apiAuthed("GET", path, tok, nil)
		if err != nil {
			return messagesLoadedMsg{err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return messagesLoadedMsg{err: fmt.Errorf("poll failed (status %d)", resp.StatusCode)}
		}

		var raw []rawMessage
		json.NewDecoder(resp.Body).Decode(&raw)

		newUsernames := make(map[uuid.UUID]string)
		var resolved []resolvedMessage
		var last *uuid.UUID

		for _, m := range raw {
			id := m.Id
			last = &id

			// On subsequent polls, skip own messages (we add them optimistically)
			if !isInitialLoad && m.SenderId == myId {
				continue
			}

			sender, ok := snap[m.SenderId]
			if !ok {
				r2, e2 := apiAuthed("GET", fmt.Sprintf("/auth/user/%s", m.SenderId), tok, nil)
				if e2 == nil && r2.StatusCode == http.StatusOK {
					var u struct {
						Username string `json:"username"`
					}
					json.NewDecoder(r2.Body).Decode(&u)
					r2.Body.Close()
					sender = u.Username
					snap[m.SenderId] = sender
					newUsernames[m.SenderId] = sender
				} else {
					sender = m.SenderId.String()[:8]
				}
			}

			resolved = append(resolved, resolvedMessage{
				id:       m.Id,
				sender:   sender,
				senderId: m.SenderId,
				text:     m.Message,
			})
		}

		return messagesLoadedMsg{
			messages:      resolved,
			newUsernames:  newUsernames,
			lastMessageId: last,
		}
	}
}

func cmdSendMessage(tok uuid.UUID, convId uuid.UUID, text string) tea.Cmd {
	return func() tea.Msg {
		resp, err := apiAuthed(
			"POST",
			fmt.Sprintf("/messages/m/%s", convId),
			tok,
			map[string]string{"message": text},
		)
		if err != nil {
			return messageSentMsg{err: err}
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return messageSentMsg{err: fmt.Errorf("send failed (status %d)", resp.StatusCode)}
		}
		return messageSentMsg{}
	}
}

func cmdPollTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

// Add this struct to replace the raw string chat lines
type chatLine struct {
	text      string
	arrivedAt time.Time
	fromPoll  bool // true = came from polling, false = optimistic own send
}

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

// ═══════════════════════════════════════════════════════════════════════════
// Model
// ═══════════════════════════════════════════════════════════════════════════

type model struct {
	screen screen

	// auth flow
	authChoice string
	username   string
	password   string

	// session
	token  uuid.UUID
	myId   uuid.UUID
	myName string

	// conversations
	conversations []conversation
	cursor        int

	// active chat
	convId        uuid.UUID
	chatLines     []chatLine // ← changed from []string
	lastMessageId *uuid.UUID
	usernameCache map[uuid.UUID]string
	fadingActive  bool // ← new: is the fade tick loop running?

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
		screen:        screenAuthChoice,
		textInput:     ti,
		usernameCache: make(map[uuid.UUID]string),
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
		if cl.fromPoll && !cl.arrivedAt.IsZero() {
			elapsed := now.Sub(cl.arrivedAt)
			if elapsed < 5*time.Second {
				step := int(elapsed.Milliseconds() / 500)
				if step >= len(fadeColors) {
					step = len(fadeColors) - 1
				}
				tilde := lipgloss.NewStyle().
					Foreground(lipgloss.Color(fadeColors[step])).
					Render("~ ")
				lines = append(lines, tilde+cl.text)
				continue
			}
		}
		// no tilde — faded out or own message
		lines = append(lines, cl.text)
	}
	return strings.Join(lines, "\n")
}

// hasFading checks if any chat lines still have an active fade
func (m model) hasFading() bool {
	now := time.Now()
	for _, cl := range m.chatLines {
		if cl.fromPoll && !cl.arrivedAt.IsZero() && now.Sub(cl.arrivedAt) < 5*time.Second {
			return true
		}
	}
	return false
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// ═══════════════════════════════════════════════════════════════════════════
// Update — top level
// ═══════════════════════════════════════════════════════════════════════════

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── global keys ──────────────────────────────────────────────────
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	// ── window resize ────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpH := m.height - 5 // header + input + help + borders
		if vpH < 1 {
			vpH = 1
		}
		if !m.vpReady {
			m.viewport = viewport.New(m.width, vpH)
			m.viewport.SetContent("")
			m.vpReady = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpH
		}

	// ── async results (handled regardless of current screen) ─────────

	case registerResultMsg:
		m.loading = false
		if msg.err != nil {
			m.errStr = msg.err.Error()
			m.screen = screenPassword
			return m, nil
		}
		m.status = "Registered ✔ — logging in…"
		m.loading = true
		return m, cmdLogin(m.username, m.password)

	case loginResultMsg:
		m.loading = false
		if msg.err != nil {
			m.errStr = msg.err.Error()
			m.screen = screenPassword
			return m, nil
		}
		m.token = msg.token
		m.myId = msg.id
		m.myName = msg.username
		m.usernameCache[msg.id] = msg.username
		m.status = "Loading conversations…"
		m.loading = true
		return m, cmdLoadConversations(m.token)

	case conversationsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errStr = msg.err.Error()
		} else {
			m.conversations = msg.convos
			m.errStr = ""
		}
		if m.cursor >= len(m.conversations) {
			m.cursor = max(0, len(m.conversations)-1)
		}
		m.screen = screenConversations
		return m, nil

	case conversationCreatedMsg:
		m.loading = false
		if msg.err != nil {
			m.errStr = msg.err.Error()
			m.screen = screenConversations
			return m, nil
		}
		m.enterChat(msg.id)
		return m, tea.Batch(
			cmdPollMessages(m.token, m.convId, nil, m.myId, m.usernameCache),
			textinput.Blink,
		)

	case messagesLoadedMsg:
		if msg.err == nil {
			for k, v := range msg.newUsernames {
				m.usernameCache[k] = v
			}
			if msg.lastMessageId != nil {
				m.lastMessageId = msg.lastMessageId
			}

			now := time.Now()
			for _, rm := range msg.messages {
				var line string
				if rm.senderId == m.myId {
					line = myMsgStyle.Render(rm.sender+": ") + rm.text
				} else {
					line = otherMsgStyle.Render(rm.sender+": ") + rm.text
				}
				m.chatLines = append(m.chatLines, chatLine{
					text:      line,
					arrivedAt: now,
					fromPoll:  true,
				})
			}

			if len(msg.messages) > 0 && m.vpReady {
				m.viewport.SetContent(m.renderChatContent())
				m.viewport.GotoBottom()
			}

			// start fade tick loop if we got new messages and it's not already running
			if len(msg.messages) > 0 && !m.fadingActive {
				m.fadingActive = true
				if m.screen == screenChat {
					return m, tea.Batch(cmdPollTick(), cmdFadeTick())
				}
			}
		}
		if m.screen == screenChat {
			return m, cmdPollTick()
		}
		return m, nil

	// ── fade animation tick ──────────────────────────────────────────
	case fadeTickMsg:
		if m.screen != screenChat {
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

	case messageSentMsg:
		if msg.err != nil {
			m.errStr = msg.err.Error()
		}
		return m, nil

	case pollTickMsg:
		if m.screen == screenChat {
			return m, cmdPollMessages(m.token, m.convId, m.lastMessageId, m.myId, m.usernameCache)
		}
		return m, nil
	}

	// ── delegate to per-screen handlers ──────────────────────────────
	switch m.screen {
	case screenAuthChoice:
		return m.updateAuthChoice(msg)
	case screenUsername:
		return m.updateUsername(msg)
	case screenPassword:
		return m.updatePassword(msg)
	case screenConversations:
		return m.updateConversations(msg)
	case screenNewChat:
		return m.updateNewChat(msg)
	case screenChat:
		return m.updateChat(msg)
	}

	return m, nil
}

// ── helper to transition into chat ──────────────────────────────────────────

func (m *model) enterChat(convId uuid.UUID) {
	m.convId = convId
	m.chatLines = nil // already correct — now []chatLine
	m.lastMessageId = nil
	m.errStr = ""
	m.fadingActive = false
	m.screen = screenChat
	m.textInput.SetValue("")
	m.textInput.Placeholder = "Type a message…"
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Focus()
	if m.vpReady {
		m.viewport.SetContent("")
		m.viewport.GotoBottom()
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Per-screen update handlers
// ═══════════════════════════════════════════════════════════════════════════

func (m model) updateAuthChoice(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "l", "r":
			m.authChoice = key.String()
			m.errStr = ""
			m.screen = screenUsername
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
			m.screen = screenPassword
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Password"
			m.textInput.EchoMode = textinput.EchoPassword
			m.textInput.Focus()
			return m, textinput.Blink
		case "esc":
			m.screen = screenAuthChoice
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
				return m, cmdRegister(m.username, m.password)
			}
			m.status = "Logging in…"
			return m, cmdLogin(m.username, m.password)
		case "esc":
			m.screen = screenUsername
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
					cmdPollMessages(m.token, m.convId, nil, m.myId, m.usernameCache),
					textinput.Blink,
				)
			}
		case "n":
			m.screen = screenNewChat
			m.errStr = ""
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Usernames (comma-separated)"
			m.textInput.EchoMode = textinput.EchoNormal
			m.textInput.Focus()
			return m, textinput.Blink
		case "r":
			m.loading = true
			m.status = "Refreshing…"
			return m, cmdLoadConversations(m.token)
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
			usernames := []string{m.myName}
			for _, u := range strings.Split(val, ",") {
				u = strings.TrimSpace(u)
				if u != "" && u != m.myName {
					usernames = append(usernames, u)
				}
			}
			m.loading = true
			m.errStr = ""
			m.status = "Creating conversation…"
			return m, cmdCreateConversation(m.token, usernames)
		case "esc":
			m.screen = screenConversations
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
			m.screen = screenConversations
			m.fadingActive = false
			m.loading = true
			m.status = "Loading conversations…"
			return m, cmdLoadConversations(m.token)
		case "enter":
			text := strings.TrimSpace(m.textInput.Value())
			if text == "" {
				return m, nil
			}
			m.textInput.SetValue("")

			// optimistic — own messages never get a tilde
			line := myMsgStyle.Render(m.myName+": ") + text
			m.chatLines = append(m.chatLines, chatLine{
				text:     line,
				fromPoll: false,
			})
			if m.vpReady {
				m.viewport.SetContent(m.renderChatContent())
				m.viewport.GotoBottom()
			}
			return m, cmdSendMessage(m.token, m.convId, text)
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

// ═══════════════════════════════════════════════════════════════════════════
// View
// ═══════════════════════════════════════════════════════════════════════════

func (m model) View() string {
	if m.loading {
		return m.frame(statusStyle.Render("⏳ "+m.status) + "\n")
	}
	switch m.screen {
	case screenAuthChoice:
		return m.viewAuthChoice()
	case screenUsername:
		return m.viewUsername()
	case screenPassword:
		return m.viewPassword()
	case screenConversations:
		return m.viewConversations()
	case screenNewChat:
		return m.viewNewChat()
	case screenChat:
		return m.viewChat()
	}
	return ""
}

// ── frame helper: centres content with optional error ───────────────────────

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

// ── auth choice ─────────────────────────────────────────────────────────────

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

// ── username ────────────────────────────────────────────────────────────────

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

// ── password ────────────────────────────────────────────────────────────────

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

// ── conversation list ───────────────────────────────────────────────────────

func (m model) viewConversations() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Logged in as %s\n\n", selectedStyle.Render(m.myName)))

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
			members := strings.Join(c.Members, ", ")
			line := fmt.Sprintf("%s%s", cursor, style.Render(members))
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", min(m.width, 50))) + "\n")
	b.WriteString(helpStyle.Render("↑/↓ navigate • enter open • n new chat • r refresh • q quit") + "\n")

	return m.frame(b.String())
}

// ── new chat ────────────────────────────────────────────────────────────────

func (m model) viewNewChat() string {
	body := fmt.Sprintf(
		"New conversation — who do you want to chat with?\n\n  %s\n\n%s\n",
		m.textInput.View(),
		helpStyle.Render("enter comma-separated usernames • esc to cancel"),
	)
	return m.frame(body)
}

// ── chat ────────────────────────────────────────────────────────────────────

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

// ═══════════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════════

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ═══════════════════════════════════════════════════════════════════════════
// Main
// ═══════════════════════════════════════════════════════════════════════════

func main() {
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
