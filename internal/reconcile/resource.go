// Package reconcile is the engine that makes AWS match a declared set of resources.
//
// It works in three steps: observe what exists, diff it against what should exist, and
// apply the difference in dependency order. AWS is always the source of truth; nothing
// here trusts a cached copy of state.
package reconcile

import (
	"context"
	"fmt"
)

// Resource is one AWS thing symbiosis owns. Implementations must be idempotent:
// applying the same desired state twice changes nothing the second time.
type Resource interface {
	// ID is the stable logical id, for example "foundation/state-table".
	ID() string
	// DependsOn lists the ids of resources that must exist before this one.
	DependsOn() []string
	// Observe reads actual state from AWS. A missing resource is not an error: it is
	// reported as Actual{Exists: false}.
	Observe(ctx context.Context) (Actual, error)
	// Diff compares desired config against actual and reports what would change. It
	// makes no AWS calls.
	Diff(actual Actual) Change
	// Apply carries out a create or update returned by Diff, and returns only once
	// the resource is usable.
	Apply(ctx context.Context, change Change) error
	// Delete removes the resource. Deleting a resource that is already gone succeeds.
	Delete(ctx context.Context) error
}

// Actual is what Observe found in AWS.
type Actual struct {
	Exists bool
	// Attrs carries whatever the resource needs from Observe to Diff. The engine
	// never looks inside it.
	Attrs any
}

// Change is what Diff decided should happen to a resource.
type Change struct {
	Action Action
	// Details describes the change for a person, one line per setting, for example
	// "versioning: Suspended → Enabled".
	Details []string
	// Data carries whatever the resource needs from Diff to Apply, such as which
	// settings drifted. The engine never looks inside it.
	Data any
}

// Action is the kind of change planned for a resource.
type Action int

// The actions a plan can contain.
const (
	ActionNoOp Action = iota
	ActionCreate
	ActionUpdate
	ActionDelete
)

func (a Action) String() string {
	switch a {
	case ActionNoOp:
		return "no change"
	case ActionCreate:
		return "create"
	case ActionUpdate:
		return "update"
	case ActionDelete:
		return "delete"
	default:
		return fmt.Sprintf("Action(%d)", int(a))
	}
}
