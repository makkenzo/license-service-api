package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/makkenzo/license-service-api/internal/config"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"github.com/makkenzo/license-service-api/internal/storage/memstorage"
	"go.uber.org/zap"
)

type AuthService struct {
	userRepo  *memstorage.UserRepositoryMock
	jwtConfig *config.JWTConfig
	logger    *zap.Logger
}

func NewAuthService(userRepo *memstorage.UserRepositoryMock, jwtConfig *config.JWTConfig, logger *zap.Logger) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtConfig: jwtConfig,
		logger:    logger.Named("AuthService"),
	}
}

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

func (s *AuthService) Login(ctx context.Context, username, password string) (string, error) {
	s.logger.Info("Attempting login", zap.String("username", username))

	u, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ierr.ErrUserNotFound) {
			s.logger.Warn("Login failed: user not found", zap.String("username", username))
			return "", ierr.ErrInvalidCredentials
		}
		s.logger.Error("Error finding user during login", zap.String("username", username), zap.Error(err))
		return "", fmt.Errorf("internal error during login: %w", err)
	}

	if !memstorage.CheckPassword(u.PasswordHash, password) {
		s.logger.Warn("Login failed: invalid password", zap.String("username", username))
		return "", ierr.ErrInvalidCredentials
	}

	expirationTime := time.Now().Add(s.jwtConfig.TokenTTL)
	claims := &Claims{
		UserID: u.ID,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "license-service",
			Subject:   u.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(s.jwtConfig.SecretKey))
	if err != nil {
		s.logger.Error("Failed to sign JWT token", zap.Error(err))
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	s.logger.Info("Login successful, token generated", zap.String("username", username), zap.String("user_id", u.ID.String()))
	return tokenString, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {

	keyFunc := func(token *jwt.Token) (interface{}, error) {

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			s.logger.Warn("Unexpected signing method in token", zap.Any("alg", token.Header["alg"]))
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(s.jwtConfig.SecretKey), nil
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, keyFunc)
	if err != nil {
		s.logger.Warn("Failed to parse JWT token", zap.Error(err))
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ierr.ErrInvalidToken
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ierr.ErrInvalidToken
		}
		return nil, ierr.ErrTokenParsingFailed
	}

	if !token.Valid {
		s.logger.Warn("Invalid token received")
		return nil, ierr.ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		s.logger.Error("Token claims are not of expected type")
		return nil, ierr.ErrTokenInvalidClaims
	}

	if claims == nil {
		s.logger.Error("Token claims are nil after parsing")
		return nil, ierr.ErrTokenNoClaims
	}

	s.logger.Debug("Token validated successfully", zap.String("user_id", claims.UserID.String()))
	return claims, nil
}
