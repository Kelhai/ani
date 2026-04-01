package services

import (
	"fmt"

	"github.com/Kelhai/ani/common"
)

type AuthService struct{}

func SetupAuthService() AuthService {
	return AuthService{}
}

func (as AuthService) Login(username, password string) (common.User, error) {

	return common.User{}, nil
}

func (as AuthService) GetSessionByToken(token string) (*common.Session, error) {
	session, err := pgStorage.GetSessionByToken(token)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	return session, nil
}
