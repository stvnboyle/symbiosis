package reconcile

import (
	"context"
	"fmt"
	"slices"
)

// Apply makes AWS match desired state, in dependency order, and returns what it did.
//
// Each resource is observed immediately before it is changed, so a resource created
// earlier in the same run is visible to the ones that depend on it. Apply stops at the
// first error and returns the entries completed so far; running it again picks up
// where it left off.
func (e *Engine) Apply(ctx context.Context) (Plan, error) {
	var done Plan
	for _, r := range e.ordered {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		change, err := diff(ctx, r)
		if err != nil {
			return done, err
		}
		switch change.Action {
		case ActionNoOp:
		case ActionCreate, ActionUpdate:
			if err := r.Apply(ctx, change); err != nil {
				return done, fmt.Errorf("%s %s: %w", change.Action, r.ID(), err)
			}
		default:
			// Deleting is the engine's decision, made only by Destroy.
			return done, fmt.Errorf("%s: Diff returned %q, want create, update or no change", r.ID(), change.Action)
		}
		done.Entries = append(done.Entries, Entry{ID: r.ID(), Change: change})
	}
	return done, nil
}

// Destroy deletes every resource that exists, dependents first, and returns what it
// did. It stops at the first error, leaving everything the failed resource depends on
// in place.
func (e *Engine) Destroy(ctx context.Context) (Plan, error) {
	var done Plan
	for _, r := range slices.Backward(e.ordered) {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		change, err := diffDestroy(ctx, r)
		if err != nil {
			return done, err
		}
		if change.Action == ActionDelete {
			if err := r.Delete(ctx); err != nil {
				return done, fmt.Errorf("delete %s: %w", r.ID(), err)
			}
		}
		done.Entries = append(done.Entries, Entry{ID: r.ID(), Change: change})
	}
	return done, nil
}
