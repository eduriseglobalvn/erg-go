package compose

import (
	"fmt"
	"os"

	"erg.ninja/pkg/config"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// Load reads a deploy.yaml manifest from path and merges per-service config
// on top of baseCfg using the deep-merge strategy: default → baseCfg → manifest.
//
// The per-service Config map is NOT automatically merged into baseCfg; call
// MergeServiceConfig to apply it to a *config.Config copy before wiring the
// service. This keeps the operation explicit and testable.
//
// Example deploy.yaml:
//
//	services:
//	  - name: bot
//	    enabled: true
//	    port: 8081
//	    dependencies:
//	      - crawler
//	    config:
//	      bot:
//	        command_prefix: "/"
//	        max_conversations: 10000
//
// If path is empty the function returns ErrNoDeployManifest.
var ErrNoDeployManifest = fmt.Errorf("compose: no deploy manifest found")

func Load(path string, baseCfg *config.Config) (*ServiceManifest, error) {
	if path == "" {
		path = "deploy.yaml"
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("%w: %s", ErrNoDeployManifest, path)
		}
		return nil, fmt.Errorf("compose: read deploy manifest: %w", err)
	}

	var manifest ServiceManifest
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:      &manifest,
		ErrorUnused: false,
		TagName:     "mapstructure",
	})
	if err != nil {
		return nil, fmt.Errorf("compose: mapstructure decoder: %w", err)
	}
	if err := dec.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("compose: decode manifest: %w", err)
	}

	// Validate manifest consistency.
	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// validateManifest checks for undefined and disabled dependencies.
func validateManifest(manifest *ServiceManifest) error {
	// Build name → spec lookup.
	specs := make(map[string]*ServiceSpec, len(manifest.Services))
	for i := range manifest.Services {
		specs[manifest.Services[i].Name] = &manifest.Services[i]
	}

	// Validate enabled services.
	for i := range manifest.Services {
		spec := &manifest.Services[i]
		if !spec.Enabled {
			continue
		}
		for _, dep := range spec.Dependencies {
			depSpec, exists := specs[dep]
			if !exists {
				return &ServiceNotFoundError{MissingService: dep, DependedBy: spec.Name}
			}
			if !depSpec.Enabled {
				return &DisabledDependencyError{DisabledService: dep, DependedBy: spec.Name}
			}
		}
	}
	return nil
}

// MergeServiceConfig applies the per-service config map to dst.
// It overwrites fields in dst that are present in serviceCfg.
// The returned *config.Config is a shallow copy of dst; nested structs
// are not deep-copied so callers should treat it as read-only once
// passed to service constructors.
func MergeServiceConfig(dst *config.Config, serviceCfg map[string]any) (*config.Config, error) {
	if len(serviceCfg) == 0 {
		return dst, nil
	}

	// Encode dst to a flat map, overlay serviceCfg, then decode back.
	dstMap := structToMap(dst)
	merged := mergeMap(dstMap, serviceCfg)

	mergedCfg := &config.Config{}
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:      mergedCfg,
		ErrorUnused: false,
		TagName:     "mapstructure",
	})
	if err != nil {
		return nil, fmt.Errorf("compose: merge decoder: %w", err)
	}
	if err := dec.Decode(merged); err != nil {
		return nil, fmt.Errorf("compose: merge decode: %w", err)
	}
	return mergedCfg, nil
}

// structToMap converts a struct to a map[string]any using mapstructure.
// It handles nil receivers gracefully.
func structToMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	out := make(map[string]any)
	dec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:     &out,
		TagName:    "mapstructure",
		ZeroFields: false,
	})
	_ = dec.Decode(v)
	return out
}

// mergeMap overlays src onto dst (dst wins for conflicting keys).
// Nested maps are merged recursively.
func mergeMap(dst, src map[string]any) map[string]any {
	if dst == nil {
		return src
	}
	if src == nil {
		return dst
	}
	result := make(map[string]any, len(dst)+len(src))
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		if dstV, dstOk := dst[k]; dstOk {
			dstMap, dstIsMap := dstV.(map[string]any)
			srcMap, srcIsMap := v.(map[string]any)
			if dstOk && dstIsMap && srcIsMap {
				result[k] = mergeMap(dstMap, srcMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// EnabledServices returns all service specs that have Enabled = true.
func EnabledServices(manifest *ServiceManifest) []ServiceSpec {
	var out []ServiceSpec
	for i := range manifest.Services {
		if manifest.Services[i].Enabled {
			out = append(out, manifest.Services[i])
		}
	}
	return out
}

// GetService returns a pointer to the service spec with the given name.
// Returns nil if not found.
func GetService(manifest *ServiceManifest, name string) *ServiceSpec {
	for i := range manifest.Services {
		if manifest.Services[i].Name == name {
			return &manifest.Services[i]
		}
	}
	return nil
}

// ReadDeployManifest is a convenience that reads deploy.yaml from cwd
// when no path is provided.
func ReadDeployManifest() (*ServiceManifest, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("compose: getcwd: %w", err)
	}
	return Load(cwd+"/deploy.yaml", config.NewDefault())
}
