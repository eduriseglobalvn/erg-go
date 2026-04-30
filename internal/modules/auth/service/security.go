package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"time"

	authresp "erg.ninja/internal/modules/auth/dto/response"
	"erg.ninja/internal/modules/auth/entities"
	"erg.ninja/internal/modules/auth/repository"
)

const (
	loginFailEmailKeyPrefix = "auth_fail:email:"
	loginFailIPKeyPrefix    = "auth_fail:ip:"
	ipBlockedKeyPrefix      = "ip_blocked:"
	ipAllowlistKeyPrefix    = "ip_allowlisted:"
)

type LoginSecurityContext struct {
	IPAddress         string
	UserAgent         string
	DeviceID          string
	DeviceName        string
	DeviceFingerprint string
	CountryCode       string
	CountryName       string
	ContinentCode     string
}

type firewallBlockRecord struct {
	IP        string     `json:"ip"`
	Reason    string     `json:"reason"`
	Source    string     `json:"source"`
	BlockedAt time.Time  `json:"blockedAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

func (s *AuthService) normalizeSecurityContext(sec LoginSecurityContext) LoginSecurityContext {
	sec.IPAddress = strings.TrimSpace(sec.IPAddress)
	sec.UserAgent = strings.TrimSpace(sec.UserAgent)
	sec.DeviceID = strings.TrimSpace(sec.DeviceID)
	sec.DeviceName = strings.TrimSpace(sec.DeviceName)
	sec.DeviceFingerprint = strings.TrimSpace(sec.DeviceFingerprint)
	sec.CountryCode = strings.ToUpper(strings.TrimSpace(sec.CountryCode))
	sec.CountryName = strings.TrimSpace(sec.CountryName)
	sec.ContinentCode = strings.ToUpper(strings.TrimSpace(sec.ContinentCode))

	if sec.DeviceID == "" {
		sec.DeviceID = sec.DeviceFingerprint
	}
	if sec.ContinentCode == "" && sec.CountryCode != "" {
		sec.ContinentCode = continentForCountryCode(sec.CountryCode)
	}
	return sec
}

func (s *AuthService) precheckLoginSecurity(ctx context.Context, tenantID, email string, sec LoginSecurityContext) (entities.LoginAttemptReason, error) {
	if sec.IPAddress != "" {
		if blocked, _ := s.isIPBlocked(ctx, sec.IPAddress); blocked {
			return entities.LoginAttemptReasonIPBlocked, ErrIPBlocked
		}
	}

	allowlisted, _ := s.isIPAllowlisted(ctx, sec.IPAddress)
	if !allowlisted && s.shouldBlockByGeo(sec) {
		s.blockIP(ctx, sec.IPAddress, "geo_blocked")
		return entities.LoginAttemptReasonGeoBlocked, ErrGeoBlocked
	}

	if s.cfg.Auth.MaxFailedLogin > 0 {
		emailCount, err := s.failedLoginCount(ctx, tenantID, email, "")
		if err != nil {
			return "", err
		}
		ipCount, err := s.failedLoginCount(ctx, tenantID, "", sec.IPAddress)
		if err != nil {
			return "", err
		}
		if emailCount >= int64(s.cfg.Auth.MaxFailedLogin) || ipCount >= int64(s.cfg.Auth.MaxFailedLogin) {
			s.blockIP(ctx, sec.IPAddress, "too_many_attempts")
			return entities.LoginAttemptReasonTooManyAttempts, ErrTooManyAttempts
		}
	}

	return "", nil
}

func (s *AuthService) shouldBlockByGeo(sec LoginSecurityContext) bool {
	if !s.cfg.Auth.GeoBlockEnabled || sec.IPAddress == "" || isPrivateOrLoopbackIP(sec.IPAddress) {
		return false
	}
	allowed := s.cfg.Auth.AllowedContinents
	if len(allowed) == 0 {
		allowed = []string{"AS"}
	}
	if sec.ContinentCode == "" {
		return s.cfg.Auth.BlockUnknownGeo
	}
	return !isAllowedContinent(sec.ContinentCode, allowed)
}

func (s *AuthService) recordFailedLogin(ctx context.Context, tenantID, email string, sec LoginSecurityContext) error {
	if s.cfg.Auth.MaxFailedLogin <= 0 {
		return nil
	}
	window := s.failedLoginWindow()
	emailCount, err := s.failedLoginCount(ctx, tenantID, email, "")
	if err != nil {
		return err
	}
	ipCount, err := s.failedLoginCount(ctx, tenantID, "", sec.IPAddress)
	if err != nil {
		return err
	}
	if normalizeEmailForSecurity(email) != "" {
		emailCount++
		s.incrementCounter(ctx, failedEmailKey(tenantID, email), window)
	}
	if strings.TrimSpace(sec.IPAddress) != "" {
		ipCount++
		s.incrementCounter(ctx, failedIPKey(sec.IPAddress), window)
	}
	if emailCount >= int64(s.cfg.Auth.MaxFailedLogin) || ipCount >= int64(s.cfg.Auth.MaxFailedLogin) {
		s.blockIP(ctx, sec.IPAddress, "too_many_attempts")
		return ErrTooManyAttempts
	}
	return nil
}

func (s *AuthService) resetFailedLoginCounters(ctx context.Context, tenantID, email, ip string) {
	if s.redis == nil {
		return
	}
	keys := []string{failedEmailKey(tenantID, email)}
	if strings.TrimSpace(ip) != "" {
		keys = append(keys, failedIPKey(ip))
	}
	_ = s.redis.Del(ctx, keys...)
}

func (s *AuthService) recordLoginAttempt(ctx context.Context, tenantID, userID, email string, result entities.LoginAttemptResult, reason entities.LoginAttemptReason, sec LoginSecurityContext) {
	attempt := &entities.LoginAttempt{
		TenantID:           tenantID,
		UserID:             strings.TrimSpace(userID),
		AttemptedEmail:     normalizeEmailForSecurity(email),
		AttemptedEmailHash: emailHash(email),
		IPAddress:          sec.IPAddress,
		CountryCode:        sec.CountryCode,
		CountryName:        sec.CountryName,
		ContinentCode:      sec.ContinentCode,
		UserAgent:          sec.UserAgent,
		DeviceID:           sec.DeviceID,
		DeviceName:         sec.DeviceName,
		Result:             result,
		Reason:             reason,
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.repo.CreateLoginAttempt(ctx, attempt); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("ip", sec.IPAddress).Str("result", string(result)).Msg("auth.service: failed to store login attempt")
	}
}

func (s *AuthService) ListLoginAttempts(ctx context.Context, params repository.LoginAttemptListParams) ([]authresp.LoginAttemptResponse, int64, error) {
	if params.TenantID == "" {
		params.TenantID = s.defaultTenantID()
	}
	attempts, total, err := s.repo.ListLoginAttempts(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	out := make([]authresp.LoginAttemptResponse, 0, len(attempts))
	for i := range attempts {
		out = append(out, authresp.NewLoginAttemptResponse(&attempts[i]))
	}
	return out, total, nil
}

func (s *AuthService) GetIPSecurityStatus(ctx context.Context, email string, sec LoginSecurityContext) (*authresp.IPSecurityStatusResponse, error) {
	sec = s.normalizeSecurityContext(sec)
	blocked, err := s.isIPBlocked(ctx, sec.IPAddress)
	if err != nil {
		return nil, err
	}
	allowlisted, err := s.isIPAllowlisted(ctx, sec.IPAddress)
	if err != nil {
		return nil, err
	}
	return &authresp.IPSecurityStatusResponse{
		IP:                  sec.IPAddress,
		Blocked:             blocked,
		Allowlisted:         allowlisted,
		CountryCode:         sec.CountryCode,
		ContinentCode:       sec.ContinentCode,
		GeoBlocked:          !allowlisted && s.shouldBlockByGeo(sec),
		FailedAttemptsIP:    s.mustFailedLoginCount(ctx, s.defaultTenantID(), "", sec.IPAddress),
		FailedAttemptsEmail: s.mustFailedLoginCount(ctx, s.defaultTenantID(), email, ""),
		WindowSeconds:       int64(s.failedLoginWindow().Seconds()),
		Threshold:           s.cfg.Auth.MaxFailedLogin,
	}, nil
}

func (s *AuthService) failedLoginCount(ctx context.Context, tenantID, email, ip string) (int64, error) {
	emailHash := ""
	if email != "" {
		emailHash = emailHashForSecurity(email)
	}
	ip = strings.TrimSpace(ip)
	if emailHash == "" && ip == "" {
		return 0, nil
	}
	return s.repo.CountFailedLoginAttempts(ctx, repository.FailedLoginAttemptCountParams{
		TenantID:       tenantID,
		EmailHash:      emailHash,
		IP:             ip,
		Since:          time.Now().UTC().Add(-s.failedLoginWindow()),
		ResetOnSuccess: true,
	})
}

func (s *AuthService) mustFailedLoginCount(ctx context.Context, tenantID, email, ip string) int64 {
	count, err := s.failedLoginCount(ctx, tenantID, email, ip)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Str("ip", ip).Msg("auth.service: failed-login count lookup failed")
		return 0
	}
	return count
}

func (s *AuthService) incrementCounter(ctx context.Context, key string, ttl time.Duration) int64 {
	if s.redis == nil || key == "" {
		return 0
	}
	count, err := s.redis.Incr(ctx, key)
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Str("key", key).Msg("auth.service: failed-login counter increment failed")
		return 0
	}
	existingTTL, _ := s.redis.TTL(ctx, key)
	if existingTTL <= 0 {
		_ = s.redis.Expire(ctx, key, ttl)
	}
	return count
}

func (s *AuthService) blockIP(ctx context.Context, ip, reason string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	duration := s.blockDuration()
	expiresAt := time.Now().UTC().Add(duration)
	record := firewallBlockRecord{
		IP:        ip,
		Reason:    strings.TrimSpace(reason),
		Source:    "auth",
		BlockedAt: time.Now().UTC(),
		ExpiresAt: &expiresAt,
	}
	if err := s.repo.UpsertFirewallRule(ctx, repository.FirewallRuleRecord{
		Entry:     record.IP,
		RuleType:  repository.FirewallRuleTypeBlock,
		Reason:    record.Reason,
		Source:    record.Source,
		ExpiresAt: record.ExpiresAt,
		CreatedAt: record.BlockedAt,
		UpdatedAt: record.BlockedAt,
	}); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("ip", record.IP).Msg("auth.service: failed to persist ip block")
	}
	if s.redis == nil {
		return
	}
	raw, err := json.Marshal(record)
	if err != nil {
		raw = []byte("true")
	}
	if err := s.redis.Set(ctx, ipBlockedKeyPrefix+record.IP, string(raw), duration); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("ip", record.IP).Msg("auth.service: failed to cache ip block")
	}
}

func (s *AuthService) isIPBlocked(ctx context.Context, ip string) (bool, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false, nil
	}
	blocked, err := s.repo.IsFirewallRuleActive(ctx, repository.FirewallRuleTypeBlock, ip)
	if err != nil {
		return false, err
	}
	return blocked, nil
}

func (s *AuthService) isIPAllowlisted(ctx context.Context, ip string) (bool, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false, nil
	}
	rules, err := s.repo.ListActiveFirewallRules(ctx, repository.FirewallRuleTypeAllowlist)
	if err != nil {
		return false, err
	}
	for _, rule := range rules {
		if ipMatchesAllowlistEntry(ip, rule.Entry) {
			return true, nil
		}
	}
	return false, nil
}

func (s *AuthService) failedLoginWindow() time.Duration {
	if s.cfg.Auth.FailedLoginWindow > 0 {
		return s.cfg.Auth.FailedLoginWindow
	}
	if s.cfg.Auth.BlockDuration > 0 {
		return s.cfg.Auth.BlockDuration
	}
	return 15 * time.Minute
}

func (s *AuthService) blockDuration() time.Duration {
	if s.cfg.Auth.BlockDuration > 0 {
		return s.cfg.Auth.BlockDuration
	}
	return 15 * time.Minute
}

func failedEmailKey(tenantID, email string) string {
	email = normalizeEmailForSecurity(email)
	if email == "" {
		return ""
	}
	return loginFailEmailKeyPrefix + strings.TrimSpace(tenantID) + ":" + emailHash(email)
}

func failedIPKey(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	return loginFailIPKeyPrefix + ip
}

func normalizeEmailForSecurity(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func emailHash(email string) string {
	normalized := normalizeEmailForSecurity(email)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func emailHashForSecurity(email string) string {
	return emailHash(email)
}

func isAllowedContinent(continent string, allowed []string) bool {
	continent = strings.ToUpper(strings.TrimSpace(continent))
	for _, item := range allowed {
		if continent == strings.ToUpper(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

func isPrivateOrLoopbackIP(ip string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
}

func ipMatchesAllowlistEntry(ip, entry string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	entry = strings.TrimSpace(entry)
	if prefix, err := netip.ParsePrefix(entry); err == nil {
		return prefix.Contains(addr)
	}
	allowedAddr, err := netip.ParseAddr(entry)
	return err == nil && allowedAddr == addr
}

func continentForCountryCode(country string) string {
	switch strings.ToUpper(strings.TrimSpace(country)) {
	case "AF", "AM", "AZ", "BH", "BD", "BT", "BN", "KH", "CN", "CY", "GE", "HK", "IN", "ID", "IR", "IQ", "IL", "JP", "JO", "KZ", "KW", "KG", "LA", "LB", "MO", "MY", "MV", "MN", "MM", "NP", "KP", "OM", "PK", "PS", "PH", "QA", "SA", "SG", "KR", "LK", "SY", "TW", "TJ", "TH", "TL", "TR", "TM", "AE", "UZ", "VN", "YE", "RU":
		return "AS"
	default:
		return ""
	}
}
