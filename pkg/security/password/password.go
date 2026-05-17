package password

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	defaultMemory      uint32 = 32 * 1024
	defaultIterations  uint32 = 2
	defaultParallelism uint8  = 2
	saltLength                = 16
	keyLength                 = 32
)

var (
	ErrEmptyPassword = errors.New("password: password cannot be empty")
	ErrInvalidHash   = errors.New("password: invalid hash format")
)

type Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
}

func NormalizeParams(memory, iterations uint32) Params {
	params := Params{
		Memory:      memory,
		Iterations:  iterations,
		Parallelism: defaultParallelism,
	}
	if params.Memory == 0 {
		params.Memory = defaultMemory
	}
	if params.Iterations == 0 {
		params.Iterations = defaultIterations
	}
	return params
}

func Hash(plain string, params Params) (string, error) {
	if strings.TrimSpace(plain) == "" {
		return "", ErrEmptyPassword
	}
	params = normalize(params)
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password: generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(plain), salt, params.Iterations, params.Memory, params.Parallelism, keyLength)
	enc := base64.RawStdEncoding
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		params.Memory,
		params.Iterations,
		params.Parallelism,
		enc.EncodeToString(salt),
		enc.EncodeToString(hash),
	), nil
}

func Verify(plain, stored string, expected Params) (bool, bool) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return false, false
	}
	if strings.HasPrefix(stored, "$argon2id$") {
		ok, actual := verifyArgon2ID(plain, stored)
		return ok, ok && !sameParams(actual, normalize(expected))
	}
	if verifyLegacySHA256(plain, stored) {
		return true, true
	}
	return false, false
}

func normalize(params Params) Params {
	if params.Memory == 0 {
		params.Memory = defaultMemory
	}
	if params.Iterations == 0 {
		params.Iterations = defaultIterations
	}
	if params.Parallelism == 0 {
		params.Parallelism = defaultParallelism
	}
	return params
}

func sameParams(a, b Params) bool {
	return a.Memory == b.Memory && a.Iterations == b.Iterations && a.Parallelism == b.Parallelism
}

func verifyArgon2ID(plain, stored string) (bool, Params) {
	parts := strings.Split(stored, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, Params{}
	}
	params, err := parseParams(parts[3])
	if err != nil {
		return false, Params{}
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false, Params{}
	}
	expected, err := enc.DecodeString(parts[5])
	if err != nil || len(expected) != keyLength {
		return false, Params{}
	}
	actual := argon2.IDKey([]byte(plain), salt, params.Iterations, params.Memory, params.Parallelism, keyLength)
	return subtle.ConstantTimeCompare(actual, expected) == 1, params
}

func parseParams(raw string) (Params, error) {
	var params Params
	for _, part := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return Params{}, ErrInvalidHash
		}
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil || n == 0 {
			return Params{}, ErrInvalidHash
		}
		switch key {
		case "m":
			params.Memory = uint32(n)
		case "t":
			params.Iterations = uint32(n)
		case "p":
			if n > 255 {
				return Params{}, ErrInvalidHash
			}
			params.Parallelism = uint8(n)
		default:
			return Params{}, ErrInvalidHash
		}
	}
	params = normalize(params)
	return params, nil
}

func verifyLegacySHA256(plain, stored string) bool {
	defaultSalted := sha256.Sum256([]byte(plain + "salt_65536_3"))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(defaultSalted[:])), []byte(stored)) == 1 {
		return true
	}
	plainSHA := sha256.Sum256([]byte(plain))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(plainSHA[:])), []byte(stored)) == 1 {
		return true
	}
	const binaryPrefix = "sha256:"
	if strings.HasPrefix(stored, binaryPrefix) {
		legacyBinary := binaryPrefix + string(plainSHA[:])
		return subtle.ConstantTimeCompare([]byte(legacyBinary), []byte(stored)) == 1
	}
	return false
}
