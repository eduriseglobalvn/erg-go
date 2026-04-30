package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"

	"erg.ninja/internal/modules/operations/entities"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

type SystemStatus struct {
	OS            string    `json:"os"`
	Platform      string    `json:"platform"`
	Uptime        uint64    `json:"uptime"`
	CPUUsage      float64   `json:"cpu_usage"`
	MemoryPercent float64   `json:"memory_percent"`
	MemoryTotal   uint64    `json:"memory_total"`
	MemoryFree    uint64    `json:"memory_free"`
	GoRoutines    int       `json:"go_routines"`
	Timestamp     time.Time `json:"timestamp"`
	DBStatus      string    `json:"db_status"`
}

type Service struct {
	mongo *database.MongoClient
	pg    *database.GORMPostgresClient
	redis *cache.RedisClient
	log   *logger.Logger
}

type FirewallRule struct {
	IP        string     `json:"ip,omitempty"`
	Entry     string     `json:"entry,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	Source    string     `json:"source,omitempty"`
	CreatedAt time.Time  `json:"createdAt,omitempty"`
	BlockedAt time.Time  `json:"blockedAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

func NewService(mongo *database.MongoClient, pg *database.GORMPostgresClient, redis *cache.RedisClient, log *logger.Logger) *Service {
	return &Service{
		mongo: mongo,
		pg:    pg,
		redis: redis,
		log:   log,
	}
}

func (s *Service) GetSystemStatus(ctx context.Context) (*SystemStatus, error) {
	v, _ := mem.VirtualMemory()
	c, _ := cpu.Percent(time.Second, false)
	h, _ := host.Info()

	dbStatus := "connected"
	switch {
	case s.pg != nil && s.pg.Ping(ctx) != nil:
		dbStatus = "disconnected"
	case s.pg == nil && s.mongo != nil && s.mongo.Client().Ping(ctx, nil) != nil:
		dbStatus = "disconnected"
	}

	cpuUsage := 0.0
	if len(c) > 0 {
		cpuUsage = c[0]
	}

	return &SystemStatus{
		OS:            runtime.GOOS,
		Platform:      h.Platform,
		Uptime:        h.Uptime,
		CPUUsage:      cpuUsage,
		MemoryPercent: v.UsedPercent,
		MemoryTotal:   v.Total,
		MemoryFree:    v.Free,
		GoRoutines:    runtime.NumGoroutine(),
		Timestamp:     time.Now().UTC(),
		DBStatus:      dbStatus,
	}, nil
}

// ─── IP Firewall Logic ───────────────────────────────────────────────────────

const (
	ipBlockedKeyPrefix   = "ip_blocked:"
	ipAllowlistKeyPrefix = "ip_allowlisted:"
)

func (s *Service) BlockIP(ctx context.Context, ip string, duration time.Duration) error {
	return s.BlockIPWithMetadata(ctx, ip, duration, "manual", "operations")
}

func (s *Service) BlockIPWithMetadata(ctx context.Context, ip string, duration time.Duration, reason, source string) error {
	if s.redis == nil || s.redis.Client() == nil {
		return fmt.Errorf("operations: redis client unavailable")
	}
	ip = strings.TrimSpace(ip)
	key := ipBlockedKeyPrefix + ip
	if duration <= 0 {
		duration = 365 * 24 * time.Hour // 1 year
	}
	expiresAt := time.Now().UTC().Add(duration)
	record := FirewallRule{
		IP:        ip,
		Reason:    strings.TrimSpace(reason),
		Source:    strings.TrimSpace(source),
		BlockedAt: time.Now().UTC(),
		ExpiresAt: &expiresAt,
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("operations: marshal firewall block: %w", err)
	}
	err = s.redis.Client().Set(ctx, key, string(raw), duration).Err()
	if err != nil {
		return fmt.Errorf("operations: failed to block ip: %w", err)
	}
	s.log.Info().Str("ip", ip).Str("reason", reason).Msg("operations: ip blocked")
	return nil
}

func (s *Service) UnblockIP(ctx context.Context, ip string) error {
	if s.redis == nil || s.redis.Client() == nil {
		return fmt.Errorf("operations: redis client unavailable")
	}
	key := ipBlockedKeyPrefix + strings.TrimSpace(ip)
	err := s.redis.Client().Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("operations: failed to unblock ip: %w", err)
	}
	s.log.Info().Str("ip", ip).Msg("operations: ip unblocked")
	return nil
}

