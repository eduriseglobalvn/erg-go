package secretbox

import "testing"

func TestEncryptDecryptString(t *testing.T) {
	box, err := New("0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ciphertext, nonce, err := box.EncryptString("gsk_test_secret")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}
	if ciphertext == "" || nonce == "" {
		t.Fatal("expected ciphertext and nonce")
	}
	if ciphertext == "gsk_test_secret" {
		t.Fatal("ciphertext must not equal plaintext")
	}

	plain, err := box.DecryptString(ciphertext, nonce, VersionV1)
	if err != nil {
		t.Fatalf("DecryptString() error = %v", err)
	}
	if plain != "gsk_test_secret" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestRejectsWeakSecret(t *testing.T) {
	if _, err := New("short"); err != ErrInvalidSecret {
		t.Fatalf("expected ErrInvalidSecret, got %v", err)
	}
}

func TestFingerprintIsStableAndSecretBacked(t *testing.T) {
	box, err := New("0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	first := box.Fingerprint("secret")
	second := box.Fingerprint("secret")
	if first == "" || first != second {
		t.Fatalf("fingerprint not stable: %q %q", first, second)
	}
}
