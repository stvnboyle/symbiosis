package reconcile

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// totalWrites sums write calls across fakes.
func totalWrites(fakes ...*fake) int {
	n := 0
	for _, f := range fakes {
		n += f.writes
	}
	return n
}

func mustEngine(t *testing.T, fakes ...*fake) *Engine {
	t.Helper()
	engine, err := New(resources(fakes...)...)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func actions(p Plan) []string {
	out := make([]string, len(p.Entries))
	for i, e := range p.Entries {
		out[i] = e.ID + ": " + e.Change.Action.String()
	}
	return out
}

func TestPlanNewRejectsAnInvalidGraph(t *testing.T) {
	_, err := New(resources(&fake{id: "a", deps: []string{"missing"}})...)
	if err == nil {
		t.Fatal("New accepted an unknown dependency")
	}
}

func TestPlanReportsEachKindOfChange(t *testing.T) {
	missing := &fake{id: "a-missing"}
	drifted := &fake{id: "b-drifted", exists: true, drift: []string{"versioning: Suspended → Enabled"}}
	matching := &fake{id: "c-matching", exists: true}
	engine := mustEngine(t, missing, drifted, matching)

	plan, err := engine.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a-missing: create", "b-drifted: update", "c-matching: no change"}
	if !slices.Equal(actions(plan), want) {
		t.Errorf("plan = %v, want %v", actions(plan), want)
	}
	if got := plan.Entries[1].Change.Details; !slices.Equal(got, drifted.drift) {
		t.Errorf("update details = %v, want %v", got, drifted.drift)
	}
	if create, update, del := plan.Counts(); create != 1 || update != 1 || del != 0 {
		t.Errorf("Counts = %d, %d, %d; want 1, 1, 0", create, update, del)
	}
	if !plan.HasChanges() {
		t.Error("HasChanges = false, want true")
	}
}

func TestPlanMakesNoWrites(t *testing.T) {
	a := &fake{id: "a"}
	b := &fake{id: "b", exists: true, drift: []string{"x"}, deps: []string{"a"}}
	engine := mustEngine(t, a, b)

	for range 3 {
		if _, err := engine.Plan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.PlanDestroy(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if n := totalWrites(a, b); n != 0 {
		t.Errorf("planning made %d write calls, want 0", n)
	}
	if a.reads != 6 || b.reads != 6 {
		t.Errorf("reads = %d and %d, want 6 each", a.reads, b.reads)
	}
}

func TestPlanWithNothingToDoHasNoChanges(t *testing.T) {
	plan, err := mustEngine(t, &fake{id: "a", exists: true}).Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasChanges() {
		t.Errorf("HasChanges = true for %v", actions(plan))
	}
}

func TestPlanObserveErrorNamesTheResource(t *testing.T) {
	boom := errors.New("access denied")
	engine := mustEngine(t,
		&fake{id: "a"},
		&fake{id: "b", observeErr: boom},
		&fake{id: "c"},
	)
	plan, err := engine.Plan(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
	if !strings.Contains(err.Error(), "observe b") {
		t.Errorf("error %q does not name the resource", err)
	}
	if want := []string{"a: create"}; !slices.Equal(actions(plan), want) {
		t.Errorf("partial plan = %v, want %v", actions(plan), want)
	}
}

func TestPlanStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &fake{id: "a"}
	_, err := mustEngine(t, a).Plan(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if a.reads != 0 {
		t.Errorf("observed %d times after cancellation, want 0", a.reads)
	}
}

func TestPlanDestroyRunsInReverseAndSkipsAbsent(t *testing.T) {
	engine := mustEngine(t,
		&fake{id: "a", exists: true},
		&fake{id: "b", exists: false, deps: []string{"a"}},
		&fake{id: "c", exists: true, deps: []string{"b"}},
	)
	plan, err := engine.PlanDestroy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"c: delete", "b: no change", "a: delete"}
	if !slices.Equal(actions(plan), want) {
		t.Errorf("destroy plan = %v, want %v", actions(plan), want)
	}
	if _, _, del := plan.Counts(); del != 2 {
		t.Errorf("delete count = %d, want 2", del)
	}
}

func TestPlanDestroyObserveError(t *testing.T) {
	boom := errors.New("throttled")
	_, err := mustEngine(t, &fake{id: "a", observeErr: boom}).PlanDestroy(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
}
