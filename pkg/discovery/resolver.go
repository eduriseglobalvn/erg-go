package discovery

import (
	"context"
	crypto_rand "crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"google.golang.org/grpc/resolver"
)

// Resolver is a gRPC resolver that queries a discovery Catalog.
type Resolver struct {
	catalog Catalog
	name    string
	tags    []string
	cc      resolver.ClientConn
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// RefreshInterval controls how often the catalog is re-queried.
	RefreshInterval time.Duration
}

// NewResolver returns a discovery-backed gRPC resolver builder.
func NewResolver(catalog Catalog, name string, tags ...string) resolver.Builder {
	return &catalogResolverBuilder{catalog: catalog, name: name, tags: tags}
}

type catalogResolverBuilder struct {
	catalog Catalog
	name    string
	tags    []string
}

func (b *catalogResolverBuilder) Build(
	target resolver.Target,
	cc resolver.ClientConn,
	_ resolver.BuildOptions,
) (resolver.Resolver, error) {
	r := &Resolver{
		catalog:         b.catalog,
		name:            b.name,
		tags:            b.tags,
		cc:              cc,
		stopCh:          make(chan struct{}),
		RefreshInterval: 30 * time.Second,
	}
	if target.Endpoint() != "" {
		r.name = target.Endpoint()
	}
	go r.watch()
	return r, nil
}

func (*catalogResolverBuilder) Scheme() string { return "discovery" }

// watch periodically updates the gRPC ClientConn with healthy instances.
func (r *Resolver) watch() {
	r.wg.Add(1)
	defer r.wg.Done()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initial lookup.
	r.resolve(ctx)

	ticker := time.NewTicker(r.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.resolve(ctx)
		}
	}
}

func (r *Resolver) resolve(ctx context.Context) {
	svcs, err := r.catalog.Find(ctx, r.name, r.tags)
	if err != nil {
		r.cc.ReportError(err)
		return
	}
	addrs := make([]resolver.Address, 0, len(svcs))
	for _, s := range svcs {
		if s.TTL.After(time.Now()) || s.TTL.IsZero() {
			addrs = append(addrs, resolver.Address{
				Addr:       s.Address,
				ServerName: s.Name,
			})
		}
	}
	if len(addrs) == 0 {
		return
	}
	_ = r.cc.UpdateState(resolver.State{Addresses: addrs})
}

// ResolveNow is called by gRPC to force an immediate refresh.
func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {
	r.resolve(context.Background())
}

// Close shuts down the watcher goroutine.
func (r *Resolver) Close() {
	close(r.stopCh)
	r.wg.Wait()
}

// PickFirst returns a random address from the catalog for simple load balancing.
// For production, rely on gRPC's built-in round_robin policy instead:
//
//	grpc.Dial("discovery:///crawler", grpc.WithResolvers(discovery.NewResolver(cat, "crawler")))
func PickFirst(svcs []Service) (Service, error) {
	if len(svcs) == 0 {
		return Service{}, fmt.Errorf("discovery: pick: no instances available")
	}
	// Filter out expired entries.
	now := time.Now()
	valid := make([]Service, 0, len(svcs))
	for _, s := range svcs {
		if s.TTL.After(now) || s.TTL.IsZero() {
			valid = append(valid, s)
		}
	}
	if len(valid) == 0 {
		return Service{}, fmt.Errorf("discovery: pick: all instances expired")
	}
	return valid[randomServiceIndex(len(valid))], nil
}

func randomServiceIndex(length int) int {
	if length <= 1 {
		return 0
	}
	n, err := crypto_rand.Int(crypto_rand.Reader, big.NewInt(int64(length)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(length))
	}
	return int(n.Int64())
}
