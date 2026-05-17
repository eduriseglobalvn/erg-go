package validation

import "github.com/go-playground/validator/v10"

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
