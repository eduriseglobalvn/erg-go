// Package tenant provides Asynq queue wrappers with per-tenant queue isolation.
package tenant

import (
	"context"
	"encoding/json"
	"fmt"

	"erg.ninja/pkg/queue"

	_ "github.com/hibiken/asynq"
)

// TenantAsynqClient wraps an Asynq client to route jobs into per-tenant queues.
// By default jobs go to shared queues; when a tenant ID is present in context
// (or explicitly specified), they are routed to "{base_queue}_{tenant_id}".
//
// DLQ naming: failed jobs for a tenant land in "dlq_{tenant_id}" so that
// tenant A's failures never pollute tenant B's dead-letter queue.
type TenantAsynqClient struct {
	*queue.AsynqClient
	defaultTenantID string
	dlqQueueName    string
}

// NewTenantAsynqClient wraps an existing AsynqClient for multi-tenant use.
func NewTenantAsynqClient(client *queue.AsynqClient, defaultTenantID string) *TenantAsynqClient {
	return &TenantAsynqClient{
		AsynqClient:     client,
		defaultTenantID: defaultTenantID,
		dlqQueueName:    "erg-dlq",
	}
}

// EnqueueTenant enqueues a job into a tenant-scoped queue.
// The queue name is "{base_queue}_{tenant_id}" and the DLQ is "dlq_{tenant_id}".
//
// The tenant ID is resolved in the following order:
//
//  1. Explicit tenantID parameter
//  2. Tenant ID from ctx (via pkg/tenant context)
//  3. Configured default tenant ID
//
// If tenantDedicatedQueue is false, all jobs go to the shared queue regardless
// of tenant.
func (c *TenantAsynqClient) EnqueueTenant(
	ctx context.Context,
	jobType string,
	payload interface{},
	tenantDedicatedQueue bool,
	opts ...queue.Option,
) (string, error) {
	tenantID := resolveEnqueueTenant(ctx, c.defaultTenantID)

	options := opts
	if tenantDedicatedQueue {
		// Route to per-tenant queue.
		qName := fmt.Sprintf("%s_%s", c.Config().QueueName, tenantID)
		options = append([]queue.Option{queue.WithQueue(qName)}, options...)
	}

	return c.AsynqClient.Enqueue(ctx, jobType, payload, options...)
}

// EnqueueToTenant is a convenience alias for EnqueueTenant with dedicated queues enabled.
func (c *TenantAsynqClient) EnqueueToTenant(
	ctx context.Context,
	jobType string,
	payload interface{},
	opts ...queue.Option,
) (string, error) {
	return c.EnqueueTenant(ctx, jobType, payload, true, opts...)
}

// EnqueueWithTenantCtx is like EnqueueTenant but always extracts tenant from ctx.
func (c *TenantAsynqClient) EnqueueWithTenantCtx(
	ctx context.Context,
	jobType string,
	payload interface{},
	opts ...queue.Option,
) (string, error) {
	return c.EnqueueTenant(ctx, jobType, payload, true, opts...)
}

// TenantJobPayload is embedded in all tenant-aware job payloads.
// It carries the tenant ID through the Asynq queue so that workers can
// build the correct tenant-scoped context without relying on the HTTP ctx.
type TenantJobPayload struct {
	TenantID string          `json:"tenant_id"`
	Payload  json.RawMessage `json:"payload"`
}

// WrapPayload injects the tenant ID into a job payload and returns a
// TenantJobPayload that can be serialised and enqueued.
func WrapPayload(ctx context.Context, payload interface{}) ([]byte, error) {
	tenantID := FromContext(ctx)
	if tenantID == "" {
		tenantID = "default"
	}
	inner, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("tenant: marshal payload: %w", err)
	}
	wrapped := TenantJobPayload{
		TenantID: tenantID,
		Payload:  inner,
	}
	return json.Marshal(wrapped)
}

// UnwrapPayload extracts the inner payload from a TenantJobPayload.
func UnwrapPayload(data []byte, out interface{}) error {
	var wrapped TenantJobPayload
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return fmt.Errorf("tenant: unwrap payload: %w", err)
	}
	return json.Unmarshal(wrapped.Payload, out)
}

// ExtractTenantFromPayload reads the tenant ID from raw Asynq task payload bytes.
func ExtractTenantFromPayload(data []byte) string {
	var wrapped TenantJobPayload
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return ""
	}
	return wrapped.TenantID
}

// DLQQueueName returns the dead-letter queue name for a specific tenant.
func DLQQueueName(tenantID string) string {
	return fmt.Sprintf("dlq_%s", tenantID)
}

// TenantQueueName returns the queue name for a specific tenant and base queue.
func TenantQueueName(baseQueue, tenantID string) string {
	return fmt.Sprintf("%s_%s", baseQueue, tenantID)
}

// resolveEnqueueTenant resolves the tenant ID for enqueueing.
func resolveEnqueueTenant(ctx context.Context, defaultID string) string {
	id := FromContext(ctx)
	if id != "" {
		return id
	}
	if defaultID != "" {
		return defaultID
	}
	return "default"
}

// TenantAsynqServer wraps an Asynq server to add per-tenant queue configuration.
// It configures per-tenant queues and DLQ queues in the Asynq server.
type TenantAsynqServer struct {
	*queue.AsynqServer
	tenantQueues  map[string]int // tenantID -> priority weight
	defaultWeight int
}

// NewTenantAsynqServer creates a TenantAsynqServer with per-tenant queue weights.
// tenantQueues maps tenant ID to priority (higher = more Asynq workers).
func NewTenantAsynqServer(
	server *queue.AsynqServer,
	tenantQueues map[string]int,
	defaultWeight int,
) *TenantAsynqServer {
	return &TenantAsynqServer{
		AsynqServer:   server,
		tenantQueues:  tenantQueues,
		defaultWeight: defaultWeight,
	}
}

// TenantWeight returns the configured priority weight for a tenant,
// or the default weight if not explicitly configured.
func (s *TenantAsynqServer) TenantWeight(tenantID string) int {
	if w, ok := s.tenantQueues[tenantID]; ok {
		return w
	}
	return s.defaultWeight
}

// DefaultQueues returns the standard Asynq queue priority map.
func DefaultQueues() map[string]int {
	return map[string]int{
		"critical": 10,
		"high":     7,
		"default":  5,
		"low":      2,
	}
}

// AddTenantQueuesToConfig extends an Asynq Config with per-tenant queues.
// Call this when building the asynq.Config before starting the server.
func AddTenantQueuesToConfig(queues map[string]int, baseQueues map[string]int) map[string]int {
	if baseQueues == nil {
		baseQueues = DefaultQueues()
	}
	for tenantID, weight := range queues {
		qName := TenantQueueName("default", tenantID)
		baseQueues[qName] = weight
		// Also add the per-tenant DLQ.
		baseQueues[DLQQueueName(tenantID)] = 1
	}
	return baseQueues
}
