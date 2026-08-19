// Package router wires the bounded contexts to the HTTP transport and builds
// the OpenAPI document from what they registered.
//
// It is the one place that knows the whole list, which is what makes "every
// context is mounted" a testable statement rather than a habit.
package router

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/audit"
	"github.com/SimonOsipov/cadence-app/api/internal/content"
	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/journal"
	"github.com/SimonOsipov/cadence-app/api/internal/measurements"
	"github.com/SimonOsipov/cadence-app/api/internal/messaging"
	"github.com/SimonOsipov/cadence-app/api/internal/notifications"
	"github.com/SimonOsipov/cadence-app/api/internal/nutrition"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

type boundedContext struct {
	name     string
	register func(huma.API)
}

// contexts is every bounded context that serves HTTP. The registry test in this
// package compares it against the directories under internal/, so a context
// added and not listed here fails the build rather than going quietly unmounted.
//
// A function rather than a variable since one context needs the pools and the
// provisioner to answer. Its registrar is still that context's own method, so
// the test that checks each entry against the package it came from keeps
// working — a closure declared here would belong to this package and pass a
// check that is meant to catch exactly that mistake.
func contexts(opts Options) []boundedContext {
	// Nil is what the document generator passes, and the operations are declared
	// either way: openapi.json is the shape of the API, not of this deployment's
	// dependencies.
	var onboarding *identity.Onboarding
	if opts.Pool != nil && opts.ServicePool != nil && opts.Provisioner != nil {
		onboarding = identity.NewOnboarding(opts.Pool, opts.ServicePool, opts.Provisioner)
	}

	// The request pool alone: this one writes the caller's own row under the
	// caller's own identity, so it needs neither the service path nor the
	// identity provider. A nil pool yields a nil service.
	sessions := identity.NewSessions(opts.Pool)
	roster := identity.NewRoster(opts.Pool, opts.Provisioner)
	// Assigned through a variable of the interface type rather than straight into Deps: a nil
	// *identity.Profiles put in an interface field is an interface that is not nil, and the handler's
	// nil check would pass while every call dereferenced nothing.
	var profiles identity.ProfileReader
	if reader := identity.NewProfiles(opts.Pool); reader != nil {
		profiles = reader
	}

	return []boundedContext{
		{"audit", audit.Register},
		{"content", content.Register},
		{"dosing", dosing.Register},
		{"identity", identity.NewService(identity.Deps{
			Onboarding: onboarding,
			Sessions:   sessions,
			Roster:     roster,
			Profiles:   profiles,
		}).Register},
		{"inventory", inventory.Register},
		{"journal", journal.Register},
		{"measurements", measurements.Register},
		{"messaging", messaging.Register},
		{"notifications", notifications.Register},
		{"nutrition", nutrition.Register},
		{"protocol", protocol.Register},
	}
}

// Register mounts every bounded context's operations on the API.
func Register(api huma.API, opts Options) {
	for _, c := range contexts(opts) {
		c.register(api)
	}
}

// Document returns the OpenAPI document of the whole API.
//
// This is the only definition of the bytes: cmd/openapi writes what it returns
// and the drift test compares against what it returns, so the committed file
// and the generator cannot disagree about formatting and produce a diff that
// looks like a contract change.
func Document() ([]byte, error) {
	// A throwaway router: building the document needs an API to register onto,
	// not a server to serve from. No options either — the document is the shape
	// of the operations, and none of it depends on what they answer from.
	api := httpserver.NewAPI(chi.NewRouter())
	Register(api, Options{})

	spec, err := api.OpenAPI().MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshalling the OpenAPI document: %w", err)
	}

	// Indented and newline-terminated: the file is read in review far more often
	// than it is read by a generator, and a one-line document makes a renamed
	// field indistinguishable from a rewritten contract.
	var indented bytes.Buffer
	if err := json.Indent(&indented, spec, "", "  "); err != nil {
		return nil, fmt.Errorf("indenting the OpenAPI document: %w", err)
	}
	indented.WriteByte('\n')

	return indented.Bytes(), nil
}
