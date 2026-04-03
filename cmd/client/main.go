package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	baseURL  = "http://localhost:52971"
	token    uuid.UUID
	myUser   struct {
		Id       uuid.UUID `json:"id"`
		Username string    `json:"username"`
	}
	client = &http.Client{}
)

func init() {
	if port := os.Getenv("ANI_PORT"); port != "" {
		baseURL = "http://localhost:" + port
	}
}

func authRequest(path, username, password string) (*http.Response, error) {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req, _ := http.NewRequest("POST", baseURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func authedRequest(method, path string, body any) (*http.Response, error) {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, baseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.String())
	return client.Do(req)
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("(l)ogin or (r)egister? ")
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	fmt.Print("username: ")
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())

	fmt.Print("password: ")
	scanner.Scan()
	password := strings.TrimSpace(scanner.Text())

	if choice == "r" {
		resp, err := authRequest("/auth/register", username, password)
		if err != nil || resp.StatusCode != http.StatusCreated {
			fmt.Println("registration failed")
			os.Exit(1)
		}
		resp.Body.Close()
		fmt.Println("registered successfully")
	}

	resp, err := authRequest("/auth/login", username, password)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Println("login failed")
		os.Exit(1)
	}
	var loginResp struct {
		Token    uuid.UUID `json:"token"`
		Id       uuid.UUID `json:"id"`
		Username string    `json:"username"`
	}
	json.NewDecoder(resp.Body).Decode(&loginResp)
	resp.Body.Close()
	token = loginResp.Token
	myUser.Id = loginResp.Id
	myUser.Username = loginResp.Username

	fmt.Printf("logged in as %s\n", myUser.Username)

	resp, err = authedRequest("GET", "/messages/conversations", nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		var conversations []struct {
			Id      uuid.UUID `json:"id"`
			Members []string  `json:"members"`
		}
		json.NewDecoder(resp.Body).Decode(&conversations)
		resp.Body.Close()

		if len(conversations) > 0 {
			fmt.Println("existing conversations:")
			for _, c := range conversations {
				fmt.Printf("  [%s] with %s\n", c.Id, strings.Join(c.Members, ", "))
			}
		}
	}

	fmt.Print("chat with (comma separated usernames): ")
	scanner.Scan()
	usernames := []string{myUser.Username}
	for _, p := range strings.Split(scanner.Text(), ",") {
		usernames = append(usernames, strings.TrimSpace(p))
	}

	resp, err = authedRequest("POST", "/messages/conversation", map[string]any{"usernames": usernames})
	if err != nil || resp.StatusCode != http.StatusCreated {
		fmt.Println("failed to get or create conversation")
		os.Exit(1)
	}
	var convResp struct {
		ConversationId uuid.UUID `json:"conversationId"`
	}
	json.NewDecoder(resp.Body).Decode(&convResp)
	resp.Body.Close()
	conversationId := convResp.ConversationId

	// build a sender id -> username cache
	usernameCache := map[uuid.UUID]string{
		myUser.Id: myUser.Username,
	}

	fmt.Println("--- chat started, type to send, ctrl+c to quit ---")

	var lastMessageId *uuid.UUID

	go func() {
		for {
			time.Sleep(2 * time.Second)

			path := fmt.Sprintf("/messages/conversation/%s", conversationId)
			if lastMessageId != nil {
				path = fmt.Sprintf("/messages/conversation/%s/%s", conversationId, lastMessageId)
			}

			resp, err := authedRequest("GET", path, nil)
			if err != nil || resp.StatusCode != http.StatusOK {
				continue
			}

			var messages []struct {
				Id       uuid.UUID `json:"id"`
				SenderId uuid.UUID `json:"senderId"`
				Message  string    `json:"message"`
			}
			json.NewDecoder(resp.Body).Decode(&messages)
			resp.Body.Close()

			for _, m := range messages {
				id := m.Id
				lastMessageId = &id

				// skip your own messages
				if m.SenderId == myUser.Id {
					continue
				}

				// resolve sender username if not cached
				if _, ok := usernameCache[m.SenderId]; !ok {
					resp, err := authedRequest("GET", fmt.Sprintf("/auth/user/%s", m.SenderId), nil)
					if err == nil && resp.StatusCode == http.StatusOK {
						var u struct {
							Username string `json:"username"`
						}
						json.NewDecoder(resp.Body).Decode(&u)
						resp.Body.Close()
						usernameCache[m.SenderId] = u.Username
					}
				}

				sender := usernameCache[m.SenderId]
				if sender == "" {
					sender = m.SenderId.String()
				}
				fmt.Printf("\r%s: %s\n> ", sender, m.Message)
			}
		}
	}()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		resp, err := authedRequest("POST", fmt.Sprintf("/messages/m/%s", conversationId), map[string]string{"message": text})
		if err != nil || resp.StatusCode != http.StatusCreated {
			fmt.Println("failed to send message")
			continue
		}
		resp.Body.Close()
	}
}
