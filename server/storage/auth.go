package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Session struct {
	bun.BaseModel `bun:"table:sessions"`

	Id        uuid.UUID `bun:"id,pk,type:uuid"`
	UserId    uuid.UUID `bun:"user_id,type:uuid,notnull"`
	ExpiresAt time.Time `bun:"expires_at,notnull"`
}

func createAuthSchema(db *bun.DB) {
	ctx := context.Background()

	if _, err := db.NewCreateTable().Model((*common.User)(nil)).IfNotExists().Exec(ctx); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.NewCreateTable().Model((*Session)(nil)).IfNotExists().Exec(ctx); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}
}

func (pgs PgStorage) GetUserByUsername(username string) (*common.User, error) {
	user := new(common.User)
	err := pgs.db.NewSelect().
		Model(user).
		Where("username = ?", username).
		Scan(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Failed to get user: %w", err)
	}
	return user, nil
}

func (pgs PgStorage) GetUserIdsByUsernames(usernames []string) (map[string]uuid.UUID, error) {
	if len(usernames) == 0 {
		return map[string]uuid.UUID{}, nil
	}

	var users []common.User
	err := pgs.db.NewSelect().
		Model(&users).
		Where("username IN (?)", bun.List(usernames)).
		Scan(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	result := make(map[string]uuid.UUID, len(users))
	for _, u := range users {
		result[u.Username] = u.Id
	}

	return result, nil
}

func (pgs PgStorage) GetUserIdByUsername(username string) (*uuid.UUID, error) {
	user := new(common.User)
	err := pgs.db.NewSelect().
		Model(user).
		Where("username = ?", username).
		Scan(context.Background())
	if err != nil {
		log.Printf("Failed to get user: %s", err.Error())
		return nil, fmt.Errorf("Failed to get user: %w", err)
	}

	return &user.Id, nil
}

func (pgs PgStorage) GetUsersByIds(userIds []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(userIds) == 0 {
		return map[uuid.UUID]string{}, nil
	}

	var users []common.User
	err := pgs.db.NewSelect().
		Model(&users).
		Where("id IN (?)", bun.List(userIds)).
		Scan(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	result := make(map[uuid.UUID]string, len(users))
	for _, u := range users {
		result[u.Id] = u.Username
	}

	return result, nil
}

func (pgs PgStorage) AddUser(user common.User) error {
	_, err := pgs.db.NewInsert().Model(&user).Exec(context.Background())
	if err != nil {
		log.Printf("Failed to insert user: %s", err.Error())
		return fmt.Errorf("Failed to insert user: %w: %w", common.ErrPgInsertFailed, err)
	}
	return nil
}

func (pgs PgStorage) GetSessionByToken(token uuid.UUID) (*common.Session, error) {
	session := &Session{}
	err := pgs.db.NewSelect().
		Model(session).
		Where("id = ?", token).
		Scan(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &common.Session{
		Id:        session.Id,
		UserId:    session.UserId,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (pgs PgStorage) GetSessionByUser(userId uuid.UUID) (*common.Session, error) {
	session := &Session{}
	err := pgs.db.NewSelect().
		Model(session).
		Where("user_id = ?", userId).
		Scan(context.Background())
	if err != nil {
		return nil, common.ErrNotFound
	}
	return &common.Session{
		Id:        session.Id,
		UserId:    session.UserId,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (pgs PgStorage) NewSession(userId uuid.UUID) (*common.Session, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, common.ErrUuidFailed
	}

	_, err = pgs.db.NewDelete().
		TableExpr("sessions").
		Where("user_id = ?", userId).
		Exec(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to delete old sessions: %w", err)
	}

	pgSession := Session{
		Id:        id,
		UserId:    userId,
		ExpiresAt: time.Now().Add(8 * time.Hour),
	}
	_, err = pgs.db.NewInsert().Model(&pgSession).Exec(context.Background())
	if err != nil {
		log.Printf("Failed to insert new session: %s", err.Error())
		return nil, fmt.Errorf("Failed to insert session: %w: %w", common.ErrPgInsertFailed, err)
	}
	return &common.Session{
		Id:        pgSession.Id,
		UserId:    userId,
		ExpiresAt: pgSession.ExpiresAt,
	}, nil
}

func (pgs PgStorage) GetUser(userId uuid.UUID) (*common.User, error) {
	user := new(common.User)
	err := pgs.db.NewSelect().
		Model(&user).
		Where("id = ?", userId).
		Scan(context.Background())
	if err != nil {
		return nil, common.ErrNotFound
	}

	return user, nil
}
