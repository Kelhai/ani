package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

var (
	BaseUrl    string
	serverHost string
	serverPort string
	AniHome    string
)

func SetupConfig() error {
	var found bool
	var err error
	var home string

	home, found = os.LookupEnv("ANI_HOME")
	if !found {
		home, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Failed to get homedir: %w", err)
		}
	}

	AniHome = filepath.Join(home, ".ani")

	err = godotenv.Load(filepath.Join(AniHome, ".env"))
	if err != nil {
		return fmt.Errorf("Failed to load ~/.ani/.env: %w", err)
	}

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

