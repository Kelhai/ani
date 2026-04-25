package services

import (
	"encoding/base64"
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
	statusCode, body, err := apiService.GET("/messages/conversations", nil)
	if err != nil {
		return nil, err
	}
	if statusCode == http.StatusNoContent {
		return []common.ConversationWithUsernames{}, nil
	}
	if statusCode != http.StatusOK {
		return nil, client.ErrUnknownErr
	}
	var convs []common.ConversationWithUsernames
	if err := json.Unmarshal(body, &convs); err != nil {
		return nil, client.ErrJsonUnmarshal
	}
	return convs, nil
}

func (_ MessageService) CreateConversation(usernames []string) (*uuid.UUID, error) {
	statusCode, body, err := apiService.POST("/messages/conversation", map[string][]string{
		"usernames": usernames,
	})
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusCreated {
		return nil, client.ErrUnknownErr
	}
	var resp struct {
		ConversationId uuid.UUID `json:"conversationId"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, client.ErrJsonUnmarshal
	}
	return &resp.ConversationId, nil
}

// GetMessages fetches messages since lastMessage (nil = all), decrypts them,
// and returns plaintext DecryptedMessages. myUsername is the local user.
func (_ MessageService) GetMessages(
	conversationId uuid.UUID,
	myUsername string,
	lastMessage *uuid.UUID,
) ([]common.DecryptedMessage, error) {

	path := fmt.Sprintf("/messages/conversation/%s", conversationId)
	if lastMessage != nil {
		path += "/" + lastMessage.String()
	}

	statusCode, body, err := apiService.GET(path, nil)
	if err != nil {
		return nil, err
	}
	if statusCode == http.StatusNoContent {
		return []common.DecryptedMessage{}, nil
	}
	if statusCode != http.StatusOK {
		return nil, client.ErrUnknownErr
	}

	var raw []common.ShortMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, client.ErrJsonUnmarshal
	}

	kemKeyId, _, err := storage.FindKeyByTag(myUsername, storage.KeyTagKem)
	if err != nil {
		return nil, fmt.Errorf("failed to find our KEM key: %w", err)
	}
	ourKemSk, err := storage.LoadPrivKey(myUsername, kemKeyId)
	if err != nil {
		return nil, fmt.Errorf("failed to load our KEM secret key: %w", err)
	}

	var out []common.DecryptedMessage

	for _, msg := range raw {
		if msg.Sender == myUsername {
			out = append(out, common.DecryptedMessage{
				Id:      msg.Id,
				Sender:  msg.Sender,
				Content: "(sent)",
			})
			continue
		}

		env, err := unpackEnvelope(msg.Content)
		if err != nil {
			log.Printf("failed to unpack envelope from %s: %v", msg.Sender, err)
			continue
		}

		state, ratchetKeyId, justInited, err := getOrInitRecvState(
			myUsername, msg.Sender, ourKemSk, env.Header,
		)
		if err != nil {
			log.Printf("failed to get ratchet state for %s: %v", msg.Sender, err)
			continue
		}

		senderBundle, err := AuthService{}.GetUserBundle(msg.Sender)
		if err != nil {
			log.Printf("failed to fetch bundle for %s: %v", msg.Sender, err)
			continue
		}
		var senderPk mldsa87.PublicKey
		if err := senderPk.UnmarshalBinary(senderBundle.IdentityPk); err != nil {
			log.Printf("failed to unmarshal sender pk: %v", err)
			continue
		}

		decryptHdr := env.Header
		if justInited {
			decryptHdr.KemCiphertext = nil
		}

		plaintext, err := RatchetDecrypt(
			state,
			env.Ciphertext,
			env.Header,
			decryptHdr,
			env.Signature,
			conversationId[:],
			&senderPk,
		)
		if err != nil {
			log.Printf("decryption failed for msg %s: %v", msg.Id, err)
			continue
		}

		if err := storage.SaveRatchetState(myUsername, msg.Sender, ratchetKeyId, *state); err != nil {
			log.Printf("failed to save ratchet state: %v", err)
		}

		out = append(out, common.DecryptedMessage{
			Id:      msg.Id,
			Sender:  msg.Sender,
			Content: string(plaintext),
		})
	}

	return out, nil
}

// SendMessage encrypts text and sends it to conversationId.
// peer is the single recipient username (pairwise only).
func (_ MessageService) SendMessage(
	conversationId uuid.UUID,
	myUsername, peer, text string,
) (*uuid.UUID, error) {

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

	state, ratchetKeyId, hdr, err := getOrInitSendState(myUsername, peer)
	if err != nil {
		return nil, fmt.Errorf("failed to get ratchet state: %w", err)
	}

	aad := conversationId[:]

	var (
		ciphertext []byte
		header     common.RatchetHeader
		sig        []byte
	)

	if hdr != nil {
		header = *hdr

		newChain, msgKey := advanceChain(state.SendChainKey)
		state.SendChainKey = newChain

		ciphertext, err = sealMessage([]byte(text), msgKey, aad)
		if err != nil {
			return nil, fmt.Errorf("encrypt failed: %w", err)
		}

		hdrBytes, _ := marshalHeader(header)
		sigPayload := append(ciphertext, hdrBytes...)
		sig, err = identitySk.Sign(nil, sigPayload, nil)
		if err != nil {
			return nil, fmt.Errorf("sign failed: %w", err)
		}
	} else {
		ciphertext, header, sig, err = RatchetEncrypt(state, []byte(text), aad, &identitySk)
		if err != nil {
			return nil, fmt.Errorf("ratchet encrypt failed: %w", err)
		}
	}

	if err := storage.SaveRatchetState(myUsername, peer, ratchetKeyId, *state); err != nil {
		return nil, fmt.Errorf("failed to save ratchet state: %w", err)
	}

	content, err := packEnvelope(common.RatchetEnvelope{
		Ciphertext: ciphertext,
		Header:     header,
		Signature:  sig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to pack envelope: %w", err)
	}

	statusCode, respBody, err := apiService.POST(
		"/messages/m/"+conversationId.String(),
		map[string]string{"message": content},
	)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusCreated {
		return nil, client.ErrUnknownErr
	}

	var msgResp struct {
		MessageId uuid.UUID `json:"message_id"`
	}
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return nil, client.ErrJsonUnmarshal
	}

	return &msgResp.MessageId, nil
}

// getOrInitSendState returns the ratchet state for sending to peer. If no
// state exists, it initialises a new session as sender. hdr is non-nil only
// for the very first message (carries the KEM init header).
func getOrInitSendState(
	myUsername, peer string,
) (state *storage.RatchetState, keyId uuid.UUID, initHdr *common.RatchetHeader, err error) {

	keyId, _, err = storage.FindKeyByPeer(myUsername, storage.KeyTagRatchet, peer)
	if err == nil {
		state, err = storage.LoadRatchetState(myUsername, peer, keyId)
		if err != nil {
			return
		}
		return
	}

	peerBundle, e := AuthService{}.GetUserBundle(peer)
	if e != nil {
		err = fmt.Errorf("failed to fetch peer bundle: %w", e)
		return
	}

	state, initHdr, err = InitSessionAsSender(
		peerBundle.IdentityPk,
		peerBundle.KemPk,
		peerBundle.KemPkSignature,
	)
	if err != nil {
		return
	}

	keyId, err = uuid.NewV7()
	if err != nil {
		return
	}
	if err = storage.SaveRatchetState(myUsername, peer, keyId, *state); err != nil {
		return
	}
	err = storage.AddLegendEntry(myUsername, keyId, storage.LegendEntry{
		Tag:     storage.KeyTagRatchet,
		Type:    peer,
		Created: time.Now(),
	})
	return
}

// getOrInitRecvState returns the ratchet state for receiving from peer. If no
// state exists, it initialises a new session as receiver using the incoming
// message header.
func getOrInitRecvState(
	myUsername, peer string,
	ourKemSk []byte,
	hdr common.RatchetHeader,
) (*storage.RatchetState, uuid.UUID, bool, error) {

	keyId, _, err := storage.FindKeyByPeer(myUsername, storage.KeyTagRatchet, peer)
	if err == nil {
		state, err := storage.LoadRatchetState(myUsername, peer, keyId)
		return state, keyId, false, err
	}

	state, err := InitSessionAsReceiver(ourKemSk, hdr)
	if err != nil {
		return nil, uuid.Nil, false, fmt.Errorf("failed to init session as receiver: %w", err)
	}

	keyId, err = uuid.NewV7()
	if err != nil {
		return nil, uuid.Nil, false, err
	}
	if err := storage.SaveRatchetState(myUsername, peer, keyId, *state); err != nil {
		return nil, uuid.Nil, false, err
	}
	if err := storage.AddLegendEntry(myUsername, keyId, storage.LegendEntry{
		Tag:     storage.KeyTagRatchet,
		Type:    peer,
		Created: time.Now(),
	}); err != nil {
		return nil, uuid.Nil, false, err
	}

	return state, keyId, true, nil
}

func packEnvelope(env common.RatchetEnvelope) (string, error) {
	b, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func unpackEnvelope(content string) (*common.RatchetEnvelope, error) {
	b, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}
	var env common.RatchetEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}
	return &env, nil
}
