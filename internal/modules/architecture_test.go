package modules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModulesFollowCanonicalArchitecture(t *testing.T) {
	root := "."
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read modules root: %v", err)
	}

	requiredDirs := []string{"api", "application", "domain", "infrastructure"}
	legacyDirs := map[string]bool{
		"cache": true, "commands": true, "controller": true, "controllers": true,
		"dto": true, "entities": true, "entity": true, "handlers": true,
		"jobs": true, "middleware": true, "model": true, "models": true,
		"platform": true, "providers": true, "repositories": true,
		"repository": true, "service": true, "services": true,
		"templates": true, "watermark": true,
	}
	allowedRootGo := map[string]bool{
		"adapters.go": true,
		"compat.go":   true,
		"module.go":   true,
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		module := entry.Name()
		modulePath := filepath.Join(root, module)

		for _, dir := range requiredDirs {
			if info, err := os.Stat(filepath.Join(modulePath, dir)); err != nil || !info.IsDir() {
				t.Fatalf("%s missing required %s directory", module, dir)
			}
		}

		children, err := os.ReadDir(modulePath)
		if err != nil {
			t.Fatalf("read module %s: %v", module, err)
		}
		for _, child := range children {
			name := child.Name()
			if child.IsDir() && legacyDirs[name] {
				t.Fatalf("%s still has legacy root directory %s", module, name)
			}
			if child.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			if !allowedRootGo[name] {
				t.Fatalf("%s has unexpected root Go file %s", module, name)
			}
		}
	}
}
