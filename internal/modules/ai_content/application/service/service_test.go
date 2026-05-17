package service

import (
	"context"
	"testing"

	"erg.ninja/pkg/security/secretbox"
)

func TestEncryptKeyForStorageClearsPlaintext(t *testing.T) {
	box, err := secretbox.New("0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		t.Fatalf("secretbox.New() error = %v", err)
	}
	svc := NewService(nil, nil, nil, nil, nil, nil, box)
	key := &ApiKey{Key: "gsk_test_secret"}

	if err := svc.encryptKeyForStorage(key); err != nil {
		t.Fatalf("encryptKeyForStorage() error = %v", err)
	}

	if key.Key != "" {
		t.Fatalf("plain key must be cleared, got %q", key.Key)
	}
	if key.EncryptedKey == "" || key.KeyNonce == "" || key.KeyVersion == "" {
		t.Fatalf("encrypted fields not populated: %#v", key)
	}
	if key.MaskedKeyPreview != "gsk_...cret" {
		t.Fatalf("masked preview = %q", key.MaskedKeyPreview)
	}

	resp := apiKeyResponse(key)
	if resp.Key != "gsk_...cret" || resp.MaskedKey != "gsk_...cret" {
		t.Fatalf("response leaked unexpected key fields: %#v", resp)
	}
}

func TestPrepareKeyForRuntimeDecryptsEncryptedKey(t *testing.T) {
	box, err := secretbox.New("0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		t.Fatalf("secretbox.New() error = %v", err)
	}
	svc := NewService(nil, nil, nil, nil, nil, nil, box)
	key := &ApiKey{Key: "gsk_test_secret"}
	if err := svc.encryptKeyForStorage(key); err != nil {
		t.Fatalf("encryptKeyForStorage() error = %v", err)
	}

	if err := svc.prepareKeyForRuntime(context.Background(), key); err != nil {
		t.Fatalf("prepareKeyForRuntime() error = %v", err)
	}
	if key.Key != "gsk_test_secret" {
		t.Fatalf("decrypted key = %q", key.Key)
	}
}
