package reconcile

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// order returns resources sorted so that every resource comes after the ones it
// depends on. Ties are broken by id, so the result does not depend on input order.
func order(resources []Resource) ([]Resource, error) {
	byID := make(map[string]Resource, len(resources))
	ids := make([]string, 0, len(resources))
	for _, r := range resources {
		id := r.ID()
		if id == "" {
			return nil, errors.New("resource has an empty id")
		}
		if _, dup := byID[id]; dup {
			return nil, fmt.Errorf("duplicate resource id %q", id)
		}
		byID[id] = r
		ids = append(ids, id)
	}
	slices.Sort(ids)

	deps := make(map[string][]string, len(ids)) // id → its distinct dependencies
	dependents := make(map[string][]string, len(ids))
	for _, id := range ids {
		for _, dep := range byID[id].DependsOn() {
			if dep == id {
				return nil, fmt.Errorf("resource %q depends on itself", id)
			}
			if _, ok := byID[dep]; !ok {
				return nil, fmt.Errorf("resource %q depends on unknown resource %q", id, dep)
			}
			if !slices.Contains(deps[id], dep) {
				deps[id] = append(deps[id], dep)
				dependents[dep] = append(dependents[dep], id)
			}
		}
	}

	// Kahn's algorithm: repeatedly take a resource with no unmet dependencies.
	unmet := make(map[string]int, len(ids))
	var ready []string
	for _, id := range ids {
		unmet[id] = len(deps[id])
		if unmet[id] == 0 {
			ready = append(ready, id)
		}
	}
	ordered := make([]Resource, 0, len(ids))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byID[id])
		delete(unmet, id)
		for _, dependent := range dependents[id] {
			unmet[dependent]--
			if unmet[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		slices.Sort(ready)
	}
	if len(unmet) > 0 {
		return nil, fmt.Errorf("dependency cycle: %s", strings.Join(findCycle(ids, deps, unmet), " → "))
	}
	return ordered, nil
}

// findCycle returns one cycle among the resources Kahn's algorithm could not place,
// as a path that starts and ends on the same id. Each id depends on the one after it.
func findCycle(ids []string, deps map[string][]string, unplaced map[string]int) []string {
	// Every unplaced resource has at least one unplaced dependency, so following
	// those must eventually revisit a resource.
	var start string
	for _, id := range ids {
		if _, ok := unplaced[id]; ok {
			start = id
			break
		}
	}
	path := []string{start}
	for {
		current := path[len(path)-1]
		var next string
		for _, dep := range deps[current] {
			if _, ok := unplaced[dep]; ok {
				next = dep
				break
			}
		}
		if i := slices.Index(path, next); i >= 0 {
			// Rotate to start at the smallest id, so the same cycle always reads the same.
			cycle := path[i:]
			first := slices.Index(cycle, slices.Min(cycle))
			cycle = append(slices.Clone(cycle[first:]), cycle[:first]...)
			return append(cycle, cycle[0])
		}
		path = append(path, next)
	}
}
