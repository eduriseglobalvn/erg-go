package entity

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type LoginAttemptResult string

const (
	LoginAttemptSuccess LoginAttemptResult = "success"
	LoginAttemptFailed  LoginAttemptResult = "failed"
	LoginAttemptBlocked LoginAttemptResult = "blocked"
)

type LoginAttemptReason string

const (
	LoginAttemptReasonSuccess            LoginAttemptReason = "success"
	LoginAttemptReasonInvalidCredentials LoginAttemptReason = "invalid_credentials" // #nosec G101 -- audit reason code, not a credential value.
	LoginAttemptReasonAccountLocked      LoginAttemptReason = "account_locked"
	LoginAttemptReasonEmailNotVerified   LoginAttemptReason = "email_not_verified"
	LoginAttemptReasonTooManyAttempts    LoginAttemptReason = "too_many_attempts"
	LoginAttemptReasonIPBlocked          LoginAttemptReason = "ip_blocked"
	LoginAttemptReasonGeoBlocked         LoginAttemptReason = "geo_blocked"
	LoginAttemptReasonGoogleInvalid      LoginAttemptReason = "google_identity_invalid"
)

// LoginAttempt is an immutable audit record for auth attempts.
type LoginAttempt struct {
	ID                 bson.ObjectID      `json:"id"`
	TenantID           string             `json:"tenantId"`
	UserID             string             `json:"userId,omitempty"`
	AttemptedEmail     string             `json:"attemptedEmail,omitempty"`
	AttemptedEmailHash string             `json:"attemptedEmailHash,omitempty"`
	IPAddress          string             `json:"ipAddress"`
	CountryCode        string             `json:"countryCode,omitempty"`
	CountryName        string             `json:"countryName,omitempty"`
	ContinentCode      string             `json:"continentCode,omitempty"`
	UserAgent          string             `json:"userAgent,omitempty"`
	DeviceID           string             `json:"deviceId,omitempty"`
	DeviceName         string             `json:"deviceName,omitempty"`
	Result             LoginAttemptResult `json:"result"`
	Reason             LoginAttemptReason `json:"reason"`
	CreatedAt          time.Time          `json:"createdAt"`
}
