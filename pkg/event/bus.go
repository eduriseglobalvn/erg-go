// Package event provides an in-process event bus with optional Redis pub/sub
// for cross-service event propagation.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/logger"
)

// EventEnvelope wraps every event published through the bus.
type EventEnvelope struct {
	EventType     string          `json:"event_type"`
	SourceService string          `json:"source_service"`
	Payload       json.RawMessage `json:"payload"`
	Timestamp     time.Time       `json:"timestamp"`
	TraceID       string          `json:"trace_id,omitempty"`
}

// EventHandler is the function signature for event subscribers.
type EventHandler func(ctx context.Context, envelope EventEnvelope) error

// localSubscription holds a handler registered for in-process events.
type localSubscription struct {
	handler  EventHandler
	filter   string // event type filter
	cancel   context.CancelFunc
	cancelID int // unique ID to distinguish cancel functions
}

// EventBus provides both in-process (local) and Redis-backed (cross-service) pub/sub.
type EventBus struct {
	localSubs    map[string][]localSubscription // key: event type
	mu           sync.RWMutex
	log          *logger.Logger
	redis        *cache.RedisClient
	serviceName  string
	redisSubs    bool
	redisOnce    sync.Once
	nextCancelID int
}

// BusOption configures an EventBus.
type BusOption func(*EventBus)

// WithBusLogger sets the logger for the event bus.
func WithBusLogger(log *logger.Logger) BusOption {
	return func(b *EventBus) {
		b.log = log
	}
}

// WithRedisBackend enables Redis pub/sub for cross-service events.
func WithRedisBackend(redis *cache.RedisClient) BusOption {
	return func(b *EventBus) {
		b.redis = redis
	}
}

// WithRedisSubscriptions toggles Redis-backed cross-service subscriptions.
func WithRedisSubscriptions(enabled bool) BusOption {
	return func(b *EventBus) {
		b.redisSubs = enabled
	}
}

// NewEventBus creates a new event bus instance.
func NewEventBus(serviceName string, opts ...BusOption) *EventBus {
	b := &EventBus{
		localSubs:   make(map[string][]localSubscription),
		log:         logger.NoOp(),
		serviceName: serviceName,
		redisSubs:   true,
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

// RedisSubscriptionsEnabled reports whether cross-service Redis subscriptions are active.
func (b *EventBus) RedisSubscriptionsEnabled() bool {
	return b != nil && b.redis != nil && b.redisSubs
}

// Publish publishes an event locally (synchronous, in-process) and optionally
// to Redis for cross-service consumption.
func (b *EventBus) Publish(ctx context.Context, eventType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: marshal payload: %w", err)
	}

	envelope := EventEnvelope{
		EventType:     eventType,
		SourceService: b.serviceName,
		Payload:       payloadBytes,
		Timestamp:     time.Now().UTC(),
	}

	// Propagate local subscribers synchronously.
	b.publishLocal(ctx, envelope)

	// If Redis is configured, also publish cross-service.
	if b.redis != nil {
		envelopeBytes, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("event: marshal envelope: %w", err)
		}
		if err := b.redis.Publish(ctx, channelName(eventType), envelopeBytes); err != nil {
			b.log.Warn().Err(err).Str("event_type", eventType).Msg("redis publish failed")
			// Don't return error — local delivery already succeeded.
		}
	}

	return nil
}

// PublishLocal publishes an event only to in-process subscribers (no Redis).
func (b *EventBus) PublishLocal(ctx context.Context, eventType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: marshal payload: %w", err)
	}

	envelope := EventEnvelope{
		EventType:     eventType,
		SourceService: b.serviceName,
		Payload:       payloadBytes,
		Timestamp:     time.Now().UTC(),
	}

	b.publishLocal(ctx, envelope)
	return nil
}

// publishLocal delivers an event envelope to all matching local subscribers.
func (b *EventBus) publishLocal(ctx context.Context, envelope EventEnvelope) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs, ok := b.localSubs[envelope.EventType]
	if !ok {
		return
	}

	for _, sub := range subs {
		subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := sub.handler(subCtx, envelope); err != nil {
			b.log.Error().Err(err).
				Str("event_type", envelope.EventType).
				Str("source", envelope.SourceService).
				Msg("local subscriber error")
		}
	}
}

