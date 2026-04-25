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

func (as AuthService) CreateUser(username string, identityPk, kemPk, kemPkSig []byte) (*common.User, error) {
	if len(identityPk) != mldsa87.PublicKeySize {
		return nil, fmt.Errorf("invalid identity key size")
	}
	pk := new(mldsa87.PublicKey)
	if err := pk.UnmarshalBinary(identityPk); err != nil {
		return nil, fmt.Errorf("invalid identity key: %w", err)
	}

	// verify the KEM public key is signed by the identity key
	if !mldsa87.Verify(pk, kemPk, nil, kemPkSig) {
		return nil, fmt.Errorf("KEM key signature invalid")
	}

	id, err := uuid.NewV7()
	if err != nil {
		log.Printf("Failed to generate UUID: %s", err.Error())
		return nil, fmt.Errorf("%w: %w", common.ErrUuidFailed, err)
	}

	user := common.User{
		Id:             id,
		Username:       username,
		IdentityPk:     identityPk,
		KemPk:          kemPk,
		KemPkSignature: kemPkSig,
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

func (as AuthService) GetUserByUsername(username string) (*common.User, error) {
	return pgStorage.GetUserByUsername(username)
}

func (as AuthService) GetUserById(userId uuid.UUID) (*common.User, error) {
	user, err := pgStorage.GetUser(userId)
	if err != nil {
		return nil, err
	}

	return user, nil
}
