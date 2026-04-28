package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoaderNew tests Loader creation.
func TestLoaderNew(t *testing.T) {
	tmp := t.TempDir()
	l := NewLoader(tmp)
	if l == nil {
		t.Fatal("NewLoader returned nil")
	}
}

// TestLoaderLoadEmptyName tests that Load rejects empty name.
func TestLoaderLoadEmptyName(t *testing.T) {
	tmp := t.TempDir()
	l := NewLoader(tmp)
	_, err := l.Load("")
	if err == nil {
		t.Error("Load(\"\") expected error, got nil")
	}
}

// TestLoaderLoadNonexistent tests that Load returns error for non-existent plugin.
func TestLoaderLoadNonexistent(t *testing.T) {
	tmp := t.TempDir()
	l := NewLoader(tmp)
	_, err := l.Load("nonexistent-module-xyz-123")
	if err == nil {
		t.Error("Load(nonexistent) expected error, got nil")
	}
}

// TestLoaderLoadAllEmptyDir tests LoadAll on empty directory.
func TestLoaderLoadAllEmptyDir(t *testing.T) {
	tmp := t.TempDir()
	l := NewLoader(tmp)
	mods, errs := l.LoadAll()
	if len(mods) != 0 {
		t.Errorf("LoadAll() = %d mods, want 0", len(mods))
	}
	if len(errs) != 0 {
		t.Errorf("LoadAll() errors on empty dir: %v", errs)
	}
}

// TestLoaderLoadAllNonexistentDir tests LoadAll on non-existent directory.
func TestLoaderLoadAllNonexistentDir(t *testing.T) {
	l := NewLoader("/nonexistent/path/xyz-123")
	_, errs := l.LoadAll()
	if len(errs) == 0 {
		t.Error("LoadAll() expected error for non-existent dir, got nil")
	}
}

// TestLoaderNonSoFiles tests that LoadAll skips non-.so files.
func TestLoaderNonSoFiles(t *testing.T) {
	tmp := t.TempDir()
	// Create a non-.so file
	f, _ := os.Create(filepath.Join(tmp, "erg-crawler.txt"))
	f.WriteString("not a plugin")
	f.Close()

	mods, errs := NewLoader(tmp).LoadAll()
	if len(mods) != 0 {
		t.Errorf("LoadAll() should skip .txt files, got %d", len(mods))
	}
	if len(errs) != 0 {
		t.Errorf("LoadAll() should not error on .txt files, got: %v", errs)
	}
}
