package storage

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Message struct {
	bun.BaseModel `bun:"table:messages"`

	Id uuid.UUID `bun:"id,pk,type:uuid,default:"`
}
