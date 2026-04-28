// Package compose provides declarative, config-driven service composition.
// It loads a deploy.yaml manifest and resolves service dependencies so that
// services are initialized and started in dependency order.
package compose

import "fmt"

// ServiceSpec describes a single service declared in deploy.yaml.
type ServiceSpec struct {
	Name         string            `mapstructure:"name"`
	Enabled      bool              `mapstructure:"enabled"`
	Port         int               `mapstructure:"port"`
	Healthz      string            `mapstructure:"healthz"`
	Dependencies []string          `mapstructure:"dependencies"`
	Config       map[string]any    `mapstructure:"config"`
	Schedule     map[string]string `mapstructure:"schedule"`
}

// ServiceManifest holds the top-level services slice from deploy.yaml.
type ServiceManifest struct {
	Services []ServiceSpec `mapstructure:"services"`
}

// ServiceName is the canonical identifier for a service.
func (s *ServiceSpec) ServiceName() string { return s.Name }

// IsEnabled returns true when the service is enabled in the manifest.
func (s *ServiceSpec) IsEnabled() bool { return s.Enabled }

// HasDependency returns true when the given service name is listed as a direct
// dependency of this service.
func (s *ServiceSpec) HasDependency(dep string) bool {
	for _, d := range s.Dependencies {
		if d == dep {
			return true
		}
	}
	return false
}

// ServiceNotFoundError is returned when a declared dependency has no matching
// service definition.
type ServiceNotFoundError struct {
	MissingService string
	DependedBy     string
}

func (e *ServiceNotFoundError) Error() string {
	return fmt.Sprintf("compose: service %q declares dependency on %q, which is not defined in the manifest",
		e.DependedBy, e.MissingService)
}

// DisabledDependencyError is returned when an enabled service depends on a
// service that is explicitly disabled.
type DisabledDependencyError struct {
	DisabledService string
	DependedBy      string
}

func (e *DisabledDependencyError) Error() string {
	return fmt.Sprintf("compose: service %q depends on %q, but %q is disabled",
		e.DependedBy, e.DisabledService, e.DisabledService)
}
