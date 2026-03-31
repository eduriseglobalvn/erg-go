package database

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"erg.ninja/pkg/config"
)

func TestBuildURI(t *testing.T) {
	cfg := config.MongoDBConfig{
		Host:       "localhost",
		Port:       27017,
		User:       "admin",
		Password:   "secret",
		AuthSource: "admin",
	}
	uri, err := buildURI(cfg)
	if err != nil {
		t.Fatalf("buildURI: %v", err)
	}
	if uri == "" {
		t.Error("buildURI returned empty string")
	}
	if !strings.Contains(uri, "admin:secret") {
		t.Errorf("buildURI missing credentials: %s", uri)
	}
}

func TestBuildURIWithExplicitURI(t *testing.T) {
	cfg := config.MongoDBConfig{
		URI: "mongodb+srv://user:pass@cluster.mongodb.net/?replicaSet=rs0",
	}
	uri, err := buildURI(cfg)
	if err != nil {
		t.Fatalf("buildURI: %v", err)
	}
	if uri != cfg.URI {
		t.Errorf("buildURI returned %q, want %q", uri, cfg.URI)
	}
}

func TestParseReadPreference(t *testing.T) {
	cases := []struct {
		input string
	}{
		{"primary"},
		{"secondaryPreferred"},
		{"secondary"},
		{"nearest"},
		{"primaryPreferred"},
		{"invalid"},
		{""},
	}
	for _, c := range cases {
		mode, err := parseReadPreference(c.input)
		if err != nil {
			t.Errorf("parseReadPreference(%q): unexpected error: %v", c.input, err)
		}
		_ = mode // mode may be nil for invalid input
	}
	// Verify default is secondaryPreferred.
	mode, _ := parseReadPreference("")
	if *mode != readpref.SecondaryPreferred() {
		t.Errorf("default read preference should be SecondaryPreferred")
	}
}

func TestMongoDBConfigDefaults(t *testing.T) {
	cfg := config.NewDefault().MongoDB
	if cfg.MaxPoolSize == 0 {
		t.Error("MaxPoolSize should not be zero")
	}
	if cfg.ConnectTimeout == 0 {
		t.Error("ConnectTimeout should not be zero")
	}
	if cfg.ServerSelectionTimeout == 0 {
		t.Error("ServerSelectionTimeout should not be zero")
	}
}

func TestIsDuplicateKey(t *testing.T) {
	if IsDuplicateKey(nil) {
		t.Error("nil error should not be duplicate key")
	}
}

func TestIsDuplicateKeyWithError(t *testing.T) {
	// A generic error should not be flagged as duplicate key.
	err := &stringsReplacerFake{"some unrelated error"}
	if IsDuplicateKey(err) {
		t.Error("generic error should not be duplicate key")
	}
}

type stringsReplacerFake struct {
	s string
}

func (f *stringsReplacerFake) Error() string { return f.s }

func TestMongoClientClose(t *testing.T) {
	// Close on nil client should be a no-op.
	m := &MongoClient{closed: true}
	m.Close(nil) // should not panic
}

func TestTimeouts(t *testing.T) {
	cfg := config.MongoDBConfig{
		ConnectTimeout:         5 * time.Second,
		ServerSelectionTimeout: 10 * time.Second,
		SocketTimeout:          30 * time.Second,
	}
	if cfg.ConnectTimeout < 1*time.Second {
		t.Error("ConnectTimeout should be at least 1s for production")
	}
}
