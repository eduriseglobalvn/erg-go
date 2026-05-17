// Package auth provides JWT validation and token generation for erg-server.
// It supports HMAC-SHA256 and RSA-SHA256 algorithms, token refresh,
// and session-based auth flows.
package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ─── JWT Validator ───────────────────────────────────────────────────────────────

// JWTValidator validates JWT tokens using HMAC-SHA256 or RS256.
type JWTValidator struct {
	secretKey  []byte
	publicKey  *rsa.PublicKey
	algorithms []string
	issuer     string
}

// JWTClaims represents the standard claims extracted from a JWT.
type JWTClaims struct {
	UserID            string            `json:"user_id,omitempty"`
	Subject           string            `json:"sub,omitempty"`
	Email             string            `json:"email,omitempty"`
	SessionID         string            `json:"session_id,omitempty"`
	TenantID          string            `json:"tenant_id,omitempty"`
	AccountType       string            `json:"account_type,omitempty"`
	AccessLevel       string            `json:"access_level,omitempty"`
	Portal            string            `json:"portal,omitempty"`
	Portals           []string          `json:"portals,omitempty"`
	Permissions       []string          `json:"permissions,omitempty"`
	DeniedPermissions []string          `json:"denied_permissions,omitempty"`
	Roles             []string          `json:"roles,omitempty"`
	Attributes        map[string]string `json:"attributes,omitempty"`
	jwt.RegisteredClaims
}

// ValidatorOption configures a JWTValidator.
type ValidatorOption func(*JWTValidator)

