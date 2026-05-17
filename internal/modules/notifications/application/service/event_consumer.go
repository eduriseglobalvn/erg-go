// Package notifications provides event bus consumer for domain events.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	entities "erg.ninja/internal/modules/notifications/domain/entity"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
)

type unsubscribeFunc func()

// EventConsumer subscribes to domain events and dispatches notification jobs.
type EventConsumer struct {
	svc     *Service
	bus     *event.EventBus
	log     *logger.Logger
	mu      sync.Mutex
	done    func() // unsubscribe function
	dones   []unsubscribeFunc
	stopped bool
}

type eventSubscription struct {
	topic   string
	handler event.EventHandler
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

	redisCtx, cancelRedis := context.WithCancel(ctx) // #nosec G118 -- stored in c.dones and invoked by Stop/stopAll.

	c.mu.Lock()
	c.done = nil
	c.dones = []unsubscribeFunc{unsubscribeFunc(cancelRedis)}
	c.done = c.stopAll
	c.stopped = false
	c.mu.Unlock()

	subscriptions := make([]eventSubscription, 0, len(eventTopics))
	for topic := range eventTopics {
		topic := topic // capture loop variable
		handler := c.makeHandler(topic)
		c.addDone(c.bus.SubscribeLocal(topic, handler))
		subscriptions = append(subscriptions, eventSubscription{topic: topic, handler: handler})
		c.log.Debug().Str("topic", topic).Msg("event_consumer: subscribed locally")
	}

	if c.bus.RedisSubscriptionsEnabled() {
		c.startRedisSubscriptions(redisCtx, subscriptions)
	} else {
		c.log.Info().Msg("event_consumer: redis subscriptions unavailable, using local bus only")
	}

	c.log.Info().Int("topics", len(eventTopics)).Msg("event_consumer: started")
	return nil
}

func (c *EventConsumer) startRedisSubscriptions(ctx context.Context, subscriptions []eventSubscription) {
	handlers := make(map[string]event.EventHandler, len(subscriptions))
	for _, sub := range subscriptions {
		handlers[sub.topic] = sub.handler
	}

	cancel, err := c.bus.SubscribeMany(ctx, handlers)
	if err != nil {
		c.log.Warn().Err(err).Msg("event_consumer: redis subscriptions unavailable, using local bus only")
		return
	}
	c.addDone(cancel)
	c.log.Info().Int("topics", len(handlers)).Msg("event_consumer: subscribed via redis pubsub")
}

func (c *EventConsumer) addDone(cancel unsubscribeFunc) {
	if cancel == nil {
		return
	}
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		cancel()
		return
	}
	c.dones = append(c.dones, cancel)
	c.mu.Unlock()
}

func (c *EventConsumer) stopAll() {
	c.mu.Lock()
	dones := c.dones
	c.dones = nil
	c.stopped = true
	c.mu.Unlock()

	for i := len(dones) - 1; i >= 0; i-- {
		dones[i]()
	}
}

// Stop unsubscribes from all event topics.
func (c *EventConsumer) Stop() {
	c.mu.Lock()
	done := c.done
	c.done = nil
	c.mu.Unlock()

	if done != nil {
		done()
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

	data := c.payloadData(ctx, topic, env.Payload)

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
	data := c.payloadData(ctx, topic, env.Payload)

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

func (c *EventConsumer) payloadData(ctx context.Context, topic string, payload json.RawMessage) map[string]string {
	data := make(map[string]string)
	if len(payload) == 0 {
		return data
	}

	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		c.log.WarnContext(ctx).Err(err).Str("topic", topic).Msg("event_consumer: payload unmarshal failed, using empty data")
		return data
	}
	for key, value := range raw {
		switch typed := value.(type) {
		case string:
			data[key] = typed
		case nil:
			continue
		default:
			encoded, err := json.Marshal(typed)
			if err != nil {
				continue
			}
			data[key] = string(encoded)
		}
	}
	return data
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
