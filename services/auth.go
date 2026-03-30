package services

import "github.com/Kelhai/ani/common"

type AuthService struct {}

func SetupAuthService() AuthService {
	return AuthService{}
}

func (as AuthService) Login(username, password string) (common.User, error) {
	
	return common.User{}, nil
}

