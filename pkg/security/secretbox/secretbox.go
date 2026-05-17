package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	VersionV1  = "v1"
	minSecret  = 32
	nonceBytes = 12
)

var (
	ErrMissingSecret = errors.New("secretbox: encryption secret is required")
	ErrInvalidSecret = errors.New("secretbox: encryption secret must be at least 32 characters")
)

type Box struct {
	key     []byte
	version string
}

func New(secret string) (*Box, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, ErrMissingSecret
	}
	if len(secret) < minSecret {
		return nil, ErrInvalidSecret
	}
	sum := sha256.Sum256([]byte(secret))
	return &Box{key: sum[:], version: VersionV1}, nil
}

func (b *Box) Version() string {
	if b == nil {
		return ""
	}
	return b.version
}

func (b *Box) EncryptString(plain string) (ciphertext string, nonce string, err error) {
	if b == nil {
		return "", "", ErrMissingSecret
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", "", fmt.Errorf("secretbox: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", fmt.Errorf("secretbox: gcm: %w", err)
	}
	rawNonce := make([]byte, nonceBytes)
	if _, err := io.ReadFull(rand.Reader, rawNonce); err != nil {
		return "", "", fmt.Errorf("secretbox: nonce: %w", err)
	}
	sealed := gcm.Seal(nil, rawNonce, []byte(plain), []byte(b.version))
	enc := base64.RawStdEncoding
	return enc.EncodeToString(sealed), enc.EncodeToString(rawNonce), nil
}

func (b *Box) DecryptString(ciphertext string, nonce string, version string) (string, error) {
	if b == nil {
		return "", ErrMissingSecret
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = VersionV1
	}
	if version != b.version {
		return "", fmt.Errorf("secretbox: unsupported key version %q", version)
	}
	enc := base64.RawStdEncoding
	sealed, err := enc.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return "", fmt.Errorf("secretbox: decode ciphertext: %w", err)
	}
	rawNonce, err := enc.DecodeString(strings.TrimSpace(nonce))
	if err != nil {
		return "", fmt.Errorf("secretbox: decode nonce: %w", err)
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", fmt.Errorf("secretbox: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secretbox: gcm: %w", err)
	}
	plain, err := gcm.Open(nil, rawNonce, sealed, []byte(version))
	if err != nil {
		return "", fmt.Errorf("secretbox: decrypt: %w", err)
	}
	return string(plain), nil
}

func (b *Box) Fingerprint(value string) string {
	if b == nil || strings.TrimSpace(value) == "" {
		return ""
	}
	mac := hmac.New(sha256.New, b.key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
