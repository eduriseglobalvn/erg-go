// Package dedup provides SimHash-based content deduplication using FNV-1a hashing
// and Hamming distance comparison for near-duplicate detection.
package dedup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"erg.ninja/pkg/logger"
)

// DefaultHammingThreshold is the maximum Hamming distance for two fingerprints to be considered duplicates.
const DefaultHammingThreshold = 6

// Fingerprint represents a content fingerprint: 64-bit SimHash + SHA-256 exact hash.
type Fingerprint struct {
	SimHash uint64   `json:"simhash"`
	SHA256  [32]byte `json:"sha256"` // exact duplicate detection
}

// Entry is a stored fingerprint with its metadata.
type Entry struct {
	Fingerprint Fingerprint `json:"fingerprint"`
	URL         string      `json:"url"`
	Bucket      uint16      `json:"bucket"` // top 16 bits of SimHash for bucketing
}

// Deduper detects duplicate content using SimHash (near-duplicate) and SHA-256 (exact).
type Deduper struct {
	store            FingerprintStore
	log              *logger.Logger
	hammingThreshold int
}

// FingerprintStore defines the interface for storing and retrieving fingerprints.
// Implementations can use MongoDB, Redis, or in-memory storage.
type FingerprintStore interface {
	// StoreFingerprint saves a fingerprint entry.
	StoreFingerprint(ctx context.Context, entry *Entry) error

	// FetchByBucket retrieves fingerprints in a specific bucket.
	FetchByBucket(ctx context.Context, bucket uint16, limit int) ([]Entry, error)

	// FetchBySHA256 retrieves an entry by its exact SHA-256 hash.
	FetchBySHA256(ctx context.Context, sha256 [32]byte) (*Entry, error)

	// Exists checks if a fingerprint already exists.
	Exists(ctx context.Context, sha256 [32]byte) (bool, error)
}

// InMemoryStore is a thread-safe in-memory implementation of FingerprintStore.
type InMemoryStore struct {
	mu     sync.RWMutex
	data   map[[32]byte]*Entry // keyed by SHA-256
	bucket map[uint16][]uint64 // bucket → simhash values
}

// NewInMemoryStore creates a new in-memory fingerprint store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data:   make(map[[32]byte]*Entry),
		bucket: make(map[uint16][]uint64),
	}
}

// StoreFingerprint implements FingerprintStore.
func (s *InMemoryStore) StoreFingerprint(_ context.Context, entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[entry.Fingerprint.SHA256] = entry
	s.bucket[entry.Bucket] = append(s.bucket[entry.Bucket], entry.Fingerprint.SimHash)
	return nil
}

// FetchByBucket implements FingerprintStore.
func (s *InMemoryStore) FetchByBucket(_ context.Context, bucket uint16, limit int) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	simhashes := s.bucket[bucket]
	if len(simhashes) == 0 {
		return nil, nil
	}
	n := limit
	if n > len(simhashes) {
		n = len(simhashes)
	}
	entries := make([]Entry, 0, n)
	for _, sh := range simhashes {
		if len(entries) >= limit {
			break
		}
		for _, e := range s.data {
			if e.Fingerprint.SimHash == sh {
				entries = append(entries, *e)
				break
			}
		}
	}
	return entries, nil
}

// FetchBySHA256 implements FingerprintStore.
func (s *InMemoryStore) FetchBySHA256(_ context.Context, sha256 [32]byte) (*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if entry, ok := s.data[sha256]; ok {
		return entry, nil
	}
	return nil, nil
}

// Exists implements FingerprintStore.
func (s *InMemoryStore) Exists(_ context.Context, sha256 [32]byte) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data[sha256]
	return ok, nil
}

// DeduperOption configures a Deduper.
type DeduperOption func(*Deduper)

// WithHammingThreshold sets the maximum Hamming distance for duplicate detection.
func WithHammingThreshold(n int) DeduperOption {
	return func(d *Deduper) {
		d.hammingThreshold = n
	}
}

