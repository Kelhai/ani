package services

import (
	"github.com/Kelhai/ani/storage"
)

var (
	pgStorage storage.PgStorage
)

func SetupStorages() error {
	pgStorage = storage.SetupPgStorage()

	return nil
}

