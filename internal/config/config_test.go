package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const valid = `
# a comment
account_id: "012345678901"
region: eu-west-2   # trailing comment
profile: symbiosis-admin
alert_email: 'me@example.com'
`

func TestParseValid(t *testing.T) {
	got, err := Parse(strings.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		AccountID:  "012345678901",
		Region:     "eu-west-2",
		Profile:    "symbiosis-admin",
		AlertEmail: "me@example.com",
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// An unquoted account id must keep its leading zero. A general YAML parser would read
// it as a number and drop it.
func TestParseKeepsLeadingZeroInUnquotedAccountID(t *testing.T) {
	in := strings.Replace(valid, `"012345678901"`, `012345678901`, 1)
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountID != "012345678901" {
		t.Errorf("AccountID = %q, want the leading zero kept", got.AccountID)
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name, in, wantErr string
	}{
		{"unknown key", valid + "acount_id: 1\n", `line 7: unknown key "acount_id"`},
		{"duplicate key", valid + "region: us-east-1\n", `line 7: "region" is set twice`},
		{"no colon", "just some text\n", `line 1: want "key: value"`},
		{"nested", "profile:\n  name: admin\n", "line 2: nested or indented values are not supported"},
		{"list", "- eu-west-2\n", "line 1: lists are not supported"},
		{"unbalanced quote", "profile: \"oops\n", "line 1: unbalanced quote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.in))
			if err == nil {
				t.Fatal("got no error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	base := Config{AccountID: "123456789012", Region: "eu-west-2", Profile: "p", AlertEmail: "a@b.co"}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"account id missing", func(c *Config) { c.AccountID = "" }, "account_id is required"},
		{"account id too short", func(c *Config) { c.AccountID = "12345" }, "account_id must be exactly 12 digits"},
		{"account id not digits", func(c *Config) { c.AccountID = "12345678901x" }, "account_id must be exactly 12 digits"},
		{"region missing", func(c *Config) { c.Region = "" }, "region is required"},
		{"region malformed", func(c *Config) { c.Region = "London" }, `region "London" does not look like an AWS region`},
		{"profile missing", func(c *Config) { c.Profile = "" }, "profile is required"},
		{"email missing", func(c *Config) { c.AlertEmail = "" }, "alert_email is required"},
		{"email malformed", func(c *Config) { c.AlertEmail = "not-an-email" }, `alert_email "not-an-email" is not a valid email address`},
		{"email with display name", func(c *Config) { c.AlertEmail = "Me <a@b.co>" }, "is not a valid email address"},
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base
			tt.mutate(&c)
			err := c.Validate()
			if err == nil {
				t.Fatal("got no error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	err := Config{}.Validate()
	for _, want := range []string{"account_id", "region", "profile", "alert_email"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error %v does not mention %s", err, want)
		}
	}
}

func TestRegionsAccepted(t *testing.T) {
	for _, region := range []string{"eu-west-2", "us-east-1", "ap-southeast-4", "us-gov-west-1"} {
		c := Config{AccountID: "123456789012", Region: region, Profile: "p", AlertEmail: "a@b.co"}
		if err := c.Validate(); err != nil {
			t.Errorf("%s rejected: %v", region, err)
		}
	}
}

func TestLoadMissingFileSaysWhatToDo(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "symbiosis.yaml"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want it to wrap fs.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "symbiosis.example.yaml") {
		t.Errorf("error %q does not point at the example file", err)
	}
}

func TestLoadReadsAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "symbiosis.yaml")
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("region: eu-west-2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "account_id is required") {
		t.Errorf("error = %v, want the path and the validation failure", err)
	}
}

func TestExampleFileIsValid(t *testing.T) {
	if _, err := Load("../../symbiosis.example.yaml"); err != nil {
		t.Errorf("the committed example does not load: %v", err)
	}
}
