package compose

import (
	"container/list"
	"fmt"
	"slices"
)

// DependencyGraph builds and traverses a directed acyclic graph of service
// dependencies for topological ordering.
type DependencyGraph struct {
	nodes    []*ServiceSpec
	indexMap map[string]int
	edges    [][]bool
}

func newGraph(manifest *ServiceManifest) *DependencyGraph {
	enabled := EnabledServices(manifest)
	n := len(enabled)
	indexMap := make(map[string]int, n)
	for i, s := range enabled {
		indexMap[s.Name] = i
	}
	edges := make([][]bool, n)
	for i := range edges {
		edges[i] = make([]bool, n)
	}
	for i, svc := range enabled {
		for _, dep := range svc.Dependencies {
			if j, ok := indexMap[dep]; ok {
				edges[j][i] = true
			}
		}
	}
	// Copy to heap so TopSort gets stable pointers.
	nodes := make([]*ServiceSpec, n)
	for i := range enabled {
		nodes[i] = new(ServiceSpec)
		*nodes[i] = enabled[i]
	}
	return &DependencyGraph{nodes: nodes, indexMap: indexMap, edges: edges}
}

// TopSort returns all enabled services in dependency order using Kahn's algorithm.
// A service's dependencies appear before the service itself in the returned order.
func (g *DependencyGraph) TopSort() ([]*ServiceSpec, error) {
	n := len(g.nodes)
	if n == 0 {
		return nil, nil
	}

	// inDeg[j] = number of nodes that depend on j (incoming edges to j).
	inDeg := make([]int, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if g.edges[i][j] {
				inDeg[j]++
			}
		}
	}

	// Start with nodes that have no dependencies (inDeg == 0 = sources).
	q := list.New()
	for j := 0; j < n; j++ {
		if inDeg[j] == 0 {
			q.PushBack(j)
		}
	}

	var order []*ServiceSpec
	for q.Len() > 0 {
		front := q.Remove(q.Front()).(int)
		order = append(order, g.nodes[front])

		// Remove outgoing edges from this node; decrement inDeg of its dependents.
		for j := 0; j < n; j++ {
			if g.edges[front][j] {
				inDeg[j]--
				if inDeg[j] == 0 {
					q.PushBack(j)
				}
			}
		}
	}

	if len(order) < n {
		return nil, &CycleError{services: g.nodes, edges: g.edges}
	}

	return order, nil
}

func Resolve(manifest *ServiceManifest) ([]*ServiceSpec, error) {
	g := newGraph(manifest)
	order, err := g.TopSort()
	if err != nil {
		return nil, err
	}
	return order, nil
}

type CycleError struct {
	services []*ServiceSpec
	edges    [][]bool
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("compose: circular dependency detected among services: %v", e.serviceNames())
}

func (e *CycleError) serviceNames() []string {
	var names []string
	for _, s := range e.services {
		names = append(names, s.Name)
	}
	return names
}

func DirectDependencies(svc *ServiceSpec) []string {
	return slices.Clone(svc.Dependencies)
}

func IsDependedOnBy(manifest *ServiceManifest, svcName string) bool {
	for _, s := range manifest.Services {
		if !s.Enabled || s.Name == svcName {
			continue
		}
		if s.HasDependency(svcName) {
			return true
		}
	}
	return false
}

func ReverseOrder(order []*ServiceSpec) []*ServiceSpec {
	n := len(order)
	rev := make([]*ServiceSpec, n)
	for i, svc := range order {
		rev[n-1-i] = svc
	}
	return rev
}
