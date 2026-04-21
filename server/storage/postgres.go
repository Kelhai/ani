package storage

import (
	"database/sql"
	"fmt"

	"github.com/Kelhai/ani/server/config"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type PgStorage struct {
	db *bun.DB
}

func SetupPgStorage() PgStorage {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.POSTGRES_USER,
		config.POSTGRES_PASSWORD,
		config.POSTGRES_HOST,
		config.POSTGRES_PORT,
		config.POSTGRES_DB,
	)

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db := bun.NewDB(sqldb, pgdialect.New())

	createAuthSchema(db)
	createMessageSchema(db)

	return PgStorage{db: db}
}
