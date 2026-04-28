// Package main is the entry point for the erg-server binary.
//
// Build tags enable compile-time module selection:
//
//	go build -tags 'module_crawler,module_notification' ./cmd/server
//
// Valid tags: module_bot, module_crawler, module_notification,
//
//	module_trending, all_modules
//
// Without tags, all 4 modules are registered (legacy behaviour).
package main

import (
	"erg.ninja/pkg/plugin"
)

// moduleTags is a compile-time set of enabled module names.
// It is populated by init() functions in build-tag-gated files
// (module_bot.go, module_crawler.go, etc.).
var moduleTags []string

// init registers all modules whose build tags are active.
func init() {
	moduleTags = detectModules()
}

// detectModules returns the list of module names that are registered
// in the plugin registry (selected via build tags).
func detectModules() []string {
	specs := plugin.Registered()
	names := make([]string, 0, len(specs))
	for _, s := range specs {
		names = append(names, s.Name)
	}
	return names
}

// EnabledModules returns the compile-time enabled module names.
func EnabledModules() []string {
	return moduleTags
}
