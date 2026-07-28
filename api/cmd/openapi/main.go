// Command openapi writes the OpenAPI document to a file.
//
// The contract is generated from the Go types, so it has to be regenerated
// whenever they change — and then committed, because it is what the Kotlin
// client and the dashboard client are generated from. A test in internal/router
// fails when the committed file and the types disagree, which turns a silent
// rename into a diff someone has to approve.
//
// It builds the API without starting a server: no port, no database, nothing to
// tear down, so it is safe to run from a Makefile target or a CI step.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/SimonOsipov/cadence-app/api/internal/router"
)

func main() {
	out := flag.String("out", "openapi.json", "path to write the document to; - writes to stdout")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintf(os.Stderr, "openapi: %v\n", err)
		os.Exit(1)
	}
}

func run(out string) error {
	document, err := router.Document()
	if err != nil {
		return err
	}

	if out == "-" {
		if _, err := os.Stdout.Write(document); err != nil {
			return fmt.Errorf("writing to stdout: %w", err)
		}

		return nil
	}

	if err := os.WriteFile(out, document, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", out, err)
	}

	fmt.Fprintf(os.Stderr, "openapi: wrote %s\n", out)

	return nil
}
