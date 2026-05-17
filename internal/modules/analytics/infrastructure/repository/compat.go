package repository

import (
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/analytics/domain/entity"
	"erg.ninja/pkg/database"
)

func analyticsIDFilter(id string) bson.M {
	if objID, ok := database.ParseObjectID(id); ok {
		return bson.M{
			"$or": []bson.M{
				{"_id": id},
				{"_id": objID},
			},
		}
	}
	return bson.M{"_id": id}
}

func analyticsDateRangeFilter(from, to time.Time) bson.M {
	dateRange := bson.M{"$gte": from, "$lte": to}
	return bson.M{
		"$or": []bson.M{
			{"created_at": dateRange},
			{"createdAt": dateRange},
		},
	}
}

func normalizeVisitDoc(doc bson.M) *entities.Visit {
	if len(doc) == 0 {
		return nil
	}

	ipHash := analyticsString(doc, "ip_hash")
	if ipHash == "" {
		ipHash = analyticsString(doc, "ipAddress")
	}

	duration := analyticsInt(doc, "duration_seconds", "durationSeconds")
	pageViews := analyticsInt(doc, "page_views")
	if pageViews == 0 {
		pageViews = 1
	}

	return &entities.Visit{
		ID:              analyticsIDString(doc["_id"]),
		TenantID:        analyticsString(doc, "tenant_id", "tenantId"),
		UserID:          analyticsInt64Ptr(doc, "user_id", "userId"),
		SessionID:       analyticsString(doc, "session_id", "sessionInternalId"),
		PageURL:         analyticsString(doc, "url"),
		Referrer:        analyticsString(doc, "referrer"),
		IPHash:          ipHash,
		UserAgent:       analyticsString(doc, "user_agent", "userAgent"),
		DeviceType:      analyticsString(doc, "device_type", "deviceType"),
		Browser:         analyticsString(doc, "browser"),
		OS:              analyticsString(doc, "os"),
		Country:         analyticsString(doc, "country"),
		City:            analyticsString(doc, "city"),
		Region:          analyticsString(doc, "region"),
		Timezone:        analyticsString(doc, "timezone"),
		EntityType:      analyticsString(doc, "entity_type", "entityType"),
		EntityID:        analyticsString(doc, "entity_id", "entityId"),
		DurationSeconds: duration,
		PageViews:       pageViews,
		CreatedAt:       analyticsTime(doc, "created_at", "createdAt"),
		UpdatedAt:       analyticsTime(doc, "updated_at", "updatedAt"),
	}
}

func normalizeEventDoc(doc bson.M) *entities.Event {
	if len(doc) == 0 {
		return nil
	}

	eventType := analyticsString(doc, "event_type", "eventType")
	eventName := analyticsString(doc, "event_name", "eventName")
	if eventName == "" {
		eventName = eventType
	}

	return &entities.Event{
		ID:         analyticsIDString(doc["_id"]),
		TenantID:   analyticsString(doc, "tenant_id", "tenantId"),
		SessionID:  analyticsString(doc, "session_id", "sessionInternalId"),
		UserID:     analyticsInt64Ptr(doc, "user_id", "userId"),
		EventName:  eventName,
		EventType:  eventType,
		Properties: analyticsMap(doc, "properties", "metadata"),
		CreatedAt:  analyticsTime(doc, "created_at", "createdAt"),
	}
}

func analyticsIDString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bson.ObjectID:
		return v.Hex()
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func analyticsString(doc bson.M, keys ...string) string {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func analyticsInt(doc bson.M, keys ...string) int {
	v, ok := analyticsInt64(doc, keys...)
	if !ok {
		return 0
	}
	return int(v)
}

func analyticsInt64Ptr(doc bson.M, keys ...string) *int64 {
	v, ok := analyticsInt64(doc, keys...)
	if !ok {
		return nil
	}
	value := v
	return &value
}

func analyticsInt64(doc bson.M, keys ...string) (int64, bool) {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case int:
			return int64(v), true
		case int32:
			return int64(v), true
		case int64:
			return v, true
		case float32:
			return int64(v), true
		case float64:
			return int64(v), true
		case string:
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func analyticsMap(doc bson.M, keys ...string) map[string]interface{} {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case map[string]interface{}:
			return v
		case bson.M:
			return map[string]interface{}(v)
		}
	}
	return nil
}

func analyticsTime(doc bson.M, keys ...string) time.Time {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case time.Time:
			return v.UTC()
		case *time.Time:
			if v != nil {
				return v.UTC()
			}
		case bson.DateTime:
			return v.Time().UTC()
		case string:
			if parsed, err := time.Parse(time.RFC3339, v); err == nil {
				return parsed.UTC()
			}
			if parsed, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
				return parsed.UTC()
			}
		case int64:
			return time.UnixMilli(v).UTC()
		}
	}
	return time.Time{}
}
