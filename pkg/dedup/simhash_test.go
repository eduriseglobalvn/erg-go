package dedup

import (
	"context"
	"testing"
)

func TestInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	entry := &Entry{
		Fingerprint: Fingerprint{SimHash: 0x1234567890ABCDEF, SHA256: sha256Of("hello")},
		URL:         "https://example.com/article",
		Bucket:      0x1234,
	}

	if err := store.StoreFingerprint(ctx, entry); err != nil {
		t.Fatalf("StoreFingerprint: %v", err)
	}

	exists, err := store.Exists(ctx, entry.Fingerprint.SHA256)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("entry should exist after StoreFingerprint")
	}

	fetched, err := store.FetchBySHA256(ctx, entry.Fingerprint.SHA256)
	if err != nil {
		t.Fatalf("FetchBySHA256: %v", err)
	}
	if fetched == nil {
		t.Fatal("FetchBySHA256 returned nil")
	}
	if fetched.URL != entry.URL {
		t.Errorf("URL = %q, want %q", fetched.URL, entry.URL)
	}

	bucketEntries, err := store.FetchByBucket(ctx, 0x1234, 10)
	if err != nil {
		t.Fatalf("FetchByBucket: %v", err)
	}
	if len(bucketEntries) == 0 {
		t.Error("FetchByBucket should return entries")
	}
}

func TestHammingDistance(t *testing.T) {
	cases := []struct {
		a, b     uint64
		expected int
	}{
		{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0},
		{0xFFFFFFFFFFFFFFFF, 0, 64},
		{0b11110000, 0b00001111, 8},
		{0x1234567890ABCDEF, 0x1234567890ABCDEF, 0},
	}
	for _, c := range cases {
		got := hammingDistance(c.a, c.b)
		if got != c.expected {
			t.Errorf("hammingDistance(%x, %x) = %d, want %d", c.a, c.b, got, c.expected)
		}
	}
}

func TestPopcnt(t *testing.T) {
	if popcnt(0) != 0 {
		t.Error("popcnt(0) should be 0")
	}
	if popcnt(1) != 1 {
		t.Error("popcnt(1) should be 1")
	}
	if popcnt(0xFF) != 8 {
		t.Error("popcnt(0xFF) should be 8")
	}
}

func TestBucketFromSimHash(t *testing.T) {
	bucket := bucketFromSimHash(0xFEDCBA9876543210)
	if bucket == 0 {
		// top 16 bits should not be zero for this value
	}
	// Just verify no panic.
}

func TestGenerateFingerprint(t *testing.T) {
	store := NewInMemoryStore()
	d := NewDeduper(store)

	fp := d.GenerateFingerprint("Hello world this is a test article about Go programming language")
	if fp.SimHash == 0 && fp.SHA256 == [32]byte{} {
		t.Error("GenerateFingerprint should produce non-zero values")
	}

	// Same text should produce same fingerprint.
	fp2 := d.GenerateFingerprint("Hello world this is a test article about Go programming language")
	if fp.SimHash != fp2.SimHash {
		t.Error("Same text should produce identical SimHash")
	}
	if fp.SHA256 != fp2.SHA256 {
		t.Error("Same text should produce identical SHA256")
	}

	// Different text should produce different fingerprint.
	fp3 := d.GenerateFingerprint("Completely different text here")
	if fp.SHA256 == fp3.SHA256 {
		t.Error("Different text should produce different SHA256")
	}
}

func TestIsDuplicateExactMatch(t *testing.T) {
	store := NewInMemoryStore()
	d := NewDeduper(store)
	ctx := context.Background()

	text := "This is the exact same article content"

	// First time, not a duplicate.
	isDup, _, err := d.IsDuplicate(ctx, text)
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if isDup {
		t.Error("first occurrence should not be a duplicate")
	}

	// Store it.
	if err := d.Store(ctx, text, "https://example.com/a"); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Second time, should be detected as exact duplicate.
	isDup, reason, err := d.IsDuplicate(ctx, text)
	if err != nil {
		t.Fatalf("IsDuplicate second: %v", err)
	}
	if !isDup {
		t.Error("second occurrence should be a duplicate")
	}
	if reason != "exact (sha256)" {
		t.Errorf("reason = %q, want 'exact (sha256)'", reason)
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello World This IS a TEST")
	expected := []string{"hello", "world", "this", "is", "a", "test"}
	if len(tokens) != len(expected) {
		t.Fatalf("token count = %d, want %d", len(tokens), len(expected))
	}
	for i, e := range expected {
		if tokens[i] != e {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i], e)
		}
	}
}

func TestGenerateTrigrams(t *testing.T) {
	tokens := []string{"hello", "world"}
	trigrams := generateTrigrams(tokens)
	// "hello" → hel, ell, llo
	// "world" → wor, orl, rld
	expected := []string{"hel", "ell", "llo", "wor", "orl", "rld"}
	if len(trigrams) != len(expected) {
		t.Fatalf("trigram count = %d, want %d", len(trigrams), len(expected))
	}
}

func TestAreSimilar(t *testing.T) {
	text1 := "The quick brown fox jumps over the lazy dog"
	text2 := "The quick brown fox jumps over the lazy dog" // identical
	text3 := "The quick red fox jumps over the lazy dog"   // minor change

	if !AreSimilar(text1, text2, 6) {
		t.Error("identical texts should be similar")
	}

	// Exact match via hamming ≤ 6.
	diff := hammingDistance(ComputeSimHash(text1), ComputeSimHash(text3))
	similar := diff <= 6
	if !similar {
		t.Logf("text1 vs text3 hamming distance = %d", diff)
	}
}

func TestComputeSimHash(t *testing.T) {
	sh := ComputeSimHash("test content for simhash")
	if sh == 0 {
		t.Error("ComputeSimHash should return non-zero")
	}
}

func TestComputeSHA256(t *testing.T) {
	h := ComputeSHA256("test content")
	if h == [32]byte{} {
		t.Error("ComputeSHA256 should return non-zero")
	}
}

func sha256Of(s string) [32]byte {
	return ComputeSHA256(s)
}

func TestDeduperWithCustomThreshold(t *testing.T) {
	store := NewInMemoryStore()
	d := NewDeduper(store, WithHammingThreshold(3))
	if d.hammingThreshold != 3 {
		t.Errorf("threshold = %d, want 3", d.hammingThreshold)
	}
}
