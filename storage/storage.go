package storage

var (
	pgStorage PgStorage
)

func SetupAllStorage() error {
	var err error
	pgStorage, err = SetupPgStorage()
	if err != nil {
		return err
	}

	return nil
}

