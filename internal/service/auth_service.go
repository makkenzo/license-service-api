package service

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/makkenzo/license-service-api/internal/config"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"go.uber.org/zap"
)

type ZitadelClaims struct {
	Email             string                            `json:"email"`
	EmailVerified     bool                              `json:"email_verified"`
	PreferredUsername string                            `json:"preferred_username"`
	Name              string                            `json:"name"`
	GivenName         string                            `json:"given_name"`
	FamilyName        string                            `json:"family_name"`
	Locale            string                            `json:"locale"`
	Roles             map[string]map[string]interface{} `json:"urn:zitadel:iam:org:project:id:317234470941884420:roles"`
	Scope             string                            `json:"scope"`
	ClientID          string                            `json:"client_id"`
	Audience          []string                          `json:"aud"`
	Subject           string                            `json:"sub"`
}

type AuthService struct {
	keySet   oidc.KeySet
	config   *config.OIDCConfig
	logger   *zap.Logger
	issuer   string
	clientID string
}

func NewAuthService(ctx context.Context, cfg *config.OIDCConfig, logger *zap.Logger) (*AuthService, error) {
	log := logger.Named("AuthService")
	if cfg.IssuerURL == "" || cfg.ClientID == "" {
		return nil, fmt.Errorf("OIDC IssuerURL and ClientID are required")
	}

	log.Info("Initializing OIDC provider", zap.String("issuer", cfg.IssuerURL))
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		log.Error("Failed to create OIDC provider", zap.String("issuer", cfg.IssuerURL), zap.Error(err))
		return nil, fmt.Errorf("oidc provider setup failed: %w", err)
	}

	var discoveryClaims struct {
		JWKSURI string `json:"jwks_uri"`
		Issuer  string `json:"issuer"`
	}
	if err := provider.Claims(&discoveryClaims); err != nil {
		log.Error("Failed to get discovery claims", zap.Error(err))
		return nil, fmt.Errorf("failed to get OIDC discovery claims: %w", err)
	}

	log.Info("Creating OIDC keyset from JWKS URI", zap.String("jwks_uri", discoveryClaims.JWKSURI))
	keySet := oidc.NewRemoteKeySet(ctx, discoveryClaims.JWKSURI)

	return &AuthService{
		keySet:   keySet,
		config:   cfg,
		logger:   log,
		issuer:   discoveryClaims.Issuer,
		clientID: cfg.ClientID,
	}, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, rawToken string) (*ZitadelClaims, error) {
	s.logger.Debug("Attempting to validate OIDC Access Token (JWT) using Verifier")

	verifier := oidc.NewVerifier(s.issuer, s.keySet, &oidc.Config{
		ClientID: s.clientID,
	})

	token, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		s.logger.Warn("Failed to verify access token JWT", zap.Error(err))
		return nil, fmt.Errorf("%w: %v", ierr.ErrInvalidToken, err)
	}

	var claims ZitadelClaims
	if err := token.Claims(&claims); err != nil {
		s.logger.Error("Failed to extract claims from access token", zap.Error(err))
		return nil, fmt.Errorf("%w: could not unmarshal access token claims: %v", ierr.ErrTokenInvalidClaims, err)
	}

	claims.Subject = token.Subject

	s.logger.Info("Access Token validated successfully", zap.String("subject", claims.Subject), zap.String("client_id_in_token", claims.ClientID), zap.String("scope", claims.Scope))
	return &claims, nil
}
