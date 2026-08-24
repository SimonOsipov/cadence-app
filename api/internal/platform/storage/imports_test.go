package storage_test

import (
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/SimonOsipov/cadence-app/api/"

// The driver is one-way: the store knows buckets, keys and expiry, and nothing
// about the contexts that decide who may see an object. An import the other way
// would put the visibility check inside the thing being guarded.
func TestTheStoreDoesNotImportABoundedContext(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}

	// Reconciled against internal/ rather than listed: a context added later is
	// absent from a list, and absence there reads as exemption.
	contexts, err := os.ReadDir(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("reading internal/: %v", err)
	}

	// XTestImports too, so an external test file is not the exempt way in.
	all := append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...)

	for _, entry := range contexts {
		// platform holds this package and its siblings; router is the registry.
		if !entry.IsDir() || entry.Name() == "platform" || entry.Name() == "router" {
			continue
		}
		for _, imported := range all {
			if imported == module+"internal/"+entry.Name() {
				t.Errorf("storage imports the %s context", entry.Name())
			}
		}
	}
}

// The rule the project states as «the core does not import drivers»: the SDK is
// reachable from exactly one package, so a context that wants a link has to ask
// through the interface it declared rather than sign one itself.
//
// It asks the source tree rather than holding a list of permitted packages,
// because a package added next month is absent from a list — and absence is
// what this is looking for.
func TestOnlyThisPackageImportsTheSDK(t *testing.T) {
	const sdk = "github.com/aws/aws-sdk-go-v2"

	root := filepath.Join("..", "..", "..")
	here, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolving this package: %v", err)
	}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// The vendored module cache and the build output are not ours.
			if entry.Name() == "vendor" || entry.Name() == "bin" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		absolute, err := filepath.Abs(filepath.Dir(path))
		if err != nil {
			return err
		}
		if absolute == here {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(source), sdk) {
			t.Errorf("%s reaches the S3 SDK directly", path)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
}
