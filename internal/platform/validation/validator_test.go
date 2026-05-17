package validation

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

type samplePayload struct {
	Email string `validate:"required,email"`
}

func TestFieldErrorsNormalizesValidatorErrors(t *testing.T) {
	v := validator.New()
	err := v.Struct(samplePayload{})

	fields := FieldErrors(err)
	if len(fields) != 1 {
		t.Fatalf("len(fields) = %d, want 1", len(fields))
	}
	if fields[0].Field != "Email" || fields[0].Code != "required" {
		t.Fatalf("unexpected field error: %+v", fields[0])
	}
}

func TestClampLimitUsesDefaultAndMaximum(t *testing.T) {
	if got := ClampLimit(0, 25, 100); got != 25 {
		t.Fatalf("ClampLimit default = %d", got)
	}
	if got := ClampLimit(500, 25, 100); got != 100 {
		t.Fatalf("ClampLimit max = %d", got)
	}
	if got := ClampLimit(50, 25, 100); got != 50 {
		t.Fatalf("ClampLimit keep = %d", got)
	}
}
