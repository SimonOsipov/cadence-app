package civil_test

import (
	"go/build"
	"slices"
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
	// XTestImports as well: this file is in package civil_test, so an external test
	// here could import a context without the compiler seeing a cycle.
	all := append(append([]string{}, pkg.Imports...), pkg.TestImports...)
	for _, imported := range append(all, pkg.XTestImports...) {
		if strings.HasPrefix(imported, ours) {
			t.Errorf("civil imports %s", imported)
		}
	}

	// The whole list, and it is short on purpose: each entry is a decision, so adding
	// one fails here and has to be argued rather than noticed later. time carries
	// Month, Weekday and the arithmetic behind AddDays; fmt carries the two String
	// methods, which are fixed-width because a variable-width date is another date.
	want := []string{"fmt", "time"}
	if !slices.Equal(pkg.Imports, want) {
		t.Errorf("civil imports %v, want exactly %v", pkg.Imports, want)
	}
}
