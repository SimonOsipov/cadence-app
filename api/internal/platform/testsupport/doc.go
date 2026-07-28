// Package testsupport brings up real infrastructure for the integration suite:
// a Postgres container, a throwaway database per test with the migration chain
// applied, and the connection strings of each role involved.
//
// Everything in it is behind the `integration` build tag, so this file is all
// that remains in a normal build. That is deliberate — the package must exist
// for tooling, and must contain nothing that could be linked into the API.
//
// Two mechanisms keep it out of production, because a comment is not a
// mechanism: the build tag above, and a depguard rule that forbids importing
// this package from anywhere but a _test.go file. A helper that weakens
// authentication for convenience cannot reach the binary through either.
package testsupport