func (s *Service) IsIPBlocked(ctx context.Context, ip string) (bool, error) {
	if s.redis == nil || s.redis.Client() == nil {
		return false, fmt.Errorf("operations: redis client unavailable")
	}
	key := ipBlockedKeyPrefix + strings.TrimSpace(ip)
	n, err := s.redis.Client().Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Service) ListBlockedIPs(ctx context.Context) ([]map[string]any, error) {
	if s.redis == nil || s.redis.Client() == nil {
		return nil, fmt.Errorf("operations: redis client unavailable")
	}
	keys, err := s.redis.Client().Keys(ctx, ipBlockedKeyPrefix+"*").Result()
	if err != nil {
		return nil, err
	}

	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		ttl, _ := s.redis.Client().PTTL(ctx, key).Result()
		raw, _ := s.redis.Client().Get(ctx, key).Result()
		ip := strings.TrimPrefix(key, ipBlockedKeyPrefix)
		record := map[string]any{
			"ip":  ip,
			"ttl": ttl.Milliseconds(),
		}
		var meta FirewallRule
		if err := json.Unmarshal([]byte(raw), &meta); err == nil {
			record["reason"] = meta.Reason
			record["source"] = meta.Source
			record["blockedAt"] = meta.BlockedAt
			record["expiresAt"] = meta.ExpiresAt
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Service) AllowlistIP(ctx context.Context, entry, reason string, duration time.Duration) error {
	if s.redis == nil || s.redis.Client() == nil {
		return fmt.Errorf("operations: redis client unavailable")
	}
	entry = strings.TrimSpace(entry)
	if !isValidIPOrCIDR(entry) {
		return fmt.Errorf("operations: invalid allowlist entry %q", entry)
	}
	record := FirewallRule{
		Entry:     entry,
		Reason:    strings.TrimSpace(reason),
		Source:    "operations",
		CreatedAt: time.Now().UTC(),
	}
	if duration > 0 {
		expiresAt := record.CreatedAt.Add(duration)
		record.ExpiresAt = &expiresAt
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("operations: marshal allowlist record: %w", err)
	}
	if err := s.redis.Client().Set(ctx, ipAllowlistKeyPrefix+entry, string(raw), duration).Err(); err != nil {
		return fmt.Errorf("operations: failed to allowlist ip: %w", err)
	}
	return nil
}

func (s *Service) RemoveAllowlistIP(ctx context.Context, entry string) error {
	if s.redis == nil || s.redis.Client() == nil {
		return fmt.Errorf("operations: redis client unavailable")
	}
	return s.redis.Client().Del(ctx, ipAllowlistKeyPrefix+strings.TrimSpace(entry)).Err()
}

func (s *Service) ListAllowlistedIPs(ctx context.Context) ([]map[string]any, error) {
	if s.redis == nil || s.redis.Client() == nil {
		return nil, fmt.Errorf("operations: redis client unavailable")
	}
	keys, err := s.redis.Client().Keys(ctx, ipAllowlistKeyPrefix+"*").Result()
	if err != nil {
		return nil, err
	}
	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		ttl, _ := s.redis.Client().PTTL(ctx, key).Result()
		entry := strings.TrimPrefix(key, ipAllowlistKeyPrefix)
		record := map[string]any{"entry": entry, "ttl": ttl.Milliseconds()}
		raw, _ := s.redis.Client().Get(ctx, key).Result()
		var meta FirewallRule
		if err := json.Unmarshal([]byte(raw), &meta); err == nil {
			record["reason"] = meta.Reason
			record["source"] = meta.Source
			record["createdAt"] = meta.CreatedAt
			record["expiresAt"] = meta.ExpiresAt
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Service) IsIPAllowlisted(ctx context.Context, ip string) (bool, error) {
	if s.redis == nil || s.redis.Client() == nil {
		return false, fmt.Errorf("operations: redis client unavailable")
	}
	keys, err := s.redis.Client().Keys(ctx, ipAllowlistKeyPrefix+"*").Result()
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		entry := strings.TrimPrefix(key, ipAllowlistKeyPrefix)
		if ipMatchesAllowlistEntry(ip, entry) {
			return true, nil
		}
	}
	return false, nil
}

func isValidIPOrCIDR(entry string) bool {
	entry = strings.TrimSpace(entry)
	if _, err := netip.ParseAddr(entry); err == nil {
		return true
	}
	_, err := netip.ParsePrefix(entry)
	return err == nil
}

func ipMatchesAllowlistEntry(ip, entry string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	if prefix, err := netip.ParsePrefix(strings.TrimSpace(entry)); err == nil {
		return prefix.Contains(addr)
	}
	allowedAddr, err := netip.ParseAddr(strings.TrimSpace(entry))
	return err == nil && allowedAddr == addr
}

func (s *Service) ListLogs(ctx context.Context, page, limit int) ([]map[string]any, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	coll := s.mongo.Database().Collection("audit_logs")
	total, _ := coll.CountDocuments(ctx, bson.M{})

	opts := options.Find().SetLimit(int64(limit)).SetSkip(skip).SetSort(bson.M{"timestamp": -1})
	cursor, err := coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []map[string]any
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

// ─── System Config Logic ─────────────────────────────────────────────────────

func (s *Service) GetConfig(ctx context.Context, key string) (*entities.SystemConfig, error) {
	if s.pg == nil {
		return s.getConfigFromMongo(ctx, key)
	}
	var record postgrescore.SystemConfig
	if err := s.pg.DB().WithContext(ctx).Where("key = ?", key).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return configFromRecord(&record)
}

func (s *Service) SetConfig(ctx context.Context, key string, value any, description string, userID string) error {
	if s.pg == nil {
		return s.setConfigMongo(ctx, key, value, description, userID)
	}

	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("operations: marshal config %s: %w", key, err)
	}

	now := time.Now().UTC()
	record := postgrescore.SystemConfig{
		ID:        database.NewID(),
		Key:       key,
		ValueJSON: string(valueJSON),
		UpdatedBy: userID,
		UpdatedAt: now,
	}
	if description != "" {
		record.Description = description
	}

	return s.pg.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing postgrescore.SystemConfig
		err := tx.Where("key = ?", key).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			updates := map[string]any{
				"value_json": record.ValueJSON,
				"updated_by": record.UpdatedBy,
				"updated_at": record.UpdatedAt,
			}
			if description != "" {
				updates["description"] = description
			}
			return tx.Model(&postgrescore.SystemConfig{}).Where("key = ?", key).Updates(updates).Error
		}
		return tx.Create(&record).Error
	})
}

