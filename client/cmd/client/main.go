package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Kelhai/ani/client"
	"github.com/Kelhai/ani/client/config"
	"github.com/Kelhai/ani/client/services"
	"github.com/Kelhai/ani/client/storage"
	"github.com/Kelhai/ani/common"
	"github.com/charmbracelet/x/term"
	"github.com/google/uuid"
)

var (
	authSvc    = services.AuthService{}
	messageSvc = services.MessageService{}
)

func main() {
	err := config.SetupConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	services.SetupApiService()

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "register":
		cmdRegister()
	case "login":
		cmdLogin()
	case "logout":
		cmdLogout()
	case "conversations":
		cmdConversations()
	case "message":
		cmdMessage()
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("usage:")
	fmt.Println("  ani register <username>")
	fmt.Println("  ani login <username>")
	fmt.Println("  ani logout")
	fmt.Println("  ani conversations")
	fmt.Println("  ani message <username>")
}

func cmdRegister() {
	if len(os.Args) < 3 {
		fatal("usage: ani register <username>")
	}
	username := os.Args[2]
	password := promptPassword("password: ")
	confirm := promptPassword("confirm password: ")
	if password != confirm {
		fatal("passwords do not match")
	}

	if err := authSvc.Register(username, password); err != nil {
		fatal("register failed: %v", err)
	}
	fmt.Printf("registered as %s\n", username)
}

func cmdLogin() {
	if len(os.Args) < 3 {
		fatal("usage: ani login <username>")
	}
	username := os.Args[2]
	password := promptPassword("password: ")

	storage.MasterKey = storage.DeriveMasterKey(password, username)

	if err := authSvc.Login(username, password); err != nil {
		fatal("login failed: %v", err)
	}
	fmt.Printf("logged in as %s\n", username)
}

func cmdLogout() {
	if err := storage.ClearSession(); err != nil {
		fatal("logout failed: %v", err)
	}
	fmt.Println("logged out")
}

func cmdConversations() {
	mustLoadSession()

	convs, err := messageSvc.GetConversations()
	if err != nil {
		fatal("failed to get conversations: %v", err)
	}
	if len(convs) == 0 {
		fmt.Println("no conversations")
		return
	}
	for _, c := range convs {
		fmt.Printf("%s  [%s]\n", c.Id, strings.Join(c.Members, ", "))
	}
}

func cmdMessage() {
	if len(os.Args) < 3 {
		fatal("usage: ani message <username>")
	}
	peer := os.Args[2]
	me := mustLoadSession()

	convId, _ := getOrCreateConv(me, peer)

	fmt.Printf("chatting with %s (ctrl+c to quit)\n\n", peer)

	msgs, err := messageSvc.GetMessages(convId, me, nil)
	if err != nil {
		fatal("failed to load messages: %v", err)
	}
	printMessages(msgs, me)

	var lastMessageId *uuid.UUID
	if len(msgs) > 0 {
		id := msgs[len(msgs)-1].Id
		lastMessageId = &id
	}

	incoming := make(chan []common.DecryptedMessage, 8)
	go pollLoop(convId, me, &lastMessageId, incoming)

	inputCh := make(chan string)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputCh <- scanner.Text()
		}
		close(inputCh)
	}()

	fmt.Printf("%s > ", me)

	for {
		select {
		case newMsgs := <-incoming:
			if len(newMsgs) > 0 {
				fmt.Println()
				printMessages(newMsgs, me)
				fmt.Printf("%s > ", me)
			}

		case text, ok := <-inputCh:
			if !ok {
				return
			}

			text = strings.TrimSpace(text)
			if text == "" {
				fmt.Printf("%s > ", me)
				continue
			}

			_, err := messageSvc.SendMessage(convId, me, peer, text)
			if err != nil {
				fmt.Fprintf(os.Stderr, "send failed: %v\n", err)
			}

			fmt.Printf("%s > ", me)
		}
	}
}


func mustLoadSession() string {
	sess, err := storage.LoadSession()
	if err != nil || sess == nil {
		fatal("not logged in — run: ani login <username>")
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = storage.ClearSession()
		fatal("session expired — run: ani login <username>")
	}

	services.SessionToken = sess.Token
	storage.MasterKey = nil
	client.User = &common.User{
		Id:       sess.UserId,
		Username: sess.Username,
	}

	if storage.MasterKey == nil {
		password := promptPassword("master password: ")
		storage.MasterKey = storage.DeriveMasterKey(password, sess.Username)
	}

	return sess.Username
}

func getOrCreateConv(me, peer string) (uuid.UUID, common.ConversationWithUsernames) {
	convs, err := messageSvc.GetConversations()
	if err != nil {
		fatal("failed to list conversations: %v", err)
	}

	for _, c := range convs {
		members := map[string]bool{}
		for _, m := range c.Members {
			members[m] = true
		}
		if len(c.Members) == 2 && members[me] && members[peer] {
			return c.Id, c
		}
	}

	id, err := messageSvc.CreateConversation([]string{me, peer})
	if err != nil {
		fatal("failed to create conversation: %v", err)
	}
	conv := common.ConversationWithUsernames{
		Id:      *id,
		Members: []string{me, peer},
	}
	return *id, conv
}

func pollLoop(
	convId uuid.UUID,
	me string,
	lastId **uuid.UUID,
	ch chan<- []common.DecryptedMessage,
) {
	for {
		time.Sleep(2 * time.Second)
		msgs, err := messageSvc.GetMessages(convId, me, *lastId)
		if err != nil || len(msgs) == 0 {
			continue
		}
		var incoming []common.DecryptedMessage
		for _, m := range msgs {
			if m.Sender != me {
				incoming = append(incoming, m)
			}
		}
		id := msgs[len(msgs)-1].Id
		*lastId = &id
		if len(incoming) > 0 {
			ch <- incoming
		}
	}
}

func printMessages(msgs []common.DecryptedMessage, me string) {
	for _, m := range msgs {
		if m.Sender == me {
			continue
		}
		fmt.Printf("%s > %s\n", m.Sender, m.Content)
	}
}

func promptPassword(prompt string) string {
	fmt.Print(prompt)
	b, err := term.ReadPassword(os.Stdin.Fd())
	fmt.Println()
	if err != nil {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return scanner.Text()
		}
		return ""
	}
	return string(b)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
