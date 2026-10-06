package reconcile

import (
	"context"
	"testing"
)

// fake is an in-memory Resource. It counts its calls and appends every write to a
// shared log so tests can assert on ordering.
type fake struct {
	id     string
	deps   []string
	exists bool
	drift  []string // settings that differ from desired; cleared by a successful Apply

	observeErr error
	applyErr   error
	deleteErr  error
	onWrite    func() // runs at the start of every Apply and Delete

	reads, writes int
	log           *[]string
	applied       []Change
}

func (f *fake) ID() string          { return f.id }
func (f *fake) DependsOn() []string { return f.deps }

func (f *fake) Observe(context.Context) (Actual, error) {
	f.reads++
	if f.observeErr != nil {
		return Actual{}, f.observeErr
	}
	return Actual{Exists: f.exists, Attrs: append([]string(nil), f.drift...)}, nil
}

func (f *fake) Diff(actual Actual) Change {
	if !actual.Exists {
		return Change{Action: ActionCreate}
	}
	drift, _ := actual.Attrs.([]string)
	if len(drift) > 0 {
		return Change{Action: ActionUpdate, Details: drift, Data: drift}
	}
	return Change{Action: ActionNoOp}
}

func (f *fake) Apply(_ context.Context, change Change) error {
	f.write("apply")
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applied = append(f.applied, change)
	f.exists, f.drift = true, nil
	return nil
}

func (f *fake) Delete(context.Context) error {
	f.write("delete")
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.exists = false
	return nil
}

func (f *fake) write(op string) {
	f.writes++
	if f.onWrite != nil {
		f.onWrite()
	}
	if f.log != nil {
		*f.log = append(*f.log, op+" "+f.id)
	}
}

func TestFakeFollowsTheResourceContract(t *testing.T) {
	ctx := context.Background()
	f := &fake{id: "test/thing"}
	var r Resource = f

	actual, err := r.Observe(ctx)
	if err != nil || actual.Exists {
		t.Fatalf("Observe on a missing resource = %+v, %v; want absent and no error", actual, err)
	}
	change := r.Diff(actual)
	if change.Action != ActionCreate {
		t.Fatalf("Diff on a missing resource = %s, want create", change.Action)
	}
	if err := r.Apply(ctx, change); err != nil {
		t.Fatal(err)
	}
	actual, _ = r.Observe(ctx)
	if got := r.Diff(actual).Action; got != ActionNoOp {
		t.Fatalf("Diff after Apply = %s, want no change", got)
	}
	if f.reads != 2 || f.writes != 1 {
		t.Errorf("reads=%d writes=%d, want 2 and 1", f.reads, f.writes)
	}
}

func TestActionString(t *testing.T) {
	for action, want := range map[Action]string{
		ActionNoOp:   "no change",
		ActionCreate: "create",
		ActionUpdate: "update",
		ActionDelete: "delete",
		Action(99):   "Action(99)",
	} {
		if got := action.String(); got != want {
			t.Errorf("Action(%d).String() = %q, want %q", int(action), got, want)
		}
	}
}
