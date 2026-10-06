package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/cobra"

	"github.com/stvnboyle/symbiosis/internal/awsx"
	"github.com/stvnboyle/symbiosis/internal/config"
)

// deps are the outside-world calls the commands make, replaced in tests.
type deps struct {
	loadConfig  func(path string) (config.Config, error)
	identity    func(ctx context.Context, profile, region string) (awsx.Identity, error)
	toolVersion func(ctx context.Context, name string, args ...string) (string, error)
}

func realDeps() deps {
	return deps{
		loadConfig: config.Load,
		identity: func(ctx context.Context, profile, region string) (awsx.Identity, error) {
			cfg, err := awsx.LoadConfig(ctx, profile, region)
			if err != nil {
				return awsx.Identity{}, err
			}
			return awsx.CallerIdentity(ctx, sts.NewFromConfig(cfg))
		},
		toolVersion: toolVersion,
	}
}

// toolVersion runs a tool's version command and returns the first line it prints.
func toolVersion(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(first), nil
}

type checkStatus int

const (
	checkOK checkStatus = iota
	checkWarn
	checkFail
	checkSkip
)

func (s checkStatus) marker() string {
	return [...]string{checkOK: "✓", checkWarn: "!", checkFail: "✗", checkSkip: "-"}[s]
}

type check struct {
	name   string
	status checkStatus
	detail string
	hint   string
}

func newDoctorCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check credentials, the pinned account, region and tool versions",
		Long: `Check that this machine is ready to run symbiosis.

doctor makes one AWS call, sts:GetCallerIdentity, which reads who you are and changes
nothing. Missing tools are reported as warnings; only a problem with the config,
credentials or account makes doctor fail.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			flags := cmd.Flags()
			configPath, _ := flags.GetString("config")
			profile, _ := flags.GetString("profile")
			region, _ := flags.GetString("region")

			checks := runDoctor(ctx, d, configPath, profile, region)
			if err := printChecks(cmd.OutOrStdout(), checks); err != nil {
				return err
			}
			failed := 0
			for _, c := range checks {
				if c.status == checkFail {
					failed++
				}
			}
			switch failed {
			case 0:
				return nil
			case 1:
				return fmt.Errorf("doctor: 1 check failed")
			default:
				return fmt.Errorf("doctor: %d checks failed", failed)
			}
		},
	}
}

// runDoctor runs every check. A --profile or --region flag overrides the config file.
func runDoctor(ctx context.Context, d deps, configPath, profile, region string) []check {
	var checks []check

	cfg, cfgErr := d.loadConfig(configPath)
	if cfgErr != nil {
		checks = append(checks, check{name: "config", status: checkFail, detail: cfgErr.Error()})
	} else {
		checks = append(checks, check{name: "config", status: checkOK, detail: configPath})
	}
	if profile == "" {
		profile = cfg.Profile
	}
	if region == "" {
		region = cfg.Region
	}

	var (
		id       awsx.Identity
		identErr error
	)
	switch {
	case profile == "" || region == "":
		identErr = fmt.Errorf("no profile or region: set them in %s or pass --profile and --region", configPath)
		checks = append(checks, check{name: "credentials", status: checkFail, detail: identErr.Error()})
	default:
		id, identErr = d.identity(ctx, profile, region)
		if identErr != nil {
			checks = append(checks, check{
				name: "credentials", status: checkFail, detail: identErr.Error(),
				hint: "if your session has expired, run: aws sso login --profile " + profile,
			})
		} else {
			checks = append(checks, check{name: "credentials", status: checkOK, detail: id.ARN})
		}
	}

	switch {
	case cfgErr != nil:
		checks = append(checks, check{name: "account", status: checkSkip, detail: "skipped: no pinned account to check against"})
	case identErr != nil:
		checks = append(checks, check{name: "account", status: checkSkip, detail: "skipped: no identity to check"})
	default:
		if err := awsx.CheckPin(cfg.AccountID, id); err != nil {
			checks = append(checks, check{name: "account", status: checkFail, detail: err.Error()})
		} else {
			checks = append(checks, check{name: "account", status: checkOK, detail: id.Account + " matches the pinned account"})
		}
	}

	if region != "" {
		checks = append(checks, check{name: "region", status: checkOK, detail: region})
	}

	for _, tool := range []struct {
		label, bin, arg, neededFor string
	}{
		{"go", "go", "version", "needed to build symbiosis from source"},
		{"docker", "docker", "--version", "needed to build images, from the image module onward"},
		{"aws cli", "aws", "--version", "needed for `aws sso login`"},
	} {
		version, err := d.toolVersion(ctx, tool.bin, tool.arg)
		if err != nil {
			checks = append(checks, check{name: tool.label, status: checkWarn, detail: "not found: " + tool.neededFor})
		} else {
			checks = append(checks, check{name: tool.label, status: checkOK, detail: version})
		}
	}
	return checks
}

func printChecks(w io.Writer, checks []check) error {
	width := 0
	for _, c := range checks {
		width = max(width, len(c.name))
	}
	var b strings.Builder
	for _, c := range checks {
		fmt.Fprintf(&b, "%s %-*s  %s\n", c.status.marker(), width, c.name, c.detail)
		if c.hint != "" {
			fmt.Fprintf(&b, "    %s\n", c.hint)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}
