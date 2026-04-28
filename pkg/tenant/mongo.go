// Package tenant provides MongoDB collection wrappers with per-tenant isolation.
package tenant

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"erg.ninja/pkg/database"
)

// TenantAwareDB is the interface implemented by database clients that support
// tenant-scoped collection access.
type TenantAwareDB interface {
	// TenantCollection returns a collection whose name is prefixed with the
	// tenant ID from the context (e.g. "acme_crawl_histories").
	// Falls back to the shared collection if no tenant is in context.
	TenantCollection(ctx context.Context, name string) *mongo.Collection
	// TenantCollectionFor returns a collection for an explicitly-specified tenant,
	// bypassing context. Useful in background workers where the tenant is known
	// from the job payload.
	TenantCollectionFor(tenantID, name string) *mongo.Collection
	// SharedCollection returns the unprefixed (shared) collection.
	SharedCollection(name string) *mongo.Collection
}

// TenantMongoClient wraps a database.MongoClient with tenant-scoped collection access.
type TenantMongoClient struct {
	*database.MongoClient
	mode      IsolationMode
	defaultID string
}

// NewTenantMongoClient wraps an existing MongoClient with tenant isolation support.
func NewTenantMongoClient(client *database.MongoClient, mode IsolationMode, defaultID string) *TenantMongoClient {
	if defaultID == "" {
		defaultID = "default"
	}
	return &TenantMongoClient{
		MongoClient: client,
		mode:        mode,
		defaultID:   defaultID,
	}
}

// TenantCollection returns the tenant-scoped or shared collection depending on
// the isolation mode and the presence of a tenant ID in ctx.
func (m *TenantMongoClient) TenantCollection(ctx context.Context, name string) *mongo.Collection {
	if m.mode == IsolationField {
		// All tenants share the collection; caller must add tenant_id filter to queries.
		return m.MongoClient.Collection(name)
	}
	// Per-tenant collection mode: prefix the name with the tenant ID.
	id := FromContext(ctx)
	if id == "" {
		id = m.defaultID
	}
	return m.TenantCollectionFor(id, name)
}

// TenantCollectionFor returns a collection scoped to a specific tenant ID,
// bypassing context. This is the preferred method in background workers where
// the tenant is read from the job payload rather than the HTTP context.
func (m *TenantMongoClient) TenantCollectionFor(tenantID, name string) *mongo.Collection {
	if m.mode == IsolationField {
		return m.MongoClient.Collection(name)
	}
	prefixed := fmt.Sprintf("%s_%s", tenantID, name)
	return m.MongoClient.Collection(prefixed)
}

// SharedCollection returns the unprefixed collection shared across all tenants.
func (m *TenantMongoClient) SharedCollection(name string) *mongo.Collection {
	return m.MongoClient.Collection(name)
}

// CollectionNameFor returns the effective collection name for a given tenant,
// useful for logging and migrations. Returns the prefixed name in collection
// mode, or the raw name in field mode.
func (m *TenantMongoClient) CollectionNameFor(tenantID, name string) string {
	if m.mode == IsolationField {
		return name
	}
	return fmt.Sprintf("%s_%s", tenantID, name)
}

// TenantIndexFilter returns a MongoDB filter document that selects only documents
// belonging to the given tenant. Use this when operating in IsolationField mode.
func TenantIndexFilter(tenantID string) map[string]interface{} {
	return map[string]interface{}{
		"tenant_id": tenantID,
	}
}

// WithTenantFilter extends an existing filter map with the tenant ID field.
// If tenantID is empty, returns the original filter unchanged.
func WithTenantFilter(tenantID string, filter map[string]interface{}) map[string]interface{} {
	if tenantID == "" {
		return filter
	}
	if filter == nil {
		return TenantIndexFilter(tenantID)
	}
	result := make(map[string]interface{}, len(filter)+1)
	for k, v := range filter {
		result[k] = v
	}
	result["tenant_id"] = tenantID
	return result
}
