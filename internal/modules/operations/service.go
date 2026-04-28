package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/redis/go-redis/v9"
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

func (s *Service) BlockIP(ctx context.Context, ip string, duration time.Duration) error {
	key := fmt.Sprintf("ip_blocked:%s", ip)
	if duration <= 0 {
		duration = 365 * 24 * time.Hour // 1 year
	}
	err := s.redis.Client().Set(ctx, key, "true", duration).Err()
	if err != nil {
		return fmt.Errorf("operations: failed to block ip: %w", err)
	}
	s.log.Info().Str("ip", ip).Msg("operations: ip blocked")
	return nil
}

func (s *Service) UnblockIP(ctx context.Context, ip string) error {
	key := fmt.Sprintf("ip_blocked:%s", ip)
	err := s.redis.Client().Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("operations: failed to unblock ip: %w", err)
	}
	s.log.Info().Str("ip", ip).Msg("operations: ip unblocked")
	return nil
}

func (s *Service) IsIPBlocked(ctx context.Context, ip string) (bool, error) {
	key := fmt.Sprintf("ip_blocked:%s", ip)
	val, err := s.redis.Client().Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return val == "true", nil
}

func (s *Service) ListBlockedIPs(ctx context.Context) ([]map[string]any, error) {
	keys, err := s.redis.Client().Keys(ctx, "ip_blocked:*").Result()
	if err != nil {
		return nil, err
	}

	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		ttl, _ := s.redis.Client().PTTL(ctx, key).Result()
		ip := key[11:] // strip "ip_blocked:"
		records = append(records, map[string]any{
			"ip":  ip,
			"ttl": ttl.Milliseconds(),
		})
	}
	return records, nil
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
