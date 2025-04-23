package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/makkenzo/license-service-api/internal/domain/apikey"
	apikeyRepoImpl "github.com/makkenzo/license-service-api/internal/storage/postgres"
	"github.com/makkenzo/license-service-api/internal/util"
	"go.uber.org/zap"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	fullKey, prefix, keyHash, err := util.GenerateAPIKey()
	if err != nil {
		log.Fatalf("Failed to generate API key: %v", err)
	}

	fmt.Printf("Generated API Key (SAVE THIS securely!):\n%s\n\n", fullKey)
	fmt.Printf("Prefix: %s\n", prefix)
	fmt.Printf("Key Hash: %s\n", keyHash)

	logger, _ := zap.NewDevelopment()
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	repo := apikeyRepoImpl.NewAPIKeyRepository(pool, logger)

	newKeyRecord := &apikey.APIKey{
		KeyHash:     keyHash,
		Prefix:      prefix,
		Description: "Default Agent Key for Product AwesomeApp",

		IsEnabled: true,
	}

	keyID, err := repo.Create(context.Background(), newKeyRecord)
	if err != nil {
		log.Fatalf("Failed to save API key to database: %v", err)
	}

	fmt.Printf("\nAPI Key saved to database with ID: %s\n", keyID)
}
