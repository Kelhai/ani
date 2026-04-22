package services

import (
	"crypto/mlkem"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Kelhai/ani/client"
	"github.com/Kelhai/ani/client/storage"
	"github.com/Kelhai/ani/common"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/google/uuid"
)

type AuthService struct{}

func (_ AuthService) Register(username, password string) error {
	storage.MasterKey = storage.DeriveMasterKey(password, username)

	// generate signing keypair
	pk, sk, err := mldsa87.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("failed to generate identity keypair: %w", err)
	}
	pkBytes, err := pk.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal identity public key: %w", err)
	}
	skBytes, err := sk.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal identity private key: %w", err)
	}

	// generate KEM key
	kemDk, err := mlkem.GenerateKey768()
	if err != nil {
		return fmt.Errorf("failed to generate KEM keypair: %w", err)
	}
	kemEkBytes := kemDk.EncapsulationKey().Bytes()
	kemDkBytes := kemDk.Bytes()

	// sign the KEM public key
	kemPkSig, err := sk.Sign(rand.Reader, kemEkBytes, nil)
	if err != nil {
		return fmt.Errorf("failed to sign KEM public key: %w", err)
	}

	// save identity keypair
	identityKeyId, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("failed to generate identity key UUID: %w", err)
	}
	if err := storage.SaveKeyPair(username, identityKeyId, pkBytes, skBytes); err != nil {
		return fmt.Errorf("failed to save identity keypair: %w", err)
	}
	if err := storage.AddLegendEntry(username, identityKeyId, storage.LegendEntry{
		Tag:     storage.KeyTagIdentity,
		Type:    "ML-DSA-87",
		Created: time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to update legend: %w", err)
	}

	// save KEM keypair
	kemKeyId, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("failed to generate KEM key UUID: %w", err)
	}
	if err := storage.SaveKeyPair(username, kemKeyId, kemEkBytes, kemDkBytes); err != nil {
		return fmt.Errorf("failed to save KEM keypair: %w", err)
	}
	if err := storage.AddLegendEntry(username, kemKeyId, storage.LegendEntry{
		Tag:     storage.KeyTagKem,
		Type:    "ML-KEM-768",
		Created: time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to update legend: %w", err)
	}

	payload := common.RegisterRequest{
		Username:       username,
		IdentityPk:     pkBytes,
		KemPk:          kemEkBytes,
		KemPkSignature: kemPkSig,
	}

	status, body, err := apiService.RawRequest("POST", "/auth/register", payload, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}
	if status != http.StatusCreated {
		if status == http.StatusConflict {
			return client.ErrUsernameTaken
		}
		return fmt.Errorf("invalid status code: %d", status)
	}

	user := new(common.User)
	if err := json.Unmarshal(body, user); err != nil {
		return fmt.Errorf("failed to unmarshal user response: %w", err)
	}

	client.User = user
	return nil
}

func (_ AuthService) Login(username, password string) error {
	storage.MasterKey = storage.DeriveMasterKey(password, username)

	keyId, _, err := storage.FindKeyByTag(username, storage.KeyTagIdentity)
	if err != nil {
		return fmt.Errorf("failed to find identity key: %w", err)
	}

	skBytes, err := storage.LoadPrivKey(username, keyId)
	if err != nil {
		return fmt.Errorf("failed to load identity private key: %w", err)
	}

	var sk mldsa87.PrivateKey
	if err := sk.UnmarshalBinary(skBytes); err != nil {
		return fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	blob := common.AuthBlob{
		SignatureAlgorithm: "ML-DSA-87",
		Username:           username,
		SignedTime:         time.Now(),
		TimeToLive:         time.Now().Add(5 * time.Minute),
		Uuid:               uuid.Must(uuid.NewV7()),
	}

	blobBytes, err := json.Marshal(blob)
	if err != nil {
		return fmt.Errorf("failed to marshal auth blob: %w", err)
	}

	sig, err := sk.Sign(rand.Reader, blobBytes, nil)
	if err != nil {
		return fmt.Errorf("failed to sign auth blob: %w", err)
	}

	envelope := common.AuthEnvelope{
		Blob:      blob,
		Signature: sig,
	}

	status, body, err := apiService.RawRequest("POST", "/auth/login", envelope, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}
	if status != http.StatusOK {
		if status == http.StatusUnauthorized {
			return client.ErrLoginFailed
		}
		return fmt.Errorf("invalid status code: %d", status)
	}

	session := new(common.Session)
	if err := json.Unmarshal(body, session); err != nil {
		return fmt.Errorf("failed to unmarshal session: %w", err)
	}

	client.User = &common.User{
		Id:       session.UserId,
		Username: username,
	}
	SessionToken = session.Id
	return nil
}
