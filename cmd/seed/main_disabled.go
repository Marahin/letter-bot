//go:build !devauth

// Command seed is a development-only tool: it writes demo data and must never run
// against a production database, so a build without the devauth tag refuses.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "cmd/seed is a development-only tool; build it with -tags devauth.")
	os.Exit(1)
}
