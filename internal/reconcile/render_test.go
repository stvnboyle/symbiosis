package reconcile

import (
	"errors"
	"strings"
	"testing"
)

func mixedPlan() Plan {
	return Plan{Entries: []Entry{
		{ID: "foundation/state-table", Change: Change{Action: ActionCreate}},
		{ID: "foundation/artifact-bucket", Change: Change{
			Action:  ActionUpdate,
			Details: []string{"versioning: Suspended → Enabled", "encryption: none → AES256"},
		}},
		{ID: "foundation/operator-role", Change: Change{Action: ActionNoOp}},
		{ID: "foundation/old-thing", Change: Change{Action: ActionDelete}},
	}}
}

func render(t *testing.T, p Plan) string {
	t.Helper()
	var b strings.Builder
	if err := p.Render(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestRenderMixedPlan(t *testing.T) {
	want := strings.Join([]string{
		"+ foundation/state-table      create",
		"~ foundation/artifact-bucket  update",
		"    versioning: Suspended → Enabled",
		"    encryption: none → AES256",
		"  foundation/operator-role    no change",
		"- foundation/old-thing        delete",
		"",
		"1 to create, 1 to update, 1 to delete",
		"",
	}, "\n")
	if got := render(t, mixedPlan()); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderNoChanges(t *testing.T) {
	plan := Plan{Entries: []Entry{{ID: "a", Change: Change{Action: ActionNoOp}}}}
	want := "  a  no change\n\n0 to create, 0 to update, 0 to delete\n"
	if got := render(t, plan); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderEmptyPlanIsJustTheSummary(t *testing.T) {
	want := "0 to create, 0 to update, 0 to delete\n"
	if got := render(t, Plan{}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderIsStable(t *testing.T) {
	first := render(t, mixedPlan())
	for range 5 {
		if got := render(t, mixedPlan()); got != first {
			t.Fatalf("output changed between runs:\n%s\nvs\n%s", first, got)
		}
	}
}

func TestRenderSummary(t *testing.T) {
	if got, want := mixedPlan().Summary(), "1 to create, 1 to update, 1 to delete"; got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRenderReturnsWriteErrors(t *testing.T) {
	boom := errors.New("broken pipe")
	if err := mixedPlan().Render(failingWriter{boom}); !errors.Is(err, boom) {
		t.Errorf("error = %v, want %v", err, boom)
	}
}
