// Package auth provides JWT validation with HMAC and RSA support.
package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"erg.ninja/pkg/config"
)

// JWTValidator validates JWT tokens using HMAC-SHA256 or RS256.
type JWTValidator struct {
	secretKey  []byte                 // for HS256
	publicKey  *rsa.PublicKey         // for RS256
	algorithms []string
	issuer     string
	log        *struct{ Debug func(...any) }
}

// JWTClaims represents the standard claims extracted from a JWT.
type JWTClaims struct {
	UserID      string   `json:"user_id,omitempty"`
	Subject     string   `json:"sub,omitempty"`
	Email       string   `json:"email,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// ValidatorOption configures a JWTValidator.
type ValidatorOption func(*JWTValidator)

// WithJWTIssuer sets the expected JWT issuer claim.
func WithJWTIssuer(issuer string) ValidatorOption {
	return func(v *JWTValidator) {
		v.issuer = issuer
	}
}

// NewHS256Validator creates a validator for HMAC-SHA256 signed tokens.
func NewHS256Validator(secret string, opts ...ValidatorOption) (*JWTValidator, error) {
	if secret == "" {
		return nil, fmt.Errorf("auth: HS256 secret cannot be empty")
	}
	v := &JWTValidator{
		secretKey:  []byte(secret),
		algorithms: []string{"HS256"},
	}
	for _, o := range opts {
		o(v)
	}
	return v, nil
}

// NewRS256Validator creates a validator for RSA-SHA256 signed tokens using a PEM-encoded public key.
func NewRS256Validator(pemKey string, opts ...ValidatorOption) (*JWTValidator, error) {
	if pemKey == "" {
		return nil, fmt.Errorf("auth: RSA public key cannot be empty")
	}

	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, fmt.Errorf("auth: failed to decode PEM block containing public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Try parsing as PKCS1 RSA public key.
		rsaPub, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("auth: parse public key: %w (also tried PKCS1: %v)", err, err2)
		}
		pub = rsaPub
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("auth: public key is not RSA")
	}

	v := &JWTValidator{
		publicKey:  rsaPub,
		algorithms: []string{"RS256"},
	}
	for _, o := range opts {
		o(v)
	}
	return v, nil
}

// NewValidatorFromConfig creates a JWTValidator from the application config.
func NewValidatorFromConfig(cfg config.AuthConfig) (*JWTValidator, error) {
	var v *JWTValidator
	var err error

	if contains(cfg.JWTAlgorithms, "RS256") && cfg.JWTPublicKey != "" {
		v, err = NewRS256Validator(cfg.JWTPublicKey)
	} else if contains(cfg.JWTAlgorithms, "HS256") && cfg.JWTSecret != "" {
		v, err = NewHS256Validator(cfg.JWTSecret)
	} else if cfg.JWTSecret != "" {
		v, err = NewHS256Validator(cfg.JWTSecret)
	} else {
		return nil, fmt.Errorf("auth: no JWT algorithm configured (set jwt_secret or jwt_public_key)")
	}
	if err != nil {
		return nil, err
	}

	if cfg.JWTIssuer != "" {
		opt := WithJWTIssuer(cfg.JWTIssuer)
		opt(v)
	}

	return v, nil
}

// Validate parses and validates a JWT token string, returning the claims on success.
func (v *JWTValidator) Validate(tokenString string) (*JWTClaims, error) {
	tokenString = strings.TrimSpace(tokenString)

	var keyFunc jwt.Keyfunc
	if len(v.secretKey) > 0 {
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if !v.isAllowedAlgorithm(token) {
				return nil, fmt.Errorf("auth: unexpected signing method: %v", token.Header["alg"])
			}
			return v.secretKey, nil
		}
	} else if v.publicKey != nil {
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if !v.isAllowedAlgorithm(token) {
				return nil, fmt.Errorf("auth: unexpected signing method: %v", token.Header["alg"])
			}
			return v.publicKey, nil
		}
	} else {
		return nil, fmt.Errorf("auth: no key configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, keyFunc)
	if err != nil {
		return nil, fmt.Errorf("auth: parse token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("auth: invalid token claims")
	}

	// Validate issuer if configured.
	if v.issuer != "" {
		issuer, err := claims.GetIssuer()
		if err != nil || issuer != v.issuer {
			return nil, fmt.Errorf("auth: invalid issuer: got %q, want %q", issuer, v.issuer)
		}
	}

	// Check expiration.
	exp, err := claims.GetExpirationTime()
	if err == nil && exp != nil && exp.Before(time.Now()) {
		return nil, fmt.Errorf("auth: token expired at %v", exp.Time)
	}

	// Normalize user ID from subject if user_id is not set.
	if claims.UserID == "" {
		claims.UserID = claims.Subject
	}

	return claims, nil
}

// isAllowedAlgorithm checks if the token's algorithm is in the allowed list.
func (v *JWTValidator) isAllowedAlgorithm(token *jwt.Token) bool {
	alg := token.Method.Alg()
	for _, a := range v.algorithms {
		if a == alg {
			return true
		}
	}
	return false
}

// GenerateHS256 generates a new HS256-signed JWT token with the given claims.
// This is useful for testing and for generating tokens in the auth service.
func (v *JWTValidator) GenerateHS256(claims *JWTClaims, expiry time.Duration) (string, error) {
	if len(v.secretKey) == 0 {
		return "", fmt.Errorf("auth: cannot generate with HS256 validator: no secret key")
	}

	if claims.Subject == "" && claims.UserID != "" {
		claims.Subject = claims.UserID
	}
	now := time.Now()
	claims.IssuedAt = jwt.NewNumericDate(now)
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(expiry))
	claims.NotBefore = jwt.NewNumericDate(now)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(v.secretKey)
}

// GenerateRS256 generates a new RS256-signed JWT token.
// Requires the private key to be set (not available in validator).
// For token generation, use a separate key management service.
func (v *JWTValidator) GenerateRS256(_ *JWTClaims, _ time.Duration) (string, error) {
	return "", fmt.Errorf("auth: RS256 token generation requires the private key; use a separate KMS")
}

// ValidateRequest is a convenience method that extracts and validates a token
// from an Authorization header string (e.g. "Bearer <token>").
func (v *JWTValidator) ValidateRequest(authHeader string) (*JWTClaims, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("auth: invalid authorization header format")
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return nil, fmt.Errorf("auth: expected Bearer token")
	}
	return v.Validate(parts[1])
}

// contains reports whether substr is within the slice.
func contains(slice []string, substr string) bool {
	for _, s := range slice {
		if s == substr {
			return true
		}
	}
	return false
}
