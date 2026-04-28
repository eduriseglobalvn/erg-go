// Package notifications provides event bus consumer for domain events.
package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"erg.ninja/internal/modules/notifications/entities"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
)

type unsubscribeFunc func()

// EventConsumer subscribes to domain events and dispatches notification jobs.
type EventConsumer struct {
	svc   *Service
	bus   *event.EventBus
	log   *logger.Logger
	done  func() // unsubscribe function
	dones []unsubscribeFunc
}

// EventConsumerOption configures the EventConsumer.
type EventConsumerOption func(*EventConsumer)

// WithEventConsumerLogger sets the logger.
func WithEventConsumerLogger(log *logger.Logger) EventConsumerOption {
	return func(c *EventConsumer) { c.log = log }
}

// NewEventConsumer creates a new event consumer that subscribes to domain events.
func NewEventConsumer(svc *Service, bus *event.EventBus, opts ...EventConsumerOption) *EventConsumer {
	c := &EventConsumer{
		svc: svc,
		bus: bus,
		log: logger.NoOp(),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Topics maps event type → handler configuration.
var eventTopics = map[string]struct {
	Channel  entities.ChannelType
	Subject  string
	Template string
	Priority string
}{
	"crawl.success":        {Channel: entities.ChannelDiscord, Subject: "Crawl thành công", Template: "crawl_success", Priority: "default"},
	"crawl.failed":         {Channel: entities.ChannelDiscord, Subject: "Crawl thất bại", Template: "crawl_failed", Priority: "high"},
	"trending.hot_topic":   {Channel: entities.ChannelDiscord, Subject: "🔥 Topic hot", Template: "hot_topic_alert", Priority: "high"},
	"trending.daily":       {Channel: entities.ChannelEmail, Subject: "Trending Daily", Template: "trending_summary", Priority: "default"},
	"system.alert":         {Channel: entities.ChannelDiscord, Subject: "⚠️ System Alert", Template: "system_alert", Priority: "high"},
	"system.warning":       {Channel: entities.ChannelDiscord, Subject: "⚡ System Warning", Template: "system_warning", Priority: "high"},
	"system.recovery":      {Channel: entities.ChannelDiscord, Subject: "✅ System Recovered", Template: "system_recovery", Priority: "default"},
	"queue.overload":       {Channel: entities.ChannelDiscord, Subject: "🚨 Queue Overload", Template: "queue_overload", Priority: "high"},
	"queue.status":         {Channel: entities.ChannelDiscord, Subject: "📊 Queue Status", Template: "queue_status", Priority: "default"},
	"rss.added":            {Channel: entities.ChannelDiscord, Subject: "✅ RSS Added", Template: "rss_added", Priority: "default"},
	"rss.fetch_error":      {Channel: entities.ChannelDiscord, Subject: "⚠️ RSS Fetch Error", Template: "rss_fetch_error", Priority: "default"},
	"bot.account.linked":   {Channel: entities.ChannelTelegram, Subject: "✅ Account Linked", Template: "account_linked", Priority: "default"},
	"bot.account.unlinked": {Channel: entities.ChannelTelegram, Subject: "🔓 Account Unlinked", Template: "account_unlinked", Priority: "default"},
}

// Start subscribes to all configured event topics.
func (c *EventConsumer) Start(ctx context.Context) error {
	if c.bus == nil {
		c.log.Warn().Msg("event_consumer: no event bus configured, skipping subscriptions")
		return nil
	}

	c.done = nil
	c.dones = nil
	for topic := range eventTopics {
		topic := topic // capture loop variable
		handler := c.makeHandler(topic)
		c.dones = append(c.dones, c.bus.SubscribeLocal(topic, handler))
		if cancel, err := c.bus.Subscribe(ctx, topic, handler); err != nil {
			c.log.Debug().Str("topic", topic).Err(err).Msg("event_consumer: redis subscription unavailable, using local bus only")
		} else {
			c.dones = append(c.dones, cancel)
			c.log.Info().Str("topic", topic).Msg("event_consumer: subscribed via redis")
		}
		c.log.Info().Str("topic", topic).Msg("event_consumer: subscribed")
	}
	c.done = func() {
		for i := len(c.dones) - 1; i >= 0; i-- {
			c.dones[i]()
		}
		c.dones = nil
	}

	c.log.Info().Int("topics", len(eventTopics)).Msg("event_consumer: started")
	return nil
}

// Stop unsubscribes from all event topics.
func (c *EventConsumer) Stop() {
	if c.done != nil {
		c.done()
		c.done = nil
	}
	c.log.Info().Msg("event_consumer: stopped")
}

// makeHandler returns an event handler for the given topic.
func (c *EventConsumer) makeHandler(topic string) func(context.Context, event.EventEnvelope) error {
	cfg, ok := eventTopics[topic]
	if !ok {
		// Default handler for unknown topics.
		return func(ctx context.Context, env event.EventEnvelope) error {
			return c.handleGeneric(ctx, topic, env)
		}
	}

	return func(ctx context.Context, env event.EventEnvelope) error {
		return c.handleEvent(ctx, topic, cfg, env)
	}
}

// handleEvent processes a domain event and sends a notification.
func (c *EventConsumer) handleEvent(ctx context.Context, topic string, cfg struct {
	Channel  entities.ChannelType
	Subject  string
	Template string
	Priority string
}, env event.EventEnvelope) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Parse the event payload into a generic map.
	var data map[string]string
	if env.Payload != nil {
		if err := json.Unmarshal(env.Payload, &data); err != nil {
			c.log.WarnContext(ctx).Err(err).Str("topic", topic).Msg("event_consumer: unmarshal failed, using empty data")
			data = make(map[string]string)
		}
	}

	// Build send event.
	evt := SendEvent{
		Channel:  cfg.Channel,
		Subject:  cfg.Subject,
		Template: cfg.Template,
		Data:     data,
		Priority: cfg.Priority,
	}

	// Try to extract user info from payload.
	if userID, ok := data["user_id"]; ok {
		evt.UserID = userID
	}
	if recipient, ok := data["recipient"]; ok {
		evt.Recipient = recipient
	}
	if body, ok := data["body"]; ok {
		evt.Body = body
	}

	// If no user_id, try common fields.
	if evt.UserID == "" {
		if userID, ok := data["userId"]; ok {
			evt.UserID = userID
		}
	}
	if evt.Recipient == "" {
		if recipient, ok := data["chat_id"]; ok {
			evt.Recipient = recipient
		}
		if webhookURL, ok := data["webhook_url"]; ok {
			evt.Recipient = webhookURL
		}
	}

	if evt.UserID == "" && evt.Recipient == "" {
		c.log.DebugContext(ctx).Str("topic", topic).Msg("event_consumer: no recipient, skipping")
		return nil
	}

	if err := c.svc.SendFromEvent(ctx, evt); err != nil {
		return fmt.Errorf("event_consumer handle %s: %w", topic, err)
	}

	c.log.InfoContext(ctx).Str("topic", topic).Msg("event_consumer: notification sent")
	return nil
}

// handleGeneric handles unknown event topics.
func (c *EventConsumer) handleGeneric(ctx context.Context, topic string, env event.EventEnvelope) error {
	var data map[string]string
	if env.Payload != nil {
		if err := json.Unmarshal(env.Payload, &data); err != nil {
			c.log.WarnContext(ctx).Err(err).Str("topic", topic).Msg("event_consumer: generic payload unmarshal failed")
			data = map[string]string{}
		}
	}

	evt := SendEvent{
		Channel:  entities.ChannelDiscord,
		Subject:  fmt.Sprintf("Event: %s", topic),
		Body:     fmt.Sprintf("Received event: %s\nPayload: %s", topic, string(env.Payload)),
		Priority: "default",
	}

	if userID, ok := data["user_id"]; ok {
		evt.UserID = userID
	}

	return c.svc.SendFromEvent(ctx, evt)
}

// channelType converts a string channel name to an entities.ChannelType.
func channelType(s string) entities.ChannelType {
	switch s {
	case "discord":
		return entities.ChannelDiscord
	case "telegram":
		return entities.ChannelTelegram
	case "whatsapp":
		return entities.ChannelWhatsApp
	case "email":
		return entities.ChannelEmail
	default:
		return entities.ChannelDiscord
	}
}
