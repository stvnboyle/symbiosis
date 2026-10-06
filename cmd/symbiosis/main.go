// Command symbiosis is a small platform-as-a-service that runs in your own AWS account.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time with -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("symbiosis version", version)
		return
	}
	fmt.Fprintln(os.Stderr, "symbiosis: no commands yet")
	os.Exit(1)
}
