package notification

import (
	"context"
	"strings"
	"testing"
	"time"
)

type mockProvider struct {
	name    string
	channel ChannelType
	rate    int
	sendErr error
}

func (m *mockProvider) Send(ctx context.Context, msg *Notification) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	return nil
}
func (m *mockProvider) Supports(channel ChannelType) bool { return channel == m.channel }
func (m *mockProvider) Name() string                      { return m.name }
func (m *mockProvider) RateLimit() (int, time.Duration)   { return m.rate, 30 * time.Second }

func TestChannelType(t *testing.T) {
	if ChannelDiscord != "discord" {
		t.Errorf("ChannelDiscord = %q, want 'discord'", ChannelDiscord)
	}
	if ChannelTelegram != "telegram" {
		t.Errorf("ChannelTelegram = %q, want 'telegram'", ChannelTelegram)
	}
	if ChannelWhatsApp != "whatsapp" {
		t.Errorf("ChannelWhatsApp = %q, want 'whatsapp'", ChannelWhatsApp)
	}
	if ChannelEmail != "email" {
		t.Errorf("ChannelEmail = %q, want 'email'", ChannelEmail)
	}
}

func TestNotificationStatus(t *testing.T) {
	if StatusPending != "pending" {
		t.Errorf("StatusPending = %q, want 'pending'", StatusPending)
	}
}

func TestNotifierProviderInterface(t *testing.T) {
	p := &mockProvider{name: "discord", channel: ChannelDiscord, rate: 60}
	if !p.Supports(ChannelDiscord) {
		t.Error("discord provider should support discord channel")
	}
	if p.Supports(ChannelEmail) {
		t.Error("discord provider should not support email channel")
	}
	if p.Name() != "discord" {
		t.Errorf("Name() = %q, want 'discord'", p.Name())
	}
	rpm, retry := p.RateLimit()
	if rpm != 60 {
		t.Errorf("RateLimit rpm = %d, want 60", rpm)
	}
	if retry != 30*time.Second {
		t.Errorf("RateLimit retry = %v, want 30s", retry)
	}
}

func TestProviderRegistry(t *testing.T) {
	discord := &mockProvider{name: "discord", channel: ChannelDiscord, rate: 60}
	email := &mockProvider{name: "email", channel: ChannelEmail, rate: 100}

	registry := NewProviderRegistry(discord, email)

	if got := registry.Get(ChannelDiscord); got == nil {
		t.Error("Get(ChannelDiscord) returned nil")
	}
	if got := registry.Get(ChannelEmail); got == nil {
		t.Error("Get(ChannelEmail) returned nil")
	}
	if got := registry.Get(ChannelTelegram); got != nil {
		t.Error("Get(ChannelTelegram) should return nil")
	}
}

func TestProviderRegistryRegister(t *testing.T) {
	registry := NewProviderRegistry()
	registry.Register(&mockProvider{name: "slack", channel: ChannelSlack, rate: 50})
	if registry.Get(ChannelSlack) == nil {
		t.Error("after Register, Get should return the provider")
	}
}

func TestDispatch(t *testing.T) {
	registry := NewProviderRegistry(&mockProvider{name: "discord", channel: ChannelDiscord, rate: 60})
	msg := &Notification{
		ID:      "msg-1",
		Channel: ChannelDiscord,
		Body:    "Hello!",
		Status:  StatusPending,
	}

	err := Dispatch(context.Background(), registry, msg)
	if err != nil {
		t.Errorf("Dispatch: %v", err)
	}
}

func TestDispatchNoProvider(t *testing.T) {
	registry := NewProviderRegistry()
	msg := &Notification{ID: "msg-2", Channel: ChannelTelegram, Body: "Hello!"}
	err := Dispatch(context.Background(), registry, msg)
	if err == nil {
		t.Error("expected error when no provider is registered")
	}
	var noProviderErr *NoProviderError
	if _, ok := err.(*NoProviderError); !ok {
		t.Errorf("expected NoProviderError, got %T", err)
	}
	_ = noProviderErr
}

func TestNoProviderError(t *testing.T) {
	err := &NoProviderError{Channel: ChannelWhatsApp}
	if !strings.Contains(err.Error(), "whatsapp") {
		t.Errorf("Error() should mention channel: %s", err.Error())
	}
}

func TestRetryPolicyNextDelay(t *testing.T) {
	policy := DefaultRetryPolicy

	delay0 := policy.NextDelay(0)
	if delay0 != 30*time.Second {
		t.Errorf("delay for attempt 0 = %v, want 30s", delay0)
	}

	delay1 := policy.NextDelay(1)
	if delay1 != 60*time.Second {
		t.Errorf("delay for attempt 1 = %v, want 60s", delay1)
	}

	delay2 := policy.NextDelay(2)
	if delay2 != 120*time.Second {
		t.Errorf("delay for attempt 2 = %v, want 120s", delay2)
	}
}

func TestRetryPolicyMaxDelay(t *testing.T) {
	policy := RetryPolicy{
		MaxRetries:    10,
		InitialDelay:  1 * time.Second,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 10.0,
	}

	// Exponential backoff should cap at MaxDelay.
	delay := policy.NextDelay(5)
	if delay > policy.MaxDelay {
		t.Errorf("delay should not exceed MaxDelay: got %v, max %v", delay, policy.MaxDelay)
	}
}

func TestDigestProvider(t *testing.T) {
	// Verify DigestNotification can be constructed.
	digest := DigestNotification{
		Notifications: []Notification{
			{ID: "msg-1", Body: "item 1"},
			{ID: "msg-2", Body: "item 2"},
		},
		Subject:   "Daily Digest",
		Summary:   "2 new items",
		Channel:   ChannelEmail,
		Recipient: "alice@example.com",
	}

	if len(digest.Notifications) != 2 {
		t.Errorf("Notifications count = %d, want 2", len(digest.Notifications))
	}
	if digest.Subject != "Daily Digest" {
		t.Errorf("Subject = %q, want 'Daily Digest'", digest.Subject)
	}
}
