// Command symbiosis is a small platform-as-a-service that runs in your own AWS account.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// version is overridden at build time with -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	os.Exit(realMain())
}

func realMain() int {
	// Ctrl-C cancels the context, so a command stops cleanly between AWS calls.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
