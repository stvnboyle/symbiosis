// Package config loads symbiosis.yaml, the local file that pins symbiosis to one AWS
// account and region.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/mail"
	"os"
	"regexp"
	"strings"
)

// Config is the content of symbiosis.yaml.
type Config struct {
	// AccountID is the only AWS account symbiosis may touch. It is a string because
	// account ids can start with zero.
	AccountID  string
	Region     string
	Profile    string
	AlertEmail string
}

// Load reads and validates the config file at path.
func Load(path string) (Config, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, fmt.Errorf("%s not found: copy symbiosis.example.yaml to %s and fill it in: %w", path, path, err)
	}
	if err != nil {
		return Config{}, err
	}
	defer f.Close() //nolint:errcheck // read-only file; nothing to lose on close

	c, err := Parse(f)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Parse reads a flat "key: value" file. It accepts the subset of YAML the config
// needs and rejects everything else, so every value stays a string exactly as written.
func Parse(r io.Reader) (Config, error) {
	var c Config
	fields := map[string]*string{
		"account_id":  &c.AccountID,
		"region":      &c.Region,
		"profile":     &c.Profile,
		"alert_email": &c.AlertEmail,
	}
	seen := map[string]bool{}

	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if raw[0] == ' ' || raw[0] == '\t' {
			return Config{}, fmt.Errorf("line %d: nested or indented values are not supported", n)
		}
		if strings.HasPrefix(line, "- ") {
			return Config{}, fmt.Errorf("line %d: lists are not supported", n)
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return Config{}, fmt.Errorf(`line %d: want "key: value", got %q`, n, line)
		}
		key = strings.TrimSpace(key)
		field, known := fields[key]
		if !known {
			return Config{}, fmt.Errorf("line %d: unknown key %q", n, key)
		}
		if seen[key] {
			return Config{}, fmt.Errorf("line %d: %q is set twice", n, key)
		}
		seen[key] = true

		value, err := unquote(value)
		if err != nil {
			return Config{}, fmt.Errorf("line %d: %w", n, err)
		}
		*field = value
	}
	if err := scanner.Err(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// unquote trims a value, drops a trailing comment and removes one pair of matching
// quotes.
func unquote(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if q := value[0]; q == '"' || q == '\'' {
		end := strings.IndexByte(value[1:], q)
		if end < 0 {
			return "", errors.New("unbalanced quote")
		}
		rest := strings.TrimSpace(value[end+2:])
		if rest != "" && !strings.HasPrefix(rest, "#") {
			return "", fmt.Errorf("unexpected text after closing quote: %q", rest)
		}
		return value[1 : end+1], nil
	}
	if i := strings.Index(value, " #"); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return value, nil
}

var (
	accountIDPattern = regexp.MustCompile(`^\d{12}$`)
	regionPattern    = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d+$`)
)

// Validate checks every field and reports all problems together.
func (c Config) Validate() error {
	var problems []error
	switch {
	case c.AccountID == "":
		problems = append(problems, errors.New("account_id is required"))
	case !accountIDPattern.MatchString(c.AccountID):
		problems = append(problems, errors.New("account_id must be exactly 12 digits"))
	}
	switch {
	case c.Region == "":
		problems = append(problems, errors.New("region is required"))
	case !regionPattern.MatchString(c.Region):
		problems = append(problems, fmt.Errorf("region %q does not look like an AWS region such as eu-west-2", c.Region))
	}
	if c.Profile == "" {
		problems = append(problems, errors.New("profile is required"))
	}
	switch addr, err := mail.ParseAddress(c.AlertEmail); {
	case c.AlertEmail == "":
		problems = append(problems, errors.New("alert_email is required"))
	case err != nil || addr.Address != c.AlertEmail:
		problems = append(problems, fmt.Errorf("alert_email %q is not a valid email address", c.AlertEmail))
	}
	return errors.Join(problems...)
}
