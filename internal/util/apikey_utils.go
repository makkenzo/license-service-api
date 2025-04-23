package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/makkenzo/license-service-api/internal/domain/apikey"
)

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func generateRandomString(length int) (string, error) {
	byteLength := (length*3 + 3) / 4
	b, err := generateRandomBytes(byteLength)
	if err != nil {
		return "", err
	}

	str := base64.URLEncoding.EncodeToString(b)
	str = strings.ReplaceAll(str, "-", "")
	str = strings.ReplaceAll(str, "_", "")
	if len(str) > length {
		return str[:length], nil
	}

	return str, nil
}

func GenerateAPIKey() (fullKey string, prefix string, keyHash string, err error) {
	prefix, err = generateRandomString(apikey.APIKeyPrefixLength)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate prefix: %w", err)
	}

	secret, err := generateRandomString(apikey.APIKeySecretLength)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate secret: %w", err)
	}

	fullKey = fmt.Sprintf(apikey.APIKeyFormat, prefix, secret)

	hashBytes := sha256.Sum256([]byte(fullKey))
	keyHash = fmt.Sprintf("%x", hashBytes)

	return fullKey, prefix, keyHash, nil
}

func HashAPIKey(fullKey string) string {
	hashBytes := sha256.Sum256([]byte(fullKey))
	return fmt.Sprintf("%x", hashBytes)
}
