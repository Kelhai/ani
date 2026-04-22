package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Kelhai/ani/common"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/google/uuid"
)

type AuthService struct{}

func SetupAuthService() AuthService {
	return AuthService{}
}

func (as AuthService) VerifyEnvelope(envelope common.AuthEnvelope) (*common.User, error) {
	if time.Now().After(envelope.Blob.TimeToLive) {
		return nil, common.ErrInvalidLogin
	}

	if envelope.Blob.SignatureAlgorithm != "ML-DSA-87" {
		return nil, common.ErrInvalidLogin
	}

	user, err := pgStorage.GetUserByUsername(envelope.Blob.Username)
	if err != nil {
		return nil, common.ErrInvalidLogin
	}

	if user.IdentityPk == nil {
		return nil, common.ErrInvalidLogin
	}

	blobBytes, err := json.Marshal(envelope.Blob)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal blob: %w", err)
	}

	pk := new(mldsa87.PublicKey)
	if err := pk.UnmarshalBinary(user.IdentityPk); err != nil {
		return nil, fmt.Errorf("failed to unmarshal identity key: %w", err)
	}

	if !mldsa87.Verify(pk, blobBytes, nil, envelope.Signature) {
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

func (as AuthService) CreateUser(username string, identityPk []byte) (*common.User, error) {
	if len(identityPk) != mldsa87.PublicKeySize {
		return nil, fmt.Errorf("invalid identity key size")
	}

	// validate it's actually a parseable key
	pk := new(mldsa87.PublicKey)
	if err := pk.UnmarshalBinary(identityPk); err != nil {
		return nil, fmt.Errorf("invalid identity key: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		log.Printf("Failed to generate UUID: %s", err.Error())
		return nil, fmt.Errorf("%w: %w", common.ErrUuidFailed, err)
	}

	user := common.User{
		Id:         id,
		Username:   username,
		IdentityPk: identityPk,
	}

	if err = pgStorage.AddUser(user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (as AuthService) GetUsernamesByIds(userIds []uuid.UUID) (map[uuid.UUID]string, error) {
	users, err := pgStorage.GetUsersByIds(userIds)
	if err != nil {
		log.Printf("Failed to get users by ids: %s", err.Error())
		return nil, fmt.Errorf("Failed to get users by ids: %w", err)
	}

	return users, nil
}

func (as AuthService) GetUserById(userId uuid.UUID) (*common.User, error) {
	user, err := pgStorage.GetUser(userId)
	if err != nil {
		return nil, err
	}

	return user, nil
}
