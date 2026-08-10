// Command sqlauthorship fails the gate when a statement in the application
// packages is assembled from anything but constants.
//
// It is the mechanism behind one invariant: a value carried by a token cannot
// reach a SQL statement. The seam publishes the caller's claims as a bound
// parameter and impersonates through a bound parameter too, and this is what
// keeps the next edit from quietly composing one in instead — including for the
// statements gosec's G201/G202 do not recognise as SQL at all, SET and GRANT
// among them.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// say and sayf exist because this program's whole output is diagnostics, and a
// failure to write one is not a condition it can do anything about: the exit
// code is what the gate reads.
func say(w io.Writer, line string) {
	_, _ = fmt.Fprintln(w, line)
}

func sayf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is separated from main so the exit codes can be asserted. They are the
// whole product: a checker whose findings are printed while the process exits 0
// is a checker nobody is protected by.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("sqlauthorship", flag.ContinueOnError)
	flags.SetOutput(stderr)
	skip := flags.String("skip", "",
		"comma-separated directory names to leave alone, matched by path segment")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	roots := flags.Args()
	if len(roots) == 0 {
		say(stderr, "usage: sqlauthorship [-skip dir,dir] <directory>...")

		return 2
	}

	var (
		findings []Finding
		files    int
		skipped  []string
	)

	for _, root := range roots {
		report, err := Check(root, strings.Split(*skip, ","))
		if err != nil {
			sayf(stderr, "sqlauthorship: %v\n", err)

			return 1
		}

		findings = append(findings, report.Findings...)
		files += report.Files
		skipped = append(skipped, report.Skipped...)
	}

	// Said out loud on every run, because the alternative is that a mistyped
	// root, a renamed directory or a skip list that swallowed everything looks
	// exactly like a clean tree.
	sayf(stdout, "scanned %d file(s)", files)
	if len(skipped) > 0 {
		sayf(stdout, ", skipped %s", strings.Join(skipped, " "))
	}
	say(stdout, "")

	if files == 0 {
		say(stderr,
			"sqlauthorship: no Go files were read, so nothing was checked. "+
				"Check the roots and the skip list.")

		return 1
	}

	if len(findings) == 0 {
		return 0
	}

	for _, finding := range findings {
		say(stderr, finding.String())
	}
	sayf(stderr,
		"\n%d statement(s) assembled from something other than constants. "+
			"A statement takes its text from a constant of its own package and its values from bound "+
			"parameters; an identifier that has to vary belongs in a closed map resolved before the "+
			"call, not in the string.\n", len(findings))

	return 1
}
