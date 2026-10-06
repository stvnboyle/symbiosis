// Command symbiosis is a small platform-as-a-service that runs in your own AWS account.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time with -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
