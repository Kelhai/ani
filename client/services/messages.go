package services

import (
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
) ([]common.ShortMessage, error) {

	path := fmt.Sprintf("/messages/conversation/%s", conversationId)
	if lastMessage != nil {
		path += "/" + lastMessage.String()
	}

	statusCode, messagesJson, err := apiService.GET(path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		if statusCode == http.StatusNoContent {
			return []common.ShortMessage{}, nil
		}
		return nil, client.ErrUnknownErr
	}

	var messages []common.ShortMessage
	if err := json.Unmarshal(messagesJson, &messages); err != nil {
		return nil, client.ErrJsonUnmarshal
	}

	// 🔥 NEW: decrypt + store
	var stored []storage.StoredMessage

	for _, rm := range messages {
		peer := rm.Sender

		// load ratchet key
		ratchetKeyId, _, err := storage.FindKeyByPeer(
			currentUsername, // you need access to this
			storage.KeyTagRatchet,
			peer,
		)
		if err != nil {
			// no session yet → init from peer
			// you need your local KEM secret key here
			// skipping for now is OK for demo
			continue
		}

		state, err := storage.LoadRatchetState(currentUsername, peer, ratchetKeyId)
		if err != nil || state == nil {
			continue
		}

		// get sender identity key
		peerUser, err := AuthService{}.GetUserKeys(peer)
		if err != nil {
			continue
		}

		var senderPk mldsa87.PublicKey
		if err := senderPk.UnmarshalBinary(peerUser.IdentityPk); err != nil {
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
			continue
		}

		// save updated ratchet
		_ = storage.SaveRatchetState(currentUsername, peer, ratchetKeyId, *state)

		stored = append(stored, storage.StoredMessage{
			Id:      rm.Id,
			Sender:  rm.Sender,
			Content: string(plaintext),
		})
	}

	// save decrypted messages locally
	if len(stored) > 0 {
		_ = storage.AppendMessages(currentUsername, conversationId, stored)
	}

	// still return raw messages (UI will ignore them)
	return messages, nil
}

func (_ MessageService) SendMessage(conversationId uuid.UUID, myUsername string, recipients []string, text string) (*uuid.UUID, error) {
	if len(recipients) != 1 {
		return nil, fmt.Errorf("pairwise only supports exactly one recipient")
	}
	peer := recipients[0]

	// load identity secret key
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

	// load ratchet state
	ratchetKeyId, _, err := storage.FindKeyByPeer(myUsername, storage.KeyTagRatchet, peer)
	if err != nil {
		// no session yet
		peerUser, err := AuthService{}.GetUserKeys(peer)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch peer keys: %w", err)
		}
		state, initHeader, err := InitSession(peerUser.IdentityPk, peerUser.KemPk, peerUser.KemPkSignature)
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
	var ciphertext []byte
	var header common.RatchetHeader = common.RatchetHeader{}
	var sig []byte

	if ratchetState.InitHeader != nil {
		// first message
		header = *ratchetState.InitHeader
		ratchetState.InitHeader = nil
		newChainKey, messageKey := advanceSymmetricChain(ratchetState.SendChainKey)
		ratchetState.SendChainKey = newChainKey
		ciphertext, err = encryptMessage([]byte(text), messageKey, aad)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt: %w", err)
		}
		headerBytes, err := json.Marshal(header)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal header: %w", err)
		}
		sigPayload := append(ciphertext, headerBytes...)
		sig, err = identitySk.Sign(rand.Reader, sigPayload, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to sign: %w", err)
		}
	} else {
		ciphertext, header, sig, err = RatchetEncrypt(ratchetState, []byte(text), aad, &identitySk)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt message: %w", err)
		}
	}

	// save updated ratchet state
	if err := storage.SaveRatchetState(myUsername, peer, ratchetKeyId, *ratchetState); err != nil {
		return nil, fmt.Errorf("failed to save ratchet state: %w", err)
	}

	payload := common.SendMessageRequest{
		Ciphertext: ciphertext,
		Header:     header,
		Signature:  sig,
	}
	statusCode, messageJson, err := apiService.POST("/messages/m/"+conversationId.String(), payload)
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

	if err := storage.AppendMessages(myUsername, conversationId, []storage.StoredMessage{
		{
			Id:      messageResponse.MessageId,
			Sender:  myUsername,
			Content: text,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to append message to local store: %w", err)
	}

	return &messageResponse.MessageId, nil
}

