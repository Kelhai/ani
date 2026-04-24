package services

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Kelhai/ani/client"
	"github.com/Kelhai/ani/client/storage"
	"github.com/Kelhai/ani/common"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/google/uuid"
)

type MessageService struct{}

func (_ MessageService) GetConversations() ([]common.ConversationWithUsernames, error) {
	statusCode, conversationsBytes, err := apiService.GET("/messages/conversations", nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNoContent {
		return []common.ConversationWithUsernames{}, nil
	} else if statusCode != http.StatusOK {
		return nil, client.ErrUnknownErr
	}

	conversations := new([]common.ConversationWithUsernames)

	err = json.Unmarshal(conversationsBytes, &conversations)
	if err != nil {
		return nil, client.ErrJsonUnmarshal
	}
	
	return *conversations, nil
}

func (_ MessageService) CreateConversation(usernames []string) (*uuid.UUID, error) {
	usernameBody := map[string][]string{
		"usernames": usernames,
	}

	statusCode, conversation, err := apiService.POST("/messages/conversation", usernameBody)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusCreated {
		return nil, client.ErrUnknownErr
	}

	response := new(struct{
		ConversationId uuid.UUID `json:"conversationId"`
	})

	err = json.Unmarshal(conversation, response)
	if err != nil {
		return nil, client.ErrJsonUnmarshal
	}
	
	return &response.ConversationId, nil
}

func (_ MessageService) GetMessageFromConversation(
	conversationId uuid.UUID,
	currentUsername string,
	lastMessage *uuid.UUID,
) ([]common.DecryptedMessage, error) {

	path := fmt.Sprintf("/messages/conversation/%s", conversationId)
	if lastMessage != nil {
		path += "/" + lastMessage.String()
	}

	statusCode, messagesJson, err := apiService.GET(path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNoContent {
		return []common.DecryptedMessage{}, nil
	}
	if statusCode != http.StatusOK {
		return nil, client.ErrUnknownErr
	}

	var messages []common.ShortMessage
	if err := json.Unmarshal(messagesJson, &messages); err != nil {
		return nil, client.ErrJsonUnmarshal
	}
	log.Printf("%#v\n", messages)

	var decrypted []common.DecryptedMessage

	for _, rm := range messages {
		peer := rm.Sender

		ratchetKeyId, _, err := storage.FindKeyByPeer(currentUsername, storage.KeyTagRatchet, peer)
		if err != nil {
			peerUser, err := AuthService{}.GetUserKeys(peer)
			if err != nil {
				log.Printf("no ratchet state and cannot fetch peer keys: %v", err)
				continue
			}

			state, initHeader, err := InitSession(
				peerUser.IdentityPk,
				peerUser.KemPk,
				peerUser.KemPkSignature,
			)
			if err != nil {
				log.Printf("failed to init session for incoming message: %v", err)
				continue
			}

			state.InitHeader = initHeader

			ratchetKeyId = uuid.New()
			_ = storage.SaveRatchetState(currentUsername, peer, ratchetKeyId, *state)
			_ = storage.AddLegendEntry(currentUsername, ratchetKeyId, storage.LegendEntry{
				Tag:     storage.KeyTagRatchet,
				Type:    peer,
				Created: time.Now(),
			})
		}

		state, err := storage.LoadRatchetState(currentUsername, peer, ratchetKeyId)
		if err != nil || state == nil {
			log.Println("failed to load ratchet")
			continue
		}

		peerUser, err := AuthService{}.GetUserKeys(peer)
		if err != nil {
			log.Println("Failed to get user keys")
			continue
		}

		var senderPk mldsa87.PublicKey
		if err := senderPk.UnmarshalBinary(peerUser.IdentityPk); err != nil {
			log.Println("failed to unmarshal sender public key")
			continue
		}

		plaintext, err := RatchetDecrypt(
			state,
			rm.Ciphertext,
			rm.Header,
			rm.Signature,
			conversationId[:],
			&senderPk,
		)
		if err != nil {
			log.Printf("ratchet decryption failed: %v\n", err)
			continue
		}

		_ = storage.SaveRatchetState(currentUsername, peer, ratchetKeyId, *state)

		decrypted = append(decrypted, common.DecryptedMessage{
			Id:      rm.Id,
			Sender:  rm.Sender,
			Content: string(plaintext),
		})
	}

	// optional: store decrypted locally
	if len(decrypted) > 0 {
		_ = storage.AppendMessages(currentUsername, conversationId, nil)
	}

	return decrypted, nil
}

func (_ MessageService) SendMessage(
	conversationId uuid.UUID,
	myUsername string,
	recipients []string,
	text string,
) (*uuid.UUID, error) {

	if len(recipients) != 1 {
		return nil, fmt.Errorf("pairwise only supports exactly one recipient")
	}
	peer := recipients[0]

	identityKeyId, _, err := storage.FindKeyByTag(myUsername, storage.KeyTagIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to find identity key: %w", err)
	}

	skBytes, err := storage.LoadPrivKey(myUsername, identityKeyId)
	if err != nil {
		return nil, fmt.Errorf("failed to load identity secret key: %w", err)
	}

	var identitySk mldsa87.PrivateKey
	if err := identitySk.UnmarshalBinary(skBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal identity secret key: %w", err)
	}

	// Load or initialize ratchet
	ratchetKeyId, _, err := storage.FindKeyByPeer(myUsername, storage.KeyTagRatchet, peer)
	if err != nil {
		peerUser, err := AuthService{}.GetUserKeys(peer)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch peer keys: %w", err)
		}

		state, initHeader, err := InitSession(
			peerUser.IdentityPk,
			peerUser.KemPk,
			peerUser.KemPkSignature,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to init session: %w", err)
		}

		state.InitHeader = initHeader

		ratchetKeyId, err = uuid.NewV7()
		if err != nil {
			return nil, err
		}

		if err := storage.SaveRatchetState(myUsername, peer, ratchetKeyId, *state); err != nil {
			return nil, fmt.Errorf("failed to save ratchet state: %w", err)
		}

		if err := storage.AddLegendEntry(myUsername, ratchetKeyId, storage.LegendEntry{
			Tag:     storage.KeyTagRatchet,
			Type:    peer,
			Created: time.Now(),
		}); err != nil {
			return nil, err
		}
	}

	ratchetState, err := storage.LoadRatchetState(myUsername, peer, ratchetKeyId)
	if err != nil {
		return nil, fmt.Errorf("failed to load ratchet state: %w", err)
	}

	aad := conversationId[:]

	var (
		ciphertext []byte
		header     common.RatchetHeader
		errEnc     error
	)

	if ratchetState.InitHeader != nil {
		// first message in session
		header = *ratchetState.InitHeader
		ratchetState.InitHeader = nil

		newChainKey, messageKey := advanceSymmetricChain(ratchetState.SendChainKey)
		ratchetState.SendChainKey = newChainKey

		ciphertext, errEnc = encryptMessage([]byte(text), messageKey, aad)
		if errEnc != nil {
			return nil, fmt.Errorf("encrypt failed: %w", errEnc)
		}
	} else {
		// normal ratchet path
		ciphertext, header, _, errEnc = RatchetEncrypt(
			ratchetState,
			[]byte(text),
			aad,
			&identitySk,
		)
		if errEnc != nil {
			return nil, fmt.Errorf("ratchet encrypt failed: %w", errEnc)
		}
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("marshal header failed: %w", err)
	}

	sigPayload := append(ciphertext, headerBytes...)

	sig, err := identitySk.Sign(rand.Reader, sigPayload, nil)
	if err != nil {
		return nil, fmt.Errorf("sign failed: %w", err)
	}

	// Persist ratchet state
	if err := storage.SaveRatchetState(myUsername, peer, ratchetKeyId, *ratchetState); err != nil {
		return nil, fmt.Errorf("failed to save ratchet state: %w", err)
	}

	payload := common.SendMessageRequest{
		Ciphertext: ciphertext,
		Header:     header,
		Signature:  sig,
	}

	log.Printf("SEND sig nil=%v len=%d", sig == nil, len(sig))
	b, _ := json.Marshal(payload)
	log.Printf("payload json=%s", string(b))
	statusCode, messageJson, err := apiService.POST(
		"/messages/m/"+conversationId.String(),
		payload,
	)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusCreated {
		return nil, client.ErrUnknownErr
	}

	messageResponse := new(struct {
		MessageId uuid.UUID `json:"message_id"`
	})

	if err := json.Unmarshal(messageJson, messageResponse); err != nil {
		return nil, client.ErrJsonUnmarshal
	}

	// mirror
	if err := storage.AppendMessages(myUsername, conversationId, []storage.StoredMessage{
		{
			Id:         messageResponse.MessageId,
			Sender:     myUsername,
			Ciphertext: ciphertext,
			Header:     headerBytes,
			Signature:  sig,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to append message to local store: %w", err)
	}

	return &messageResponse.MessageId, nil
}

