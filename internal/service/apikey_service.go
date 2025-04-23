package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/makkenzo/license-service-api/internal/domain/apikey"
	"github.com/makkenzo/license-service-api/internal/handler/dto"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"github.com/makkenzo/license-service-api/internal/util"
	"go.uber.org/zap"
)

type APIKeyService struct {
	repo   apikey.Repository
	logger *zap.Logger
}

func NewAPIKeyService(repo apikey.Repository, logger *zap.Logger) *APIKeyService {
	return &APIKeyService{
		repo:   repo,
		logger: logger.Named("APIKeyService"),
	}
}

func (s *APIKeyService) CreateAPIKey(ctx context.Context, description string, productID *uuid.UUID) (*dto.CreateAPIKeyResponse, string, error) {
	s.logger.Info("Generating new API key", zap.String("description", description))

	fullKey, prefix, keyHash, err := util.GenerateAPIKey()
	if err != nil {
		s.logger.Error("Failed to generate api key components", zap.Error(err))
		return nil, "", fmt.Errorf("%w: failed generating key: %v", ierr.ErrInternalServer, err)
	}

	newKey := &apikey.APIKey{
		KeyHash:     keyHash,
		Prefix:      prefix,
		Description: description,
		IsEnabled:   true,
	}
	if productID != nil {
		newKey.ProductID = *productID
	}

	insertedID, err := s.repo.Create(ctx, newKey)
	if err != nil {

		s.logger.Error("Failed to save new api key", zap.Error(err))

		return nil, "", fmt.Errorf("repository error creating api key: %w", err)
	}

	resp := &dto.CreateAPIKeyResponse{
		ID:          insertedID,
		FullKey:     fullKey,
		Prefix:      prefix,
		Description: description,
	}
	if productID != nil {
		resp.ProductID = *productID
	}

	s.logger.Info("API key created successfully", zap.String("id", insertedID.String()), zap.String("prefix", prefix))

	return resp, fullKey, nil
}

func (s *APIKeyService) ListAPIKeys(ctx context.Context) ([]*dto.APIKeyResponse, error) {
	s.logger.Debug("Listing API keys")
	keys, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("Failed to list api keys from repository", zap.Error(err))
		return nil, fmt.Errorf("repository error listing api keys: %w", err)
	}

	responses := make([]*dto.APIKeyResponse, len(keys))
	for i, key := range keys {
		responses[i] = &dto.APIKeyResponse{
			ID:          key.ID,
			Prefix:      key.Prefix,
			Description: key.Description,
			ProductID:   key.ProductID,
			IsEnabled:   key.IsEnabled,
			CreatedAt:   key.CreatedAt,
			LastUsedAt:  key.LastUsedAt,
		}
	}
	s.logger.Info("API keys listed successfully", zap.Int("count", len(responses)))
	return responses, nil
}

func (s *APIKeyService) RevokeAPIKey(ctx context.Context, id uuid.UUID) error {
	s.logger.Info("Attempting to revoke API key", zap.String("id", id.String()))
	err := s.repo.Disable(ctx, id)
	if err != nil {

		s.logger.Error("Failed to revoke api key via repository", zap.String("id", id.String()), zap.Error(err))

		return fmt.Errorf("repository error revoking api key %s: %w", id, err)
	}
	s.logger.Info("API key revoked successfully", zap.String("id", id.String()))
	return nil
}
