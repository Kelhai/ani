package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type User struct {
    bun.BaseModel `bun:"table:users"`

    Id           uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
    Username     string    `bun:"username,unique,notnull"`
    PasswordHash []byte    `bun:"password_hash,type:bytea,notnull"`
}

type Session struct {
    bun.BaseModel `bun:"table:sessions"`

    Id        uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
    UserId    uuid.UUID `bun:"user_id,type:uuid,notnull"`
    Token     string    `bun:"token,unique,notnull"`
    ExpiresAt time.Time `bun:"expires_at,notnull"`
}

func createAuthSchema(db *bun.DB) {
	ctx := context.Background()

	// could do some error check
	db.NewCreateTable().Model((*User)(nil)).IfNotExists().Exec(ctx)
	db.NewCreateTable().Model((*Session)(nil)).IfNotExists().Exec(ctx)
}

func (pgs PgStorage) GetUserByUsername(username string) (*common.User, error) {
	user := User{}
	err := pgs.db.NewSelect().
		Model(user).
		Where("username = ?", username).
		Scan(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Failed to get user: %w", err)
	}
	return &common.User{
		Id: user.Id,
		Username: user.Username,
		PasswordHash: user.PasswordHash,
	}, nil
}

func (pgs PgStorage) GetSessionByToken(token string) (*common.Session, error) {
	session := &Session{}
	err := pgs.db.NewSelect().
		Model(session).
		Where("token = ?", token).
		Scan(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &common.Session{
		UserId: session.Id,
		Token: session.Token,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