// WithJWTIssuer sets the expected JWT issuer claim.
func WithJWTIssuer(issuer string) ValidatorOption {
	return func(v *JWTValidator) { v.issuer = issuer }
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

// NewRS256Validator creates a validator for RSA-SHA256 signed tokens.
func NewRS256Validator(pemKey string, opts ...ValidatorOption) (*JWTValidator, error) {
	if pemKey == "" {
		return nil, fmt.Errorf("auth: RSA public key cannot be empty")
	}
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, fmt.Errorf("auth: failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		rsaPub, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
		if err2 != nil {
			pkixErr := err
			pkcs1Err := err2
			var wrapErr error = pkixErr
			_ = wrapErr
			return nil, fmt.Errorf("auth: parse RSA public key — tried PKIX: %v, PKCS1: %v: %w", pkixErr, pkcs1Err, wrapErr)
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
func NewValidatorFromConfig(cfg struct {
	JWTSecret     string
	JWTPublicKey  string
	JWTIssuer     string
	JWTAlgorithms []string
}) (*JWTValidator, error) {
	if len(cfg.JWTAlgorithms) > 0 && cfg.JWTAlgorithms[0] == "RS256" && cfg.JWTPublicKey != "" {
		return NewRS256Validator(cfg.JWTPublicKey, WithJWTIssuer(cfg.JWTIssuer))
	}
	if cfg.JWTSecret != "" {
		return NewHS256Validator(cfg.JWTSecret, WithJWTIssuer(cfg.JWTIssuer))
	}
	return nil, fmt.Errorf("auth: no JWT algorithm configured (set jwt_secret or jwt_public_key)")
}

// Validate parses and validates a JWT token string.
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
	if v.issuer != "" {
		issuer, err := claims.GetIssuer()
		if err != nil || issuer != v.issuer {
			return nil, fmt.Errorf("auth: invalid issuer: got %q, want %q", issuer, v.issuer)
		}
	}
	exp, err := claims.GetExpirationTime()
	if err == nil && exp != nil && exp.Before(time.Now()) {
		return nil, fmt.Errorf("auth: token expired at %v", exp.Time)
	}
	if claims.UserID == "" {
		claims.UserID = claims.Subject
	}
	return claims, nil
}

func (v *JWTValidator) isAllowedAlgorithm(token *jwt.Token) bool {
	for _, a := range v.algorithms {
		if a == token.Method.Alg() {
			return true
		}
	}
	return false
}

// GenerateHS256 generates a new HS256-signed JWT token.
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

// ValidateRequest extracts and validates a token from an Authorization header string.
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

// SessionIDFromClaims extracts the session ID from the Permissions slice.
func SessionIDFromClaims(c *JWTClaims) string {
	if c == nil {
		return ""
	}
	if c.SessionID != "" {
		return c.SessionID
	}
	if len(c.Permissions) > 0 {
		return c.Permissions[0]
	}
	return ""
}

// ─── Token Generation ────────────────────────────────────────────────────────────

// TokenPair holds an access token and its associated refresh token.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // access token TTL in seconds
	TokenType    string `json:"token_type"`
}

// AuthServiceProvider bundles JWT generation dependencies.
type AuthServiceProvider struct {
	accessSecret  []byte
	refreshSecret []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	issuer        string
}

// AuthProviderOption configures an AuthServiceProvider.
type AuthProviderOption func(*AuthServiceProvider)

// WithAccessExpiry sets the access token TTL.
func WithAccessExpiry(d time.Duration) AuthProviderOption {
	return func(p *AuthServiceProvider) { p.accessExpiry = d }
}

// WithRefreshExpiry sets the refresh token TTL.
func WithRefreshExpiry(d time.Duration) AuthProviderOption {
	return func(p *AuthServiceProvider) { p.refreshExpiry = d }
}

// WithIssuer sets the issuer claim for newly minted tokens.
func WithIssuer(issuer string) AuthProviderOption {
	return func(p *AuthServiceProvider) { p.issuer = issuer }
}

// TokenPairOption enriches the access token claims emitted by IssuePair.
type TokenPairOption func(*JWTClaims)

// WithPortals adds the list of product portals the access token may enter.
func WithPortals(portals []string) TokenPairOption {
	return func(c *JWTClaims) { c.Portals = append([]string(nil), portals...) }
}

// WithAccountAccess adds coarse-grained account classification to the access token.
func WithAccountAccess(accountType, accessLevel string) TokenPairOption {
	return func(c *JWTClaims) {
		c.AccountType = accountType
		c.AccessLevel = accessLevel
	}
}

// WithDeniedPermissions adds explicit permission denies to the access token.
func WithDeniedPermissions(denied []string) TokenPairOption {
	return func(c *JWTClaims) { c.DeniedPermissions = append([]string(nil), denied...) }
}

// NewAuthServiceProvider creates a new provider from secret strings.
// Falls back to sensible defaults (15m access, 7d refresh).
func NewAuthServiceProvider(accessSecret, refreshSecret string, opts ...AuthProviderOption) *AuthServiceProvider {
	if accessSecret == "" {
		accessSecret = randomProviderSecret()
	}
	if refreshSecret == "" {
		refreshSecret = randomProviderSecret()
	}
	p := &AuthServiceProvider{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessExpiry:  15 * time.Minute,
		refreshExpiry: 7 * 24 * time.Hour,
		issuer:        "erg-server",
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func randomProviderSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("auth: generate provider secret: %w", err))
	}
	return hex.EncodeToString(b)
}

// IssuePair creates a new access + refresh token pair for the given claims.
func (p *AuthServiceProvider) IssuePair(sessionID, userID, email string, roles []string, perms []string, opts ...TokenPairOption) (*TokenPair, error) {
	now := time.Now()
	atClaims := &JWTClaims{
		UserID:      userID,
		Email:       email,
		SessionID:   sessionID,
		Roles:       roles,
		Permissions: append([]string(nil), perms...),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    p.tokenIssuer(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(p.accessExpiry)),
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(atClaims)
		}
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	atSigned, err := token.SignedString(p.accessSecret)
	if err != nil {
		return nil, fmt.Errorf("auth: sign access token: %w", err)
	}
	rtClaims := &JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    p.tokenIssuer(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(p.refreshExpiry)),
		},
	}
	rtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	rtSigned, err := rtToken.SignedString(p.refreshSecret)
	if err != nil {
		return nil, fmt.Errorf("auth: sign refresh token: %w", err)
	}
	return &TokenPair{
		AccessToken:  atSigned,
		RefreshToken: rtSigned,
		ExpiresIn:    int64(p.accessExpiry.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func (p *AuthServiceProvider) tokenIssuer() string {
	if strings.TrimSpace(p.issuer) == "" {
		return "erg-server"
	}
	return p.issuer
}

// ValidateAccessToken validates an access token and returns its claims.
func (p *AuthServiceProvider) ValidateAccessToken(tokenString string) (*JWTClaims, error) {
	v, err := NewHS256Validator(string(p.accessSecret))
	if err != nil {
		return nil, err
	}
	v.issuer = p.tokenIssuer()
	return v.Validate(strings.TrimSpace(tokenString))
}

// ValidateRefreshToken validates a refresh token (uses refresh secret).
func (p *AuthServiceProvider) ValidateRefreshToken(tokenString string) (*JWTClaims, error) {
	v, err := NewHS256Validator(string(p.refreshSecret))
	if err != nil {
		return nil, err
	}
	v.issuer = p.tokenIssuer()
	return v.Validate(strings.TrimSpace(tokenString))
}
