// Package testsupport is the test harness: a Postgres container with a
// throwaway database per test and the migration chain applied, and the signing
// keys, local JWK Set and token minting the authentication tests are written
// against.
//
// The two halves are gated differently, on purpose. The Postgres side is behind
// the `integration` build tag, because it needs a Docker daemon and belongs to
// the job that has one. The key side is not, because the authentication tests
// need no daemon and have to run in the fast gate — a tag there would mean a
// plain `go test ./...` silently compiles none of them, and a silently skipped
// authentication test is worse than a slow one.
//
// What keeps the whole package out of production is therefore depguard, not the
// tag: importing testsupport from anything other than a _test.go file fails the
// linter. A helper that mints a valid token cannot reach the binary, and the
// rule is a mechanism rather than an intention.
package testsupport
