package reconcile

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// chain returns three fakes where c depends on b and b depends on a, sharing one log.
func chain(log *[]string) (a, b, c *fake) {
	a = &fake{id: "a", log: log}
	b = &fake{id: "b", deps: []string{"a"}, log: log}
	c = &fake{id: "c", deps: []string{"b"}, log: log}
	return a, b, c
}

func TestApplyCreatesInDependencyOrder(t *testing.T) {
	var log []string
	a, b, c := chain(&log)
	done, err := mustEngine(t, c, b, a).Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"apply a", "apply b", "apply c"}; !slices.Equal(log, want) {
		t.Errorf("writes = %v, want %v", log, want)
	}
	if want := []string{"a: create", "b: create", "c: create"}; !slices.Equal(actions(done), want) {
		t.Errorf("result = %v, want %v", actions(done), want)
	}
}

func TestApplyTwiceMakesNoWritesTheSecondTime(t *testing.T) {
	var log []string
	a, b, c := chain(&log)
	engine := mustEngine(t, a, b, c)
	if _, err := engine.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := totalWrites(a, b, c)

	done, err := engine.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n := totalWrites(a, b, c) - before; n != 0 {
		t.Errorf("second apply made %d write calls, want 0", n)
	}
	if done.HasChanges() {
		t.Errorf("second apply reported changes: %v", actions(done))
	}
}

func TestApplyOnlyTouchesWhatDrifted(t *testing.T) {
	a := &fake{id: "a", exists: true}
	b := &fake{id: "b", exists: true, drift: []string{"encryption: off → on"}}
	done, err := mustEngine(t, a, b).Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.writes != 0 || b.writes != 1 {
		t.Errorf("writes: a=%d b=%d, want 0 and 1", a.writes, b.writes)
	}
	if want := []string{"a: no change", "b: update"}; !slices.Equal(actions(done), want) {
		t.Errorf("result = %v, want %v", actions(done), want)
	}
	// The resource receives the Change its own Diff produced, including Data.
	got, _ := b.applied[0].Data.([]string)
	if want := []string{"encryption: off → on"}; !slices.Equal(got, want) {
		t.Errorf("Apply received Data %v, want %v", got, want)
	}
}

func TestApplyStopsAtTheFirstError(t *testing.T) {
	var log []string
	a, b, c := chain(&log)
	boom := errors.New("limit exceeded")
	b.applyErr = boom

	done, err := mustEngine(t, a, b, c).Apply(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
	if !strings.Contains(err.Error(), "create b") {
		t.Errorf("error %q does not name the action and resource", err)
	}
	if c.reads != 0 || c.writes != 0 {
		t.Errorf("c was touched after b failed: reads=%d writes=%d", c.reads, c.writes)
	}
	if want := []string{"a: create"}; !slices.Equal(actions(done), want) {
		t.Errorf("result = %v, want only what completed: %v", actions(done), want)
	}
}

func TestApplyConvergesAfterAPartialFailure(t *testing.T) {
	var log []string
	a, b, c := chain(&log)
	b.applyErr = errors.New("transient")
	engine := mustEngine(t, a, b, c)
	if _, err := engine.Apply(context.Background()); err == nil {
		t.Fatal("first apply succeeded, want failure")
	}

	b.applyErr = nil
	log = nil
	if _, err := engine.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	// a already exists from the first run and must not be written again.
	if want := []string{"apply b", "apply c"}; !slices.Equal(log, want) {
		t.Errorf("second run writes = %v, want %v", log, want)
	}
	plan, err := engine.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasChanges() {
		t.Errorf("not converged: %v", actions(plan))
	}
}

func TestApplyStopsBetweenResourcesWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var log []string
	a, b, c := chain(&log)
	a.onWrite = cancel // cancelled while a is being applied

	_, err := mustEngine(t, a, b, c).Apply(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if want := []string{"apply a"}; !slices.Equal(log, want) {
		t.Errorf("writes = %v, want %v", log, want)
	}
	if b.reads != 0 {
		t.Errorf("b was observed after cancellation")
	}
}

func TestApplyObserveErrorStopsTheRun(t *testing.T) {
	boom := errors.New("access denied")
	a := &fake{id: "a", observeErr: boom}
	_, err := mustEngine(t, a).Apply(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
	if a.writes != 0 {
		t.Errorf("wrote %d times after a failed observe", a.writes)
	}
}

// deleter is a broken resource whose Diff asks for a delete.
type deleter struct{ fake }

func (d *deleter) Diff(Actual) Change { return Change{Action: ActionDelete} }

func TestApplyRejectsADeleteFromDiff(t *testing.T) {
	d := &deleter{fake{id: "a", exists: true}}
	engine, err := New(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(context.Background()); err == nil {
		t.Fatal("Apply accepted a delete from Diff")
	}
	if d.writes != 0 {
		t.Errorf("wrote %d times, want 0", d.writes)
	}
}

func existingChain(log *[]string) (a, b, c *fake) {
	a, b, c = chain(log)
	a.exists, b.exists, c.exists = true, true, true
	return a, b, c
}

func TestDestroyDeletesInReverseOrder(t *testing.T) {
	var log []string
	a, b, c := existingChain(&log)
	done, err := mustEngine(t, a, b, c).Destroy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"delete c", "delete b", "delete a"}; !slices.Equal(log, want) {
		t.Errorf("writes = %v, want %v", log, want)
	}
	if want := []string{"c: delete", "b: delete", "a: delete"}; !slices.Equal(actions(done), want) {
		t.Errorf("result = %v, want %v", actions(done), want)
	}
}

func TestDestroyTwiceMakesNoWritesTheSecondTime(t *testing.T) {
	var log []string
	a, b, c := existingChain(&log)
	engine := mustEngine(t, a, b, c)
	if _, err := engine.Destroy(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := totalWrites(a, b, c)
	done, err := engine.Destroy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n := totalWrites(a, b, c) - before; n != 0 {
		t.Errorf("second destroy made %d write calls, want 0", n)
	}
	if done.HasChanges() {
		t.Errorf("second destroy reported changes: %v", actions(done))
	}
}

func TestDestroyStopsAtTheFirstError(t *testing.T) {
	var log []string
	a, b, c := existingChain(&log)
	boom := errors.New("bucket not empty")
	b.deleteErr = boom

	done, err := mustEngine(t, a, b, c).Destroy(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
	if !strings.Contains(err.Error(), "delete b") {
		t.Errorf("error %q does not name the action and resource", err)
	}
	// a must survive: b still depends on it.
	if !a.exists || a.writes != 0 {
		t.Errorf("a was deleted even though b still exists")
	}
	if want := []string{"c: delete"}; !slices.Equal(actions(done), want) {
		t.Errorf("result = %v, want %v", actions(done), want)
	}
}

func TestDestroyStopsBetweenResourcesWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var log []string
	a, b, c := existingChain(&log)
	c.onWrite = cancel

	_, err := mustEngine(t, a, b, c).Destroy(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if want := []string{"delete c"}; !slices.Equal(log, want) {
		t.Errorf("writes = %v, want %v", log, want)
	}
}

func TestDestroyObserveErrorStopsTheRun(t *testing.T) {
	boom := errors.New("throttled")
	a := &fake{id: "a", exists: true, observeErr: boom}
	if _, err := mustEngine(t, a).Destroy(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
	if a.writes != 0 {
		t.Errorf("wrote %d times after a failed observe", a.writes)
	}
}
