package services

import (
	"crypto/mlkem"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/Kelhai/ani/client/storage"
	"github.com/Kelhai/ani/common"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"golang.org/x/crypto/chacha20poly1305"
)

func InitSession(
	peerIdentityPk []byte,
	peerKemPk []byte,
	peerKemPkSig []byte,
) (*storage.RatchetState, *common.RatchetHeader, error) {
	// verify peer's KEM key is signed by their identity key
	identityPk := new(mldsa87.PublicKey)
	if err := identityPk.UnmarshalBinary(peerIdentityPk); err != nil {
		return nil, nil, fmt.Errorf("invalid peer identity key: %w", err)
	}
	if !mldsa87.Verify(identityPk, peerKemPk, nil, peerKemPkSig) {
		return nil, nil, fmt.Errorf("KEM key signature verification failed — possible MITM")
	}

	// encapsulate to peer's KEM public key
	ek, err := mlkem.NewEncapsulationKey768(peerKemPk)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse peer KEM key: %w", err)
	}
	ciphertext, sharedSecret := ek.Encapsulate()

	// generate our own ephemeral KEM keypair for the ratchet
	ourKem, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ratchet KEM keypair: %w", err)
	}

	state := &storage.RatchetState{
		SendChainKey: storage.DeriveKey(sharedSecret, "send"),
		RecvChainKey: storage.DeriveKey(sharedSecret, "recv"),
		PeerKemPk:    peerKemPk,
		KemPk:        ourKem.EncapsulationKey().Bytes(),
		KemSk:        ourKem.Bytes(),
	}

	init := &common.RatchetHeader{
		KemCiphertext: ciphertext,
		SenderKemPk:   ourKem.EncapsulationKey().Bytes(),
	}

	return state, init, nil
}

func InitSessionFromPeer(
	localKemSk []byte,
	header common.RatchetHeader,
) (*storage.RatchetState, error) {
	dk, err := mlkem.NewDecapsulationKey768(localKemSk)
	if err != nil {
		return nil, fmt.Errorf("failed to parse local KEM secret key: %w", err)
	}
	sharedSecret, err := dk.Decapsulate(header.KemCiphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decapsulate: %w", err)
	}

	ourKem, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ratchet KEM keypair: %w", err)
	}

	return &storage.RatchetState{
		SendChainKey: storage.DeriveKey(sharedSecret, "recv"),
		RecvChainKey: storage.DeriveKey(sharedSecret, "send"),
		PeerKemPk:    header.SenderKemPk,
		KemPk:        ourKem.EncapsulationKey().Bytes(),
		KemSk:        ourKem.Bytes(),
	}, nil
}

// advanceSymmetricChain derives a message key and advances the chain key
func advanceSymmetricChain(chainKey []byte) (newChainKey, messageKey []byte) {
	newChainKey = storage.DeriveKey(chainKey, "chain")
	messageKey = storage.DeriveKey(chainKey, "message")
	return
}

// kemRatchetStep performs a KEM ratchet step, injecting new entropy into the chain
func kemRatchetStep(state *storage.RatchetState, sharedSecret []byte) error {
	ourKem, err := mlkem.GenerateKey768()
	if err != nil {
		return fmt.Errorf("failed to generate new KEM keypair: %w", err)
	}

	// inject shared secret into both chain keys
	state.SendChainKey = storage.DeriveKey(
		append(state.SendChainKey, sharedSecret...),
		"ratchet",
	)
	state.KemPk = ourKem.EncapsulationKey().Bytes()
	state.KemSk = ourKem.Bytes()

	return nil
}

func RatchetEncrypt(state *storage.RatchetState, plaintext, aad []byte, identitySk *mldsa87.PrivateKey) ([]byte, common.RatchetHeader, []byte, error) {
	header := common.RatchetHeader{}

	if state.KemRatchetDue {
		ek, err := mlkem.NewEncapsulationKey768(state.PeerKemPk)
		if err != nil {
			return nil, common.RatchetHeader{}, nil, fmt.Errorf("failed to parse peer KEM key: %w", err)
		}
		ciphertext, sharedSecret := ek.Encapsulate()
		if err := kemRatchetStep(state, sharedSecret); err != nil {
			return nil, common.RatchetHeader{}, nil, err
		}
		header.KemCiphertext = ciphertext
		header.SenderKemPk = state.KemPk
		state.KemRatchetDue = false
	}

	newChainKey, messageKey := advanceSymmetricChain(state.SendChainKey)
	state.SendChainKey = newChainKey

	encrypted, err := encryptMessage(plaintext, messageKey, aad)
	if err != nil {
		return nil, common.RatchetHeader{}, nil, err
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, common.RatchetHeader{}, nil, fmt.Errorf("failed to marshal header for signing: %w", err)
	}
	sigPayload := append(encrypted, headerBytes...)
	sig, err := identitySk.Sign(rand.Reader, sigPayload, nil)
	if err != nil {
		return nil, common.RatchetHeader{}, nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return encrypted, header, sig, nil
}

func RatchetDecrypt(state *storage.RatchetState, ciphertext []byte, header common.RatchetHeader, sig []byte, aad []byte, senderIdentityPk *mldsa87.PublicKey) ([]byte, error) {
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal header for verification: %w", err)
	}
	sigPayload := append(ciphertext, headerBytes...)
	if !mldsa87.Verify(senderIdentityPk, sigPayload, nil, sig) {
		return nil, fmt.Errorf("message signature verification failed")
	}

	if len(header.KemCiphertext) > 0 {
		dk, err := mlkem.NewDecapsulationKey768(state.KemSk)
		if err != nil {
			return nil, fmt.Errorf("failed to parse our KEM secret key: %w", err)
		}
		sharedSecret, err := dk.Decapsulate(header.KemCiphertext)
		if err != nil {
			return nil, fmt.Errorf("failed to decapsulate: %w", err)
		}
		state.RecvChainKey = storage.DeriveKey(
			append(state.RecvChainKey, sharedSecret...),
			"ratchet",
		)
		state.PeerKemPk = header.SenderKemPk
		state.KemRatchetDue = true
	}

	newChainKey, messageKey := advanceSymmetricChain(state.RecvChainKey)
	state.RecvChainKey = newChainKey
	return decryptMessage(ciphertext, messageKey, aad)
}

func encryptMessage(plaintext, key, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plaintext, aad), nil
}

func decryptMessage(ciphertext, key, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return aead.Open(nil, nonce, ciphertext, aad)
}
