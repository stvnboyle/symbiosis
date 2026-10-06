package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var errNotImplemented = errors.New("not implemented")

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "symbiosis",
		Short:   "A small platform-as-a-service that runs in your own AWS account",
		Version: version,
		// Errors are printed once, by main. A failed AWS call should not dump usage.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("profile", "", "AWS named profile to use")
	root.PersistentFlags().String("region", "", "AWS region to operate in")

	root.AddCommand(
		stubCmd("bootstrap", "Prepare an AWS account for symbiosis"),
		stubCmd("plan", "Show what would change, without changing anything"),
		stubCmd("status", "List owned resources and whether they match desired state"),
		stubCmd("doctor", "Check credentials, the pinned account, region and tool versions"),
		stubCmd("destroy", "Remove everything symbiosis created"),
	)
	return root
}

func stubCmd(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("%s: %w", name, errNotImplemented)
		},
	}
}
