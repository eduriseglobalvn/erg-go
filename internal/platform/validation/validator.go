package validation

import (
	"errors"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var ErrInvalidReferenceID = errors.New("invalid reference id")

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func FieldErrors(err error) []FieldError {
	var out []FieldError
	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, item := range errs {
			out = append(out, FieldError{
				Field:   item.Field(),
				Code:    item.Tag(),
				Message: messageFor(item),
			})
		}
	}
	return out
}

func ClampLimit(value, fallback, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func NormalizeReferenceID(value string) string {
	return strings.TrimSpace(value)
}

func ValidateReferenceID(value string) error {
	value = NormalizeReferenceID(value)
	if value == "" || len(value) > 128 {
		return ErrInvalidReferenceID
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '-', '_', '.', ':':
			continue
		default:
			return ErrInvalidReferenceID
		}
	}
	return nil
}

func ValidateReferenceIDs(values []string, max int) error {
	if max > 0 && len(values) > max {
		return ErrInvalidReferenceID
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = NormalizeReferenceID(value)
		if err := ValidateReferenceID(value); err != nil {
			return err
		}
		if _, ok := seen[value]; ok {
			return ErrInvalidReferenceID
		}
		seen[value] = struct{}{}
	}
	return nil
}

func messageFor(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return err.Field() + " is required"
	case "email":
		return err.Field() + " must be a valid email address"
	case "min":
		return err.Field() + " is too short"
	case "max":
		return err.Field() + " is too long"
	default:
		return err.Field() + " failed validation: " + err.Tag()
	}
}
