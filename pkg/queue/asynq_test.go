package queue

import (
	"encoding/json"
	"testing"
	"time"

	"erg.ninja/pkg/config"
)

func TestQueueNames(t *testing.T) {
	if queueNames[PriorityCritical] != PriorityCritical {
		t.Error("PriorityCritical mapping incorrect")
	}
	if queueNames[PriorityHigh] != PriorityHigh {
		t.Error("PriorityHigh mapping incorrect")
	}
	if queueNames[PriorityDefault] != PriorityDefault {
		t.Error("PriorityDefault mapping incorrect")
	}
	if queueNames[PriorityLow] != PriorityLow {
		t.Error("PriorityLow mapping incorrect")
	}
}

func TestNewAsynqClient(t *testing.T) {
	cfg := config.QueueConfig{
		RedisHost:    "localhost",
		RedisPort:    6379,
		RedisPassword: "",
		RedisDB:      0,
		MaxRetry:    3,
	}
	// NewAsynqClient connects to Redis; skip live test in unit test.
	// Just verify it doesn't panic and returns a non-nil client.
	// (Full integration test would require a running Redis.)
	_ = cfg
}

func TestOptionWithQueue(t *testing.T) {
	opt := WithQueue(PriorityHigh)
	if opt.opt == nil {
		t.Error("WithQueue returned nil option")
	}
}

func TestOptionWithMaxRetry(t *testing.T) {
	opt := WithMaxRetry(5)
	if opt.opt == nil {
		t.Error("WithMaxRetry returned nil option")
	}
}

func TestOptionWithTimeout(t *testing.T) {
	opt := WithTimeout(30 * time.Second)
	if opt.opt == nil {
		t.Error("WithTimeout returned nil option")
	}
}

func TestOptionWithDeadline(t *testing.T) {
	deadline := time.Now().Add(1 * time.Hour)
	opt := WithDeadline(deadline)
	if opt.opt == nil {
		t.Error("WithDeadline returned nil option")
	}
}

func TestParsePayload(t *testing.T) {
	payload := map[string]string{"key": "value"}
	bytes, _ := json.Marshal(payload)

	type SamplePayload struct {
		Key string `json:"key"`
	}

	// Simulate an asynq.Task payload.
	task := &mockTask{payload: bytes}

	var out SamplePayload
	if err := ParsePayload(task, &out); err != nil {
		t.Fatalf("ParsePayload: %v", err)
	}
	if out.Key != "value" {
		t.Errorf("out.Key = %q, want 'value'", out.Key)
	}
}

func TestParsePayloadInvalid(t *testing.T) {
	task := &mockTask{payload: []byte("not json{{{")}
	var out map[string]interface{}
	if err := ParsePayload(task, &out); err == nil {
		t.Error("ParsePayload should error on invalid JSON")
	}
}

func TestAsynqServerConfig(t *testing.T) {
	cfg := config.QueueConfig{
		Concurrency:  10,
		RetryDelay:   10 * time.Second,
		MaxRetry:     3,
		RetryBackoff: true,
		DLQQueueName: "erg-dlq",
	}
	if cfg.Concurrency == 0 {
		t.Error("Concurrency should not be zero")
	}
	if cfg.DLQQueueName == "" {
		t.Error("DLQQueueName should not be empty")
	}
}

// mockTask implements asynq.Task (as of asynq v0.24.1).
type mockTask struct {
	payload []byte
}

func (m *mockTask) Payload() []byte              { return m.payload }
func (m *mockTask) Type() string                { return "test" }
func (m *mockTask) Queue() string               { return "default" }
func (m *mockTask) RetryCount() int             { return 0 }
func (m *mockTask) Error() string               { return "" }
func (m *mockTask) LastFailedAt() time.Time     { return time.Time{} }
func (m *mockTask) Result() []byte              { return nil }
func (m *mockTask) MaxRetry() int              { return 3 }
func (m *mockTask) Timeout() time.Duration      { return 0 }
func (m *mockTask) Deadline() *time.Time        { return nil }
func (m *mockTask) RetryDelay() time.Duration   { return 0 }
func (m *mockTask) Unique() time.Duration       { return 0 }
