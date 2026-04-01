package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	POSTGRES_HOST string
	POSTGRES_USER string
	POSTGRES_PASSWORD string
	POSTGRES_PORT int
	POSTGRES_DB string
	
	SERVER_PORT int
)

func SetupConfig() error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("Failed to load .env: %w", err)
	}

	var found bool

	POSTGRES_HOST, found = os.LookupEnv("POSTGRES_HOST")
	if !found {
		log.Println("$POSTGRES_HOST empty, defaulting to localhost")
		POSTGRES_HOST = "localhost"
	}

	POSTGRES_USER, found = os.LookupEnv("POSTGRES_USER")
	if !found {
		log.Println("$POSTGRES_USER empty, defaulting to 'postgres'")
		POSTGRES_USER = "postgres"
	}

	POSTGRES_PASSWORD, found = os.LookupEnv("POSTGRES_PASSWORD")
	if !found {
		log.Fatal("$POSTGRES_PASSWORD empty, defaulting to 'postgres'")
		return errors.New("$POSTGRES_PASSWORD empty")
	}

	pgPortStr, found := os.LookupEnv("POSTGRES_PORT")
	if !found {
		log.Println("$POSTGRES_PORT empty, defaulting to 5432")
		POSTGRES_PORT = 5432
	} else {
		POSTGRES_PORT, err = strconv.Atoi(pgPortStr)
		if err != nil {
			log.Fatal("$POSTGRES_PORT must be an int")
			return errors.New("$POSTGRES_PORT must be an int")
		}
	}

	POSTGRES_DB, found = os.LookupEnv("POSTGRES_DB")
	if !found {
		log.Println("$POSTGRES_DB empty, defaulting to 'postgres'")
		POSTGRES_DB = "postgres"
	}

	serverPortStr, found := os.LookupEnv("SERVER_PORT")
	if !found {
		log.Println("$SERVER_PORT empty, defaulting to 52971")
		SERVER_PORT = 52971
	} else {
		SERVER_PORT, err = strconv.Atoi(serverPortStr)
		if err != nil {
			log.Fatal("$SERVER_PORT must be an int")
			return errors.New("$SERVER_PORT must be an int")
		}
	}

	return nil
}