// NewDeduper creates a new content deduplicator.
func NewDeduper(store FingerprintStore, opts ...DeduperOption) *Deduper {
	d := &Deduper{
		store:            store,
		log:              logger.NoOp(),
		hammingThreshold: DefaultHammingThreshold,
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// GenerateFingerprint computes the SimHash and SHA-256 fingerprint for the given text.
func (d *Deduper) GenerateFingerprint(text string) Fingerprint {
	tokens := tokenize(text)
	trigrams := generateTrigrams(tokens)
	return computeFingerprint(trigrams, text)
}

// IsDuplicate checks if the content is a duplicate of any stored content.
// It first checks exact SHA-256 match, then falls back to SimHash Hamming distance.
func (d *Deduper) IsDuplicate(ctx context.Context, text string) (bool, string, error) {
	fp := d.GenerateFingerprint(text)

	// Check exact duplicate via SHA-256.
	exists, err := d.store.Exists(ctx, fp.SHA256)
	if err != nil {
		return false, "", fmt.Errorf("dedup: check sha256: %w", err)
	}
	if exists {
		return true, "exact (sha256)", nil
	}

	// Check near-duplicate via SimHash Hamming distance.
	bucket := bucketFromSimHash(fp.SimHash)
	candidates, err := d.store.FetchByBucket(ctx, bucket, 100)
	if err != nil {
		return false, "", fmt.Errorf("dedup: fetch bucket %d: %w", bucket, err)
	}

	for _, candidate := range candidates {
		dist := hammingDistance(fp.SimHash, candidate.Fingerprint.SimHash)
		if dist <= d.hammingThreshold {
			return true, fmt.Sprintf("near-duplicate (hamming=%d, url=%s)", dist, candidate.URL), nil
		}
	}

	return false, "", nil
}

// Store stores a fingerprint for future deduplication checks.
func (d *Deduper) Store(ctx context.Context, text, url string) error {
	fp := d.GenerateFingerprint(text)
	entry := &Entry{
		Fingerprint: fp,
		URL:         url,
		Bucket:      bucketFromSimHash(fp.SimHash),
	}
	if err := d.store.StoreFingerprint(ctx, entry); err != nil {
		return fmt.Errorf("dedup: store: %w", err)
	}
	return nil
}

// tokenize splits text into lowercase words.
func tokenize(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	return words
}

// generateTrigrams generates character trigrams from a list of tokens.
func generateTrigrams(tokens []string) []string {
	var trigrams []string
	for _, token := range tokens {
		if len(token) < 3 {
			continue
		}
		for i := 0; i <= len(token)-3; i++ {
			trigrams = append(trigrams, token[i:i+3])
		}
	}
	return trigrams
}

// computeFingerprint computes the SimHash (FNV-1a over trigram hashes) and SHA-256 of the text.
func computeFingerprint(trigrams []string, text string) Fingerprint {
	// Compute 64-bit SimHash using FNV-1a.
	h := fnv.New64a()
	for _, t := range trigrams {
		_, _ = h.Write([]byte(t))
	}
	simhash := h.Sum64()

	// Compute SHA-256 for exact duplicate detection.
	sha := sha256.Sum256([]byte(text))

	return Fingerprint{
		SimHash: simhash,
		SHA256:  sha,
	}
}

// bucketFromSimHash extracts the top 16 bits of a SimHash for bucketing.
func bucketFromSimHash(simhash uint64) uint16 {
	return uint16(simhash >> 48)
}

// hammingDistance computes the Hamming distance between two 64-bit integers.
func hammingDistance(a, b uint64) int {
	x := a ^ b
	return popcnt(x)
}

// popcnt counts the number of set bits in x using naive iteration.
func popcnt(x uint64) int {
	count := 0
	for x != 0 {
		count++
		x &= x - 1
	}
	return count
}

// ComputeSimHash is a standalone function to compute a SimHash fingerprint from text.
func ComputeSimHash(text string) uint64 {
	tokens := tokenize(text)
	trigrams := generateTrigrams(tokens)
	fp := computeFingerprint(trigrams, text)
	return fp.SimHash
}

// ComputeSHA256 is a standalone function to compute a SHA-256 hash of content.
func ComputeSHA256(content string) [32]byte {
	return sha256.Sum256([]byte(content))
}

// AreSimilar checks if two texts are similar based on Hamming distance of their SimHashes.
func AreSimilar(textA, textB string, threshold int) bool {
	simA := ComputeSimHash(textA)
	simB := ComputeSimHash(textB)
	return hammingDistance(simA, simB) <= threshold
}
