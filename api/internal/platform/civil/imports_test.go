package civil_test

import (
	"go/build"
	"strings"
	"testing"
)

// The shared kernel depends on nothing of ours, and that is the whole of what makes
// it shared: a vocabulary that reached back into a context would be that context's
// vocabulary wearing a different import path.
//
// Nor does it read the clock. Every function here takes the day it is about, which
// is what lets four suites run a calendar rather than whatever day it is.
func TestCivilDependsOnNothingOfOurs(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}

	const ours = "github.com/SimonOsipov/cadence-app/api/"
	for _, imported := range append(append([]string{}, pkg.Imports...), pkg.TestImports...) {
		if strings.HasPrefix(imported, ours) {
			t.Errorf("civil imports %s", imported)
		}
	}

	// time is the one dependency it has, and it needs it: Month, Weekday and the
	// arithmetic behind AddDays.
	if len(pkg.Imports) != 1 || pkg.Imports[0] != "time" {
		t.Errorf("civil imports %v, want exactly [time]", pkg.Imports)
	}
}
