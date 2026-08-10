package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The exit code is the product. Findings printed while the process exits 0 leave
// the gate green and the operator none the wiser, so the code gets its own tests
// rather than being assumed from the fact that findings exist.
func TestRunExitCodes(t *testing.T) {
	clean := t.TempDir()
	writeProbe(t, clean, "probe.go", `
		func f(ctx context.Context, c conn) { _ = c.Exec(ctx, "SELECT 1") }`)

	dirty := t.TempDir()
	writeProbe(t, dirty, "probe.go", `
		func f(ctx context.Context, c conn, role string) { _ = c.Exec(ctx, "SET ROLE " + role) }`)

	empty := t.TempDir()

	cases := map[string]struct {
		args []string
		want int
	}{
		"a clean tree":               {args: []string{clean}, want: 0},
		"a statement from a value":   {args: []string{dirty}, want: 1},
		"no roots given":             {args: nil, want: 2},
		"a root with no Go files":    {args: []string{empty}, want: 1},
		"a root that does not exist": {args: []string{filepath.Join(empty, "nowhere")}, want: 1},
		// The root is never skipped by name, so this is a run that reads the
		// root and reports its finding — not a run that skipped everything.
		// Skipping everything is covered by "a root with no Go files".
		"a skip list naming the root": {args: []string{"-skip", filepath.Base(dirty), dirty}, want: 1},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tc.args, &stdout, &stderr); got != tc.want {
				t.Errorf("run(%v) = %d, want %d\nstdout: %s\nstderr: %s",
					tc.args, got, tc.want, stdout.String(), stderr.String())
			}
		})
	}
}

// What was read is reported on every run, clean or not: "scanned 0 files" and
// "scanned 87 files" are the same silence otherwise.
func TestRunReportsWhatItRead(t *testing.T) {
	dir := t.TempDir()
	writeProbe(t, dir, "probe.go", `
		func f(ctx context.Context, c conn) { _ = c.Exec(ctx, "SELECT 1") }`)

	harness := filepath.Join(dir, "testsupport")
	if err := os.Mkdir(harness, 0o750); err != nil {
		t.Fatalf("creating the harness directory: %v", err)
	}
	writeProbe(t, harness, "probe.go", `
		func f(ctx context.Context, c conn) { _ = c.Exec(ctx, "SELECT 1") }`)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"-skip", "testsupport", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, want 0\nstderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "scanned 1 file(s)") {
		t.Errorf("stdout = %q, want the file count", stdout.String())
	}
	if !strings.Contains(stdout.String(), "testsupport") {
		t.Errorf("stdout = %q, want the skipped directory named", stdout.String())
	}
}

func writeProbe(t *testing.T, dir, name, body string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(probeHeader+body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}
