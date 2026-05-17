package event

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"erg.ninja/pkg/cache"
)

func TestEventEnvelope(t *testing.T) {
	payload := map[string]string{"key": "value"}
	payloadBytes, _ := json.Marshal(payload)

	envelope := EventEnvelope{
		EventType:     "test.event",
		SourceService: "test-service",
		Payload:       payloadBytes,
		Timestamp:     time.Now().UTC(),
	}

	if envelope.EventType != "test.event" {
		t.Errorf("EventType = %q, want 'test.event'", envelope.EventType)
	}
	if envelope.SourceService != "test-service" {
		t.Errorf("SourceService = %q, want 'test-service'", envelope.SourceService)
	}
}

func TestNewEventBus(t *testing.T) {
	bus := NewEventBus("test-service")
	if bus == nil {
		t.Fatal("NewEventBus returned nil")
	}
	if bus.serviceName != "test-service" {
		t.Errorf("serviceName = %q, want 'test-service'", bus.serviceName)
	}
}

func TestEventBusPublishLocal(t *testing.T) {
	bus := NewEventBus("test-service")
	called := false

	cancel := bus.SubscribeLocal("user.created", func(ctx context.Context, envelope EventEnvelope) error {
		called = true
		if envelope.EventType != "user.created" {
			t.Errorf("EventType = %q, want 'user.created'", envelope.EventType)
		}
		return nil
	})
	defer cancel()

	err := bus.PublishLocal(context.Background(), "user.created", map[string]string{"user_id": "123"})
	if err != nil {
		t.Fatalf("PublishLocal: %v", err)
	}

	if !called {
		t.Error("subscriber was not called")
	}
}

func TestEventBusSubscribeLocalCancel(t *testing.T) {
	bus := NewEventBus("test-service")
	callCount := 0

	cancel := bus.SubscribeLocal("ping.event", func(ctx context.Context, envelope EventEnvelope) error {
		callCount++
		return nil
	})

	bus.PublishLocal(context.Background(), "ping.event", nil)
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	cancel()
	bus.PublishLocal(context.Background(), "ping.event", nil)
	if callCount != 1 {
		t.Errorf("after cancel, callCount = %d, want 1", callCount)
	}
}

func TestEventBusUnsubscribeAll(t *testing.T) {
	bus := NewEventBus("test-service")
	callCount := 0

	bus.SubscribeLocal("event.a", func(ctx context.Context, envelope EventEnvelope) error {
		callCount++
		return nil
	})
	bus.SubscribeLocal("event.b", func(ctx context.Context, envelope EventEnvelope) error {
		callCount++
		return nil
	})

	bus.UnsubscribeAll()
	bus.PublishLocal(context.Background(), "event.a", nil)
	bus.PublishLocal(context.Background(), "event.b", nil)

	if callCount != 0 {
		t.Errorf("after UnsubscribeAll, callCount = %d, want 0", callCount)
	}
}

func TestMarshalPayload(t *testing.T) {
	payload := map[string]int{"count": 42}
	msg, err := MarshalPayload(payload)
	if err != nil {
		t.Fatalf("MarshalPayload: %v", err)
	}
	var out map[string]int
	if err := json.Unmarshal(msg, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["count"] != 42 {
		t.Errorf("count = %d, want 42", out["count"])
	}
}

func TestUnmarshalPayload(t *testing.T) {
	payload := map[string]string{"name": "alice"}
	bytes, _ := json.Marshal(payload)
	envelope := EventEnvelope{Payload: json.RawMessage(bytes)}

	var out map[string]string
	if err := UnmarshalPayload(envelope, &out); err != nil {
		t.Fatalf("UnmarshalPayload: %v", err)
	}
	if out["name"] != "alice" {
		t.Errorf("name = %q, want 'alice'", out["name"])
	}
}

func TestChannelName(t *testing.T) {
	ch := channelName("user.created")
	if ch != "erg:events:user.created" {
		t.Errorf("channelName = %q, want 'erg:events:user.created'", ch)
	}
}

func TestToSnakeCase(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"BotMessage", "bot_message"},
		{"UserCreated", "user_created"},
		{"RSSFeed", "r_s_s_feed"},
		{"Simple", "simple"},
	}
	for _, c := range cases {
		got := toSnakeCase(c.input)
		if got != c.expected {
			t.Errorf("toSnakeCase(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestEventBusSubscribeWithoutRedis(t *testing.T) {
	bus := NewEventBus("test-service")
	_, err := bus.Subscribe(context.Background(), "some.event", func(ctx context.Context, envelope EventEnvelope) error {
		return nil
	})
	if err == nil {
		t.Error("Subscribe should fail without Redis backend")
	}
}

func TestEventBusSubscribeManyWithoutRedis(t *testing.T) {
	bus := NewEventBus("test-service")
	_, err := bus.SubscribeMany(context.Background(), map[string]EventHandler{
		"some.event": func(ctx context.Context, envelope EventEnvelope) error { return nil },
	})
	if err == nil {
		t.Error("SubscribeMany should fail without Redis backend")
	}
}

func TestEventBusRedisSubscriptionsDisabled(t *testing.T) {
	bus := NewEventBus(
		"test-service",
		WithRedisBackend(&cache.RedisClient{}),
		WithRedisSubscriptions(false),
	)

	if bus.RedisSubscriptionsEnabled() {
		t.Fatal("RedisSubscriptionsEnabled should be false when redis subscriptions are disabled")
	}

	_, err := bus.SubscribeMany(context.Background(), map[string]EventHandler{
		"some.event": func(ctx context.Context, envelope EventEnvelope) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("SubscribeMany error = %v, want disabled error", err)
	}
}
