package reconcile

import (
	"fmt"
	"io"
	"strings"
)

// Summary is the plan's one-line total, for example
// "1 to create, 0 to update, 0 to delete".
func (p Plan) Summary() string {
	create, update, del := p.Counts()
	return fmt.Sprintf("%d to create, %d to update, %d to delete", create, update, del)
}

// Render writes the plan as text: one line per resource, the details of each update
// indented beneath it, then the summary.
func (p Plan) Render(w io.Writer) error {
	width := 0
	for _, e := range p.Entries {
		width = max(width, len(e.ID))
	}

	var b strings.Builder
	for _, e := range p.Entries {
		fmt.Fprintf(&b, "%s %-*s  %s\n", marker(e.Change.Action), width, e.ID, e.Change.Action)
		for _, detail := range e.Change.Details {
			fmt.Fprintf(&b, "    %s\n", detail)
		}
	}
	if len(p.Entries) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(p.Summary() + "\n")

	_, err := io.WriteString(w, b.String())
	return err
}

func marker(a Action) string {
	switch a {
	case ActionCreate:
		return "+"
	case ActionUpdate:
		return "~"
	case ActionDelete:
		return "-"
	default:
		return " "
	}
}
