package utils

import (
	"example/hello/internal/domain"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func ValidateStruct(s interface{}) error {
	err := validate.Struct(s)
	if err != nil {
		// Here, we transform the validator error into our domain.AppError
		return &domain.AppError{
			Err:     err,
			Message: "Validation failed: " + formatValidationError(err),
			Code:    http.StatusBadRequest, // 400
		}
	}
	return nil
}

// Helper to make error messages human-readable
func formatValidationError(err error) string {
	var errMsgs []string
	for _, err := range err.(validator.ValidationErrors) {
		errMsgs = append(errMsgs, fmt.Sprintf("Field '%s' failed on '%s' condition", err.Field(), err.Tag()))
	}
	return strings.Join(errMsgs, ", ")
}
