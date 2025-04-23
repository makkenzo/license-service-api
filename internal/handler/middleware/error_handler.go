package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/makkenzo/license-service-api/internal/handler/dto"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"go.uber.org/zap"
)

func ErrorHandlerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	log := logger.Named("ErrorHandler")
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err
		log.Error("Request failed", zap.Error(err))

		status := http.StatusInternalServerError
		errResponse := dto.APIErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "An unexpected error occurred.",
		}

		var ve validator.ValidationErrors

		if errors.As(err, &ve) {
			status = http.StatusBadRequest
			errResponse.Code = "VALIDATION_ERROR"
			errResponse.Message = "Input validation failed."
			errResponse.Details = buildValidationErrors(ve)
		} else {
			switch {
			case errors.Is(err, ierr.ErrValidation):
				status = http.StatusBadRequest
				errResponse.Code = "VALIDATION_ERROR"
				errResponse.Message = err.Error()
			case errors.Is(err, ierr.ErrUnauthorized), errors.Is(err, ierr.ErrInvalidCredentials), errors.Is(err, ierr.ErrInvalidToken):
				status = http.StatusUnauthorized
				errResponse.Code = "UNAUTHENTICATED"
				errResponse.Message = "Authentication required or failed."

			case errors.Is(err, ierr.ErrForbidden):
				status = http.StatusForbidden
				errResponse.Code = "FORBIDDEN"
				errResponse.Message = "Access denied."
			case errors.Is(err, ierr.ErrNotFound), errors.Is(err, ierr.ErrUserNotFound):
				status = http.StatusNotFound
				errResponse.Code = "NOT_FOUND"
				errResponse.Message = "The requested resource was not found."
			case errors.Is(err, ierr.ErrConflict):
				status = http.StatusConflict
				errResponse.Code = "CONFLICT"
				errResponse.Message = err.Error()
			default:
				errResponse.Message = err.Error()
			}
		}

		c.AbortWithStatusJSON(status, errResponse)
	}
}

func buildValidationErrors(ve validator.ValidationErrors) []dto.FieldError {
	details := make([]dto.FieldError, len(ve))
	for i, fe := range ve {
		details[i] = dto.FieldError{
			Field:   fe.Field(),
			Message: getValidationErrorMsg(fe),
		}
	}
	return details
}

func getValidationErrorMsg(fe validator.FieldError) string {

	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("Field '%s' is required", fe.Field())
	case "email":
		return fmt.Sprintf("Field '%s' must be a valid email address", fe.Field())
	case "oneof":
		return fmt.Sprintf("Field '%s' must be one of [%s]", fe.Field(), fe.Param())
	case "gte":
		return fmt.Sprintf("Field '%s' must be greater than or equal to %s", fe.Field(), fe.Param())
	case "lte":
		return fmt.Sprintf("Field '%s' must be less than or equal to %s", fe.Field(), fe.Param())
	case "gt":
		return fmt.Sprintf("Field '%s' must be greater than %s", fe.Field(), fe.Param())
	default:
		return fmt.Sprintf("Field '%s' failed validation on the '%s' tag", fe.Field(), fe.Tag())
	}
}