// SubscribeLocal registers a handler for in-process events of the given type.
// It returns a cancel function that removes the subscription.
func (b *EventBus) SubscribeLocal(eventType string, handler EventHandler) (cancel func()) {
	_, cancelFn := context.WithCancel(context.Background())

	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextCancelID++
	sub := localSubscription{
		handler:  handler,
		filter:   eventType,
		cancel:   cancelFn,
		cancelID: b.nextCancelID,
	}
	b.localSubs[eventType] = append(b.localSubs[eventType], sub)

	cancelID := b.nextCancelID
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		cancelFn()
		subs := b.localSubs[eventType]
		for i, s := range subs {
			if s.cancelID == cancelID {
				b.localSubs[eventType] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// Subscribe registers a handler for Redis-backed cross-service events.
// It subscribes to the Redis channel for the given event type and returns
// a cancel function to unsubscribe.
func (b *EventBus) Subscribe(ctx context.Context, eventType string, handler EventHandler) (cancel func(), err error) {
	return b.SubscribeMany(ctx, map[string]EventHandler{eventType: handler})
}

// SubscribeMany registers handlers for multiple Redis-backed cross-service events
// using a single Pub/Sub connection. go-redis ChannelWithSubscriptions keeps the
// connection healthy with periodic PINGs and automatically resubscribes after
// reconnects, which is safer for remote TLS Redis/Valkey providers.
func (b *EventBus) SubscribeMany(ctx context.Context, handlers map[string]EventHandler) (cancel func(), err error) {
	if b.redis == nil {
		return nil, fmt.Errorf("event: subscribe: Redis backend not configured")
	}
	if !b.redisSubs {
		return nil, fmt.Errorf("event: subscribe: Redis subscriptions disabled")
	}
	if len(handlers) == 0 {
		return func() {}, nil
	}

	eventTypes := make([]string, 0, len(handlers))
	for eventType, handler := range handlers {
		if eventType == "" || handler == nil {
			continue
		}
		eventTypes = append(eventTypes, eventType)
	}
	if len(eventTypes) == 0 {
		return func() {}, nil
	}
	sort.Strings(eventTypes)

	channelToEvent := make(map[string]string, len(eventTypes))
	channels := make([]string, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		channel := channelName(eventType)
		channelToEvent[channel] = eventType
		channels = append(channels, channel)
	}

	subCtx, cancelCtx := context.WithCancel(ctx)
	pubsub, stop := b.redis.Subscribe(subCtx, channels...)
	messages := pubsub.ChannelWithSubscriptions(
		redis.WithChannelSize(1024),
		redis.WithChannelHealthCheckInterval(15*time.Second),
	)

	// Start a goroutine to handle incoming messages.
	go func() {
		defer stop()
		for {
			select {
			case <-subCtx.Done():
				return
			case raw, ok := <-messages:
				if !ok {
					if subCtx.Err() == nil {
						b.log.Warn().Strs("channels", channels).Msg("redis pubsub channel closed")
					}
					return
				}
				switch msg := raw.(type) {
				case *redis.Subscription:
					b.log.Debug().
						Str("kind", msg.Kind).
						Str("channel", msg.Channel).
						Int("count", msg.Count).
						Msg("redis pubsub subscription status")
				case *redis.Message:
					eventType, ok := channelToEvent[msg.Channel]
					if !ok {
						b.log.Debug().Str("channel", msg.Channel).Msg("redis pubsub ignored unknown channel")
						continue
					}
					handler := handlers[eventType]
					if handler == nil {
						continue
					}
					b.handleRedisMessage(subCtx, eventType, msg.Payload, handler)
				default:
					b.log.Debug().Str("message_type", fmt.Sprintf("%T", raw)).Msg("redis pubsub ignored unknown message type")
				}
			}
		}
	}()

	return func() {
		cancelCtx()
		stop()
	}, nil
}

func (b *EventBus) handleRedisMessage(ctx context.Context, eventType, payload string, handler EventHandler) {
	var envelope EventEnvelope
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		b.log.Error().Err(err).Str("event_type", eventType).Str("payload", payload).Msg("unmarshal envelope")
		return
	}
	if envelope.SourceService == b.serviceName {
		return
	}

	handlerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := handler(handlerCtx, envelope); err != nil {
		b.log.Error().Err(err).
			Str("event_type", envelope.EventType).
			Str("source", envelope.SourceService).
			Msg("redis subscriber error")
	}
}

// UnsubscribeAll removes all local subscriptions.
func (b *EventBus) UnsubscribeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, subs := range b.localSubs {
		for _, sub := range subs {
			sub.cancel()
		}
	}
	b.localSubs = make(map[string][]localSubscription)
}

// channelName returns the Redis channel name for an event type.
func channelName(eventType string) string {
	return "erg:events:" + eventType
}

// MarshalPayload serializes a payload into JSON RawMessage.
func MarshalPayload(v interface{}) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("event: marshal: %w", err)
	}
	return b, nil
}

// UnmarshalPayload deserializes an event envelope payload into the target type.
func UnmarshalPayload(envelope EventEnvelope, out interface{}) error {
	if err := json.Unmarshal(envelope.Payload, out); err != nil {
		return fmt.Errorf("event: unmarshal payload: %w", err)
	}
	return nil
}

// SubscribeByReflection dynamically calls a method on a receiver that matches
// the event type name (e.g., HandleBotMessage for "bot.message").
// This is an optional convenience for event-driven service patterns and should
// not be used on hot paths because it relies on reflection and per-message allocation.
func (b *EventBus) SubscribeByReflection(receiver interface{}, opts ...BusOption) error {
	val := reflect.ValueOf(receiver)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("event: receiver must be a pointer")
	}
	val = val.Elem()
	typ := val.Type()

	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		// method.Name is a string; all exported method names are valid.
		if len(method.Name) < 7 || method.Name[:6] != "Handle" {
			continue
		}
		eventType := toSnakeCase(method.Name[6:]) // e.g. "BotMessage" → "bot.message"

		wrapper := func(ctx context.Context, envelope EventEnvelope) error {
			methodType := method.Type
			if methodType.NumIn() < 2 {
				return nil
			}
			payloadType := methodType.In(1)
			payload := reflect.New(payloadType)
			if err := json.Unmarshal(envelope.Payload, payload.Interface()); err != nil {
				return err
			}
			results := method.Func.Call([]reflect.Value{
				val,
				reflect.ValueOf(ctx),
				payload.Elem(),
			})
			if len(results) > 0 && !results[0].IsNil() {
				return results[0].Interface().(error)
			}
			return nil
		}

		b.SubscribeLocal(eventType, wrapper)
	}

	return nil
}

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, c+32) // lowercase
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}
