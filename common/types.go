package common

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Auth

type User struct {
	bun.BaseModel  `bun:"table:users" json:"-"`

	Id             uuid.UUID `bun:"id,pk,type:uuid" json:"id"`
	Username       string    `bun:"username,unique,notnull" json:"username"`
	IdentityPk     []byte    `bun:"identity_pk,notnull" json:"identity_pk"`
	KemPk          []byte    `bun:"kem_pk,notnull" json:"kem_pk"`
	KemPkSignature []byte    `bun:"kem_pk_sig,notnull" json:"kem_pk_sig"`
}

type RegisterRequest struct {
	Username       string `json:"username"`
	IdentityPk     []byte `json:"identity_pk"`
	KemPk          []byte `json:"kem_pk"`
	KemPkSignature []byte `json:"kem_pk_sig"`
}

type AuthBlob struct {
	SignatureAlgorithm string    `json:"s_type"`
	Username           string    `json:"username"`
	SignedTime         time.Time `json:"s_time"`
	TimeToLive         time.Time `json:"ttl"`
	Uuid               uuid.UUID `json:"uuid"`
}

type AuthEnvelope struct {
	Blob      AuthBlob `json:"blob"`
	Signature []byte   `json:"signature"`
}

type Session struct {
	Id        uuid.UUID `json:"id"`
	UserId    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Conversations
type Conversation struct {
	Id      uuid.UUID   `json:"id"`
	Members []uuid.UUID `json:"members"`
}

type ConversationWithUsernames struct {
	Id      uuid.UUID `json:"id"`
	Members []string  `json:"members"`
}

type ShortMessage struct {
	Id      uuid.UUID `json:"id"`
	Sender  string    `json:"sender"`
	Content string    `json:"content"`
}

type RatchetEnvelope struct {
	Ciphertext []byte        `json:"ct"`
	Header     RatchetHeader `json:"hdr"`
	Signature  []byte        `json:"sig"`
}

type RatchetHeader struct {
	KemCiphertext []byte `json:"kem_ct,omitempty"`
	SenderKemPk   []byte `json:"kem_pk,omitempty"`
}

type DecryptedMessage struct {
	Id      uuid.UUID
	Sender  string
	Content string
}
