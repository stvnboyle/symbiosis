package awsx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolate points the SDK at a temporary shared config file, so tests never read the
// developer's real ~/.aws or environment.
func isolate(t *testing.T, sharedConfig string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(sharedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", path)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	for _, name := range []string{"AWS_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN"} {
		t.Setenv(name, "")
	}
}

func TestSessionLoadConfigUsesTheNamedProfileAndRegion(t *testing.T) {
	isolate(t, "[profile sandbox]\nregion = us-east-1\n")
	cfg, err := LoadConfig(context.Background(), "sandbox", "eu-west-2")
	if err != nil {
		t.Fatal(err)
	}
	// The region symbiosis is pinned to wins over the profile's own default.
	if cfg.Region != "eu-west-2" {
		t.Errorf("Region = %q, want eu-west-2", cfg.Region)
	}
}

func TestSessionLoadConfigUnknownProfileSaysWhatToDo(t *testing.T) {
	isolate(t, "[profile other]\nregion = us-east-1\n")
	_, err := LoadConfig(context.Background(), "sandbox", "eu-west-2")
	if err == nil {
		t.Fatal("unknown profile was accepted")
	}
	for _, want := range []string{`"sandbox"`, "aws configure sso"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestSessionLoadConfigRequiresProfileAndRegion(t *testing.T) {
	isolate(t, "[profile sandbox]\n")
	if _, err := LoadConfig(context.Background(), "", "eu-west-2"); err == nil {
		t.Error("empty profile was accepted")
	}
	if _, err := LoadConfig(context.Background(), "sandbox", ""); err == nil {
		t.Error("empty region was accepted")
	}
}
