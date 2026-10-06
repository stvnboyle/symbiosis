package reconcile

import (
	"context"
	"fmt"
	"slices"
)

// Engine reconciles a fixed set of resources against AWS.
type Engine struct {
	ordered []Resource // dependencies first
}

// New builds an engine. It fails if the resources' dependencies do not form a valid
// graph.
func New(resources ...Resource) (*Engine, error) {
	ordered, err := order(resources)
	if err != nil {
		return nil, err
	}
	return &Engine{ordered: ordered}, nil
}

// Plan is an ordered list of what would happen, or did happen, to each resource.
type Plan struct {
	Entries []Entry
}

// Entry is one resource's line in a plan.
type Entry struct {
	ID     string
	Change Change
}

// Counts returns how many resources the plan creates, updates and deletes.
func (p Plan) Counts() (create, update, del int) {
	for _, e := range p.Entries {
		switch e.Change.Action {
		case ActionCreate:
			create++
		case ActionUpdate:
			update++
		case ActionDelete:
			del++
		}
	}
	return create, update, del
}

// HasChanges reports whether the plan would change anything.
func (p Plan) HasChanges() bool {
	create, update, del := p.Counts()
	return create+update+del > 0
}

// Plan reports what Apply would change. It makes no write calls.
//
// A plan is a preview. Apply observes each resource again just before changing it, so
// what it does can differ if AWS changed in between.
func (e *Engine) Plan(ctx context.Context) (Plan, error) {
	var plan Plan
	for _, r := range e.ordered {
		if err := ctx.Err(); err != nil {
			return plan, err
		}
		change, err := diff(ctx, r)
		if err != nil {
			return plan, err
		}
		plan.Entries = append(plan.Entries, Entry{ID: r.ID(), Change: change})
	}
	return plan, nil
}

// PlanDestroy reports what Destroy would delete. It makes no write calls.
func (e *Engine) PlanDestroy(ctx context.Context) (Plan, error) {
	var plan Plan
	for _, r := range slices.Backward(e.ordered) {
		if err := ctx.Err(); err != nil {
			return plan, err
		}
		change, err := diffDestroy(ctx, r)
		if err != nil {
			return plan, err
		}
		plan.Entries = append(plan.Entries, Entry{ID: r.ID(), Change: change})
	}
	return plan, nil
}

// diff observes one resource and returns the change needed to reach desired state.
func diff(ctx context.Context, r Resource) (Change, error) {
	actual, err := r.Observe(ctx)
	if err != nil {
		return Change{}, fmt.Errorf("observe %s: %w", r.ID(), err)
	}
	return r.Diff(actual), nil
}

// diffDestroy observes one resource and returns a delete if it exists.
func diffDestroy(ctx context.Context, r Resource) (Change, error) {
	actual, err := r.Observe(ctx)
	if err != nil {
		return Change{}, fmt.Errorf("observe %s: %w", r.ID(), err)
	}
	if !actual.Exists {
		return Change{Action: ActionNoOp}, nil
	}
	return Change{Action: ActionDelete}, nil
}
