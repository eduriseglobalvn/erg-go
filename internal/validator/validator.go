// Package validator provides a thin, ergonomic wrapper around go-playground/validator.
// Use ValidateStruct to check structs, and FormatErrors to render errors as
// human-readable strings suitable for logging or API error messages.
package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator wraps go-playground/validator and provides structured validation.
type Validator struct {
	validate *validator.Validate
}

// New creates a Validator with standard configuration.
func New() *Validator {
	v := validator.New()

	// Register a custom tag name function to use JSON field names in error messages.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return fld.Name
		}
		return name
	})

	return &Validator{validate: v}
}

// ValidateStruct validates a struct and returns nil if it passes.
// Returns a validator.ValidationErrors if validation fails.
func (v *Validator) ValidateStruct(s any) error {
	return v.validate.Struct(s)
}

// ValidateStructPartial validates only non-ignored struct fields.
// Use this for PATCH requests where partial updates are allowed.
func (v *Validator) ValidateStructPartial(s any) error {
	return v.validate.StructPartial(s)
}

// FormatErrors renders a validator.ValidationErrors as a human-readable multi-line string.
func FormatErrors(err error) string {
	var ve validator.ValidationErrors
	if err == nil || !strings.Contains(reflect.TypeOf(err).String(), "ValidationErrors") {
		return ""
	}
	ve = err.(validator.ValidationErrors)

	var b strings.Builder
	b.WriteString("validation failed:\n")
	for _, fe := range ve {
		b.WriteString(fmt.Sprintf("  - field %q: failed tag %q\n", fe.Field(), fe.Tag()))
	}
	return b.String()
}

// FormatSingleError returns a concise one-line string for a single field error.
func FormatSingleError(fe validator.FieldError) string {
	return fmt.Sprintf("field %q failed validation: %s", fe.Field(), msgForTag(fe.Tag()))
}

// msgForTag maps a validation tag to a human-readable message template.
func msgForTag(tag string) string {
	switch tag {
	case "required":
		return "this field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "value is too short"
	case "max":
		return "value is too long"
	case "gte":
		return "value must be greater than or equal to the required minimum"
	case "lte":
		return "value must be less than or equal to the required maximum"
	case "eq":
		return "value must be equal to the required value"
	case "ne":
		return "value must not be equal to the forbidden value"
	case "uuid":
		return "must be a valid UUID"
	case "url":
		return "must be a valid URL"
	case "numeric":
		return "must be a numeric string"
	case "alphanum":
		return "must contain only letters and digits"
	case "datetime":
		return "must be a valid datetime in RFC3339 format"
	case "oneof":
		return "value must be one of the allowed values"
	default:
		return "failed validation: " + tag
	}
}
