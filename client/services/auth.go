package services

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Kelhai/ani/client"
	"github.com/Kelhai/ani/common"
)

type AuthService struct{}

func (_ AuthService) Register(username, password string) error {
	userMap := map[string]string{"username": username, "password": password}
	status, body, err := apiService.RawRequest("POST", "/auth/register", userMap, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return fmt.Errorf("Failed to register: %w", err)
	}

	if status != http.StatusCreated {
		if status == http.StatusConflict {
			return client.ErrUsernameTaken
		}
		return fmt.Errorf("Invalid status code: %d", status)
	}

	user := new(common.User)
	err = json.Unmarshal(body, user)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal user response: %w", err)
	}

	client.User = user

	return nil
}

