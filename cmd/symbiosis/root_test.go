package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

var subcommands = []string{"bootstrap", "plan", "status", "doctor", "destroy"}

// run executes the root command with args and returns what it printed.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestHelpListsEverySubcommand(t *testing.T) {
	out, err := run(t, "--help")
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	for _, name := range subcommands {
		if !strings.Contains(out, "\n  "+name+" ") {
			t.Errorf("--help does not list %q:\n%s", name, out)
		}
	}
}

func TestSubcommandsAreStubs(t *testing.T) {
	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			_, err := run(t, name)
			if !errors.Is(err, errNotImplemented) {
				t.Fatalf("got error %v, want errNotImplemented", err)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name the command", err)
			}
		})
	}
}

func TestSubcommandsRejectPositionalArgs(t *testing.T) {
	_, err := run(t, "plan", "extra")
	if err == nil || errors.Is(err, errNotImplemented) {
		t.Fatalf("got error %v, want an argument error", err)
	}
}

func TestGlobalFlagsParse(t *testing.T) {
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"doctor", "--profile", "sandbox", "--region", "eu-west-2"})
	if err := root.Execute(); !errors.Is(err, errNotImplemented) {
		t.Fatalf("got error %v, want errNotImplemented", err)
	}
	for flag, want := range map[string]string{"profile": "sandbox", "region": "eu-west-2"} {
		got, err := root.PersistentFlags().GetString(flag)
		if err != nil {
			t.Fatalf("--%s: %v", flag, err)
		}
		if got != want {
			t.Errorf("--%s = %q, want %q", flag, got, want)
		}
	}
}

func TestUnknownFlagIsAnError(t *testing.T) {
	if _, err := run(t, "plan", "--nope"); err == nil {
		t.Fatal("unknown flag was accepted")
	}
}

func TestVersion(t *testing.T) {
	out, err := run(t, "--version")
	if err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
	if want := "symbiosis version " + version; !strings.Contains(out, want) {
		t.Errorf("got %q, want it to contain %q", out, want)
	}
}
