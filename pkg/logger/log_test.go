package logger

import "testing"

func TestRedactValueMasksSensitiveKeys(t *testing.T) {
	for _, key := range []string{
		"Authorization",
		"password",
		"refresh_token",
		"SECRET_R2_ACCESS_KEY_ID",
		"cookie",
	} {
		if got := RedactValue(key, "sensitive"); got != "[REDACTED]" {
			t.Fatalf("RedactValue(%q) = %v, want redacted", key, got)
		}
	}
}

func TestRedactValueLeavesSafeKeys(t *testing.T) {
	if got := RedactValue("user_id", "u1"); got != "u1" {
		t.Fatalf("RedactValue safe key = %v", got)
	}
}
