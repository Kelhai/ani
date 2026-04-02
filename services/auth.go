package services

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

type AuthService struct{}

func SetupAuthService() AuthService {
	return AuthService{}
}

func (as AuthService) VerifyPassword(username, password string) (*common.User, error) {
	// customer errors for if user doesnt exist or error?
	user, err := pgStorage.GetUserByUsername(username)
	if err != nil {
		log.Printf("Failed to get user: %s", err.Error())
		return nil, errors.New("user does not exist")
	}

	fullHash := strings.Split(user.PasswordHash, "$")
	salt, err := base64.StdEncoding.DecodeString(fullHash[0])
	if err != nil {
		return nil, errors.New("Failed to base64 decode salt")
	}

	storedHash, err := base64.StdEncoding.DecodeString(fullHash[1])
	if err != nil {
		return nil, errors.New("Failed to base64 decode hash")
	}

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	if subtle.ConstantTimeCompare(hash, storedHash) != 1 {
		return nil, common.ErrInvalidLogin
	}

	return user, nil
}

func (as AuthService) StartSession(userId uuid.UUID) (*common.Session, error) {
	session, err := pgStorage.NewSession(userId)

	return session, err
}

func (as AuthService) GetSessionByToken(token uuid.UUID) (*common.Session, error) {
	session, err := pgStorage.GetSessionByToken(token)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	return session, nil
}

func (as AuthService) CreateUser(username, password string) (*common.User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		log.Printf("Failed to generate UUID: %s", err.Error())
		return nil, fmt.Errorf("%w: %w", common.ErrUuidFailed, err)
	}

	salt := make([]byte, 16)
	rand.Read(salt)

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	stored := base64.StdEncoding.EncodeToString(salt) + "$" +
		base64.StdEncoding.EncodeToString(hash)

	user := common.User{
		Id:           id,
		Username:     username,
		PasswordHash: stored,
	}

	err = pgStorage.AddUser(user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
