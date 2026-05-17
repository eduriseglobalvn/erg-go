// Package platform provides platform-specific API clients for Discord and Telegram.
package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// hmacSHA256 computes HMAC-SHA256 and returns hex-encoded result.
func hmacSHA256(data, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return []byte(hex.EncodeToString(mac.Sum(nil)))
}

// hmacSHA256Raw computes HMAC-SHA256 and returns raw bytes.
func hmacSHA256Raw(data, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// hmacEqual performs constant-time comparison of two byte slices.
func hmacEqual(a, b []byte) bool {
	return hmac.Equal(a, b)
}
