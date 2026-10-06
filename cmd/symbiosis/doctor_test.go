package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/stvnboyle/symbiosis/internal/awsx"
	"github.com/stvnboyle/symbiosis/internal/config"
)

const (
	testAccount = "123456789012"
	testARN     = "arn:aws:sts::123456789012:assumed-role/Admin/steven"
)

// doctorWorld is a fake environment for doctor: a config file, AWS credentials and
// installed tools.
type doctorWorld struct {
	cfg         config.Config
	cfgErr      error
	identity    awsx.Identity
	identityErr error
	missing     map[string]bool // tools that are not installed

	gotConfigPath, gotProfile, gotRegion string
	identityCalls                        int
}

func healthyWorld() *doctorWorld {
	return &doctorWorld{
		cfg:      config.Config{AccountID: testAccount, Region: "eu-west-2", Profile: "sandbox", AlertEmail: "a@b.co"},
		identity: awsx.Identity{Account: testAccount, ARN: testARN},
	}
}

func (w *doctorWorld) deps() deps {
	return deps{
		loadConfig: func(path string) (config.Config, error) {
			w.gotConfigPath = path
			return w.cfg, w.cfgErr
		},
		identity: func(_ context.Context, profile, region string) (awsx.Identity, error) {
			w.identityCalls++
			w.gotProfile, w.gotRegion = profile, region
			return w.identity, w.identityErr
		},
		toolVersion: func(_ context.Context, name string, _ ...string) (string, error) {
			if w.missing[name] {
				return "", errors.New("executable file not found in $PATH")
			}
			return name + " version 1.2.3", nil
		},
	}
}

func (w *doctorWorld) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	root := newRootCmdWith(w.deps())
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"doctor"}, args...))
	err := root.Execute()
	return out.String(), err
}

func wantLines(t *testing.T, out string, lines ...string) {
	t.Helper()
	for _, line := range lines {
		if !strings.Contains(out, line+"\n") {
			t.Errorf("output is missing the line %q:\n%s", line, out)
		}
	}
}

func TestDoctorHealthy(t *testing.T) {
	w := healthyWorld()
	out, err := w.run(t)
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	wantLines(t, out,
		"✓ config       symbiosis.yaml",
		"✓ credentials  "+testARN,
		"✓ account      123456789012 matches the pinned account",
		"✓ region       eu-west-2",
		"✓ go           go version 1.2.3",
		"✓ docker       docker version 1.2.3",
		"✓ aws cli      aws version 1.2.3",
	)
	if w.gotProfile != "sandbox" || w.gotRegion != "eu-west-2" {
		t.Errorf("identity checked with profile %q region %q, want the config's values", w.gotProfile, w.gotRegion)
	}
	if w.identityCalls != 1 {
		t.Errorf("made %d identity calls, want exactly 1", w.identityCalls)
	}
}

func TestDoctorFlagsOverrideConfig(t *testing.T) {
	w := healthyWorld()
	out, err := w.run(t, "--profile", "other", "--region", "us-east-1", "--config", "elsewhere.yaml")
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	if w.gotConfigPath != "elsewhere.yaml" || w.gotProfile != "other" || w.gotRegion != "us-east-1" {
		t.Errorf("got config %q profile %q region %q, want the flag values", w.gotConfigPath, w.gotProfile, w.gotRegion)
	}
	wantLines(t, out, "✓ region       us-east-1")
}

func TestDoctorMissingConfig(t *testing.T) {
	w := healthyWorld()
	w.cfg, w.cfgErr = config.Config{}, fs.ErrNotExist
	out, err := w.run(t)
	if err == nil {
		t.Fatalf("doctor passed with no config:\n%s", out)
	}
	if !strings.Contains(err.Error(), "2 checks failed") {
		t.Errorf("error = %q, want it to count 2 failed checks", err)
	}
	if !strings.Contains(out, "✗ config") || !strings.Contains(out, "✗ credentials  no profile or region") {
		t.Errorf("output does not show the config and credentials failures:\n%s", out)
	}
	wantLines(t, out, "- account      skipped: no pinned account to check against")
	if w.identityCalls != 0 {
		t.Errorf("called AWS %d times with no profile, want 0", w.identityCalls)
	}
}

// With no config file, flags are still enough to check that credentials work.
func TestDoctorMissingConfigWithFlags(t *testing.T) {
	w := healthyWorld()
	w.cfg, w.cfgErr = config.Config{}, fs.ErrNotExist
	out, err := w.run(t, "--profile", "sandbox", "--region", "eu-west-2")
	if err == nil || !strings.Contains(err.Error(), "1 check failed") {
		t.Fatalf("error = %v, want exactly 1 failed check\n%s", err, out)
	}
	wantLines(t, out, "✓ credentials  "+testARN)
}

func TestDoctorWrongAccount(t *testing.T) {
	w := healthyWorld()
	w.identity.Account = "999999999999"
	out, err := w.run(t)
	if err == nil {
		t.Fatalf("doctor passed against the wrong account:\n%s", out)
	}
	if !strings.Contains(out, "✗ account") || !strings.Contains(out, "999999999999") || !strings.Contains(out, testAccount) {
		t.Errorf("output does not name both accounts:\n%s", out)
	}
}

func TestDoctorCredentialFailure(t *testing.T) {
	w := healthyWorld()
	w.identityErr = errors.New("ExpiredToken: the security token included in the request is expired")
	out, err := w.run(t)
	if err == nil || !strings.Contains(err.Error(), "1 check failed") {
		t.Fatalf("error = %v, want exactly 1 failed check\n%s", err, out)
	}
	if !strings.Contains(out, "✗ credentials  ExpiredToken") {
		t.Errorf("output does not show the credential error:\n%s", out)
	}
	wantLines(t, out,
		"    if your session has expired, run: aws sso login --profile sandbox",
		"- account      skipped: no identity to check",
	)
}

func TestDoctorMissingToolsWarnWithoutFailing(t *testing.T) {
	w := healthyWorld()
	w.missing = map[string]bool{"docker": true, "aws": true}
	out, err := w.run(t)
	if err != nil {
		t.Fatalf("a missing tool failed doctor: %v\n%s", err, out)
	}
	wantLines(t, out,
		"! docker       not found: needed to build images, from the image module onward",
		"! aws cli      not found: needed for `aws sso login`",
	)
}

func TestDoctorRejectsPositionalArgs(t *testing.T) {
	if _, err := healthyWorld().run(t, "extra"); err == nil {
		t.Fatal("positional argument was accepted")
	}
}

func TestDoctorToolVersionReadsTheFirstLine(t *testing.T) {
	got, err := toolVersion(context.Background(), "go", "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "go version go") || strings.Contains(got, "\n") {
		t.Errorf("got %q, want one line starting with \"go version go\"", got)
	}
	if _, err := toolVersion(context.Background(), "symbiosis-no-such-tool"); err == nil {
		t.Error("a missing tool returned no error")
	}
}
