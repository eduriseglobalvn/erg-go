package response

import (
	"time"

	"erg.ninja/internal/modules/auth/entities"
)

type LoginAttemptResponse struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenantId"`
	UserID         string `json:"userId,omitempty"`
	AttemptedEmail string `json:"attemptedEmail,omitempty"`
	IPAddress      string `json:"ipAddress"`
	CountryCode    string `json:"countryCode,omitempty"`
	CountryName    string `json:"countryName,omitempty"`
	ContinentCode  string `json:"continentCode,omitempty"`
	UserAgent      string `json:"userAgent,omitempty"`
	DeviceID       string `json:"deviceId,omitempty"`
	DeviceName     string `json:"deviceName,omitempty"`
	Result         string `json:"result"`
	Reason         string `json:"reason"`
	CreatedAt      string `json:"createdAt"`
}

type IPSecurityStatusResponse struct {
	IP                  string `json:"ip"`
	Blocked             bool   `json:"blocked"`
	Allowlisted         bool   `json:"allowlisted"`
	CountryCode         string `json:"countryCode,omitempty"`
	ContinentCode       string `json:"continentCode,omitempty"`
	GeoBlocked          bool   `json:"geoBlocked"`
	FailedAttemptsIP    int64  `json:"failedAttemptsIp"`
	FailedAttemptsEmail int64  `json:"failedAttemptsEmail,omitempty"`
	WindowSeconds       int64  `json:"windowSeconds"`
	Threshold           int    `json:"threshold"`
}

func NewLoginAttemptResponse(attempt *entities.LoginAttempt) LoginAttemptResponse {
	if attempt == nil {
		return LoginAttemptResponse{}
	}
	return LoginAttemptResponse{
		ID:             attempt.ID.Hex(),
		TenantID:       attempt.TenantID,
		UserID:         attempt.UserID,
		AttemptedEmail: attempt.AttemptedEmail,
		IPAddress:      attempt.IPAddress,
		CountryCode:    attempt.CountryCode,
		CountryName:    attempt.CountryName,
		ContinentCode:  attempt.ContinentCode,
		UserAgent:      attempt.UserAgent,
		DeviceID:       attempt.DeviceID,
		DeviceName:     attempt.DeviceName,
		Result:         string(attempt.Result),
		Reason:         string(attempt.Reason),
		CreatedAt:      attempt.CreatedAt.UTC().Format(time.RFC3339),
	}
}
