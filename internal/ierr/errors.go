package ierr

import "errors"

var (
	ErrValidation     = errors.New("validation failed")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrUpdateFailed   = errors.New("resource update failed")
	ErrNotFound       = errors.New("resource not found")
	ErrConflict       = errors.New("resource conflict")
	ErrInternalServer = errors.New("internal server error")

	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrTokenParsingFailed = errors.New("failed to parse token")
	ErrTokenNoClaims      = errors.New("token contains no claims")
	ErrTokenInvalidClaims = errors.New("token contains invalid claims type")
	ErrAPIKeyNotFound     = errors.New("api key not found or disabled")

	ErrAPIKeyUpdateFailed = errors.New("api key update failed")
)
