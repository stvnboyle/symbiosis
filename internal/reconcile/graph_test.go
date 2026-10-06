package reconcile

import (
	"slices"
	"strings"
	"testing"
)

// resources converts fakes for passing to order and New.
func resources(fakes ...*fake) []Resource {
	out := make([]Resource, len(fakes))
	for i, f := range fakes {
		out[i] = f
	}
	return out
}

func ids(rs []Resource) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID()
	}
	return out
}

func TestGraphOrdersDependenciesFirst(t *testing.T) {
	// edge needs compute and network; compute needs network and image.
	got, err := order(resources(
		&fake{id: "edge", deps: []string{"compute", "network"}},
		&fake{id: "compute", deps: []string{"network", "image"}},
		&fake{id: "image"},
		&fake{id: "network"},
	))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"image", "network", "compute", "edge"}
	if !slices.Equal(ids(got), want) {
		t.Errorf("order = %v, want %v", ids(got), want)
	}
}

func TestGraphOrderIgnoresInputOrder(t *testing.T) {
	build := func() []*fake {
		return []*fake{
			{id: "d", deps: []string{"b", "c"}},
			{id: "c", deps: []string{"a"}},
			{id: "b", deps: []string{"a"}},
			{id: "a"},
			{id: "z"},
		}
	}
	first, err := order(resources(build()...))
	if err != nil {
		t.Fatal(err)
	}
	reversed := build()
	slices.Reverse(reversed)
	second, err := order(resources(reversed...))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(first), ids(second)) {
		t.Errorf("order depends on input order: %v vs %v", ids(first), ids(second))
	}
	if want := []string{"a", "b", "c", "d", "z"}; !slices.Equal(ids(first), want) {
		t.Errorf("order = %v, want %v", ids(first), want)
	}
}

func TestGraphRepeatedDependencyCountsOnce(t *testing.T) {
	got, err := order(resources(
		&fake{id: "b", deps: []string{"a", "a"}},
		&fake{id: "a"},
	))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !slices.Equal(ids(got), want) {
		t.Errorf("order = %v, want %v", ids(got), want)
	}
}

func TestGraphEmptyIsValid(t *testing.T) {
	got, err := order(nil)
	if err != nil || len(got) != 0 {
		t.Errorf("order(nil) = %v, %v; want empty and no error", got, err)
	}
}

func TestGraphRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		input   []*fake
		wantErr string
	}{
		{
			name:    "empty id",
			input:   []*fake{{id: ""}},
			wantErr: "empty id",
		},
		{
			name:    "duplicate id",
			input:   []*fake{{id: "a"}, {id: "a"}},
			wantErr: `duplicate resource id "a"`,
		},
		{
			name:    "unknown dependency",
			input:   []*fake{{id: "a", deps: []string{"ghost"}}},
			wantErr: `"a" depends on unknown resource "ghost"`,
		},
		{
			name:    "self dependency",
			input:   []*fake{{id: "a", deps: []string{"a"}}},
			wantErr: `"a" depends on itself`,
		},
		{
			name: "two-resource cycle",
			input: []*fake{
				{id: "a", deps: []string{"b"}},
				{id: "b", deps: []string{"a"}},
			},
			wantErr: "dependency cycle: a → b → a",
		},
		{
			name: "cycle behind valid resources",
			input: []*fake{
				{id: "root"},
				{id: "x", deps: []string{"root", "z"}},
				{id: "y", deps: []string{"x"}},
				{id: "z", deps: []string{"y"}},
				{id: "leaf", deps: []string{"z"}},
			},
			wantErr: "dependency cycle: x → z → y → x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := order(resources(tt.input...))
			if err == nil {
				t.Fatal("got no error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