func (s *Service) ListConfigs(ctx context.Context) ([]entities.SystemConfig, error) {
	if s.pg == nil {
		return s.listConfigsMongo(ctx)
	}

	var records []postgrescore.SystemConfig
	if err := s.pg.DB().WithContext(ctx).Order("\"key\" ASC").Find(&records).Error; err != nil {
		return nil, err
	}

	results := make([]entities.SystemConfig, 0, len(records))
	for i := range records {
		cfg, err := configFromRecord(&records[i])
		if err != nil {
			return nil, err
		}
		results = append(results, *cfg)
	}
	return results, nil
}

// DeleteConfig removes a system configuration key.
func (s *Service) DeleteConfig(ctx context.Context, key string) error {
	if s.pg == nil {
		coll := s.mongo.Database().Collection("system_configs")
		_, err := coll.DeleteOne(ctx, bson.M{"key": key})
		return err
	}
	return s.pg.DB().WithContext(ctx).Where("key = ?", key).Delete(&postgrescore.SystemConfig{}).Error
}

// SeedDefaults initializes the system_configs collection with default values if they don't exist.
func (s *Service) SeedDefaults(ctx context.Context) error {
	defaults := []entities.SystemConfig{
		{
			Key:         "api.maintenance_mode",
			Value:       false,
			Description: "Bật/tắt chế độ bảo trì toàn hệ thống",
		},
		{
			Key:         "ai.post_generation_limit",
			Value:       50,
			Description: "Giới hạn số bài viết AI tạo mỗi giờ",
		},
		{
			Key:         "ai.image_generation_limit",
			Value:       20,
			Description: "Giới hạn số ảnh AI tạo mỗi giờ",
		},
		{
			Key:         "crawler.concurrency",
			Value:       3,
			Description: "Số lượng link cào đồng thời tối đa",
		},
		{
			Key:         "seo.auto_linking_enabled",
			Value:       true,
			Description: "Tự động gắn link từ kho từ khóa vào nội dung",
		},
	}

	for _, item := range defaults {
		existing, err := s.GetConfig(ctx, item.Key)
		if err != nil {
			return err
		}
		if existing == nil {
			item.UpdatedAt = time.Now().UTC()
			if err := s.SetConfig(ctx, item.Key, item.Value, item.Description, ""); err != nil {
				return err
			}
			s.log.Info().Str("key", item.Key).Msg("operations: seeded default config")
		}
	}
	return nil
}

func (s *Service) getConfigFromMongo(ctx context.Context, key string) (*entities.SystemConfig, error) {
	coll := s.mongo.Database().Collection("system_configs")
	var cfg entities.SystemConfig
	err := coll.FindOne(ctx, bson.M{"key": key}).Decode(&cfg)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (s *Service) setConfigMongo(ctx context.Context, key string, value any, description string, userID string) error {
	coll := s.mongo.Database().Collection("system_configs")

	update := bson.M{
		"$set": bson.M{
			"value":      value,
			"updated_by": userID,
			"updated_at": time.Now().UTC(),
		},
	}
	if description != "" {
		update["$set"].(bson.M)["description"] = description
	}

	opts := options.UpdateOne().SetUpsert(true)
	_, err := coll.UpdateOne(ctx, bson.M{"key": key}, update, opts)
	return err
}

func (s *Service) listConfigsMongo(ctx context.Context) ([]entities.SystemConfig, error) {
	coll := s.mongo.Database().Collection("system_configs")
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []entities.SystemConfig
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func configFromRecord(record *postgrescore.SystemConfig) (*entities.SystemConfig, error) {
	value, err := unmarshalConfigValue(record.ValueJSON)
	if err != nil {
		return nil, err
	}
	return &entities.SystemConfig{
		ID:          record.ID,
		Key:         record.Key,
		Value:       value,
		Description: record.Description,
		UpdatedBy:   record.UpdatedBy,
		UpdatedAt:   record.UpdatedAt,
	}, nil
}

func unmarshalConfigValue(raw string) (any, error) {
	if raw == "" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	return value, nil
}
