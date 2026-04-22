package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	BaseUrl    string
	serverHost string
	serverPort string
)

func SetupConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Failed to get homedir: %w", err)
	}

	err = godotenv.Load(home + "/.ani/.env")
	if err != nil {
		return fmt.Errorf("Failed to load ~/.ani/.env: %w", err)
	}

	var found bool

	serverHost, found = os.LookupEnv("SERVER_HOST")
	if !found {
		log.Println("$SERVER_HOST empty, defaulting to localhost")
		serverHost = "localhost"
	}

	serverPort, found = os.LookupEnv("SERVER_PORT")
	if !found {
		log.Println("$SERVER_PORT empty, defaulting 52971")
		serverPort = "52971"
	}

	BaseUrl = fmt.Sprintf("https://%s:%s", serverHost, serverPort)

	return nil
}

