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
var contexts = []boundedContext{
	{"audit", audit.Register},
	{"content", content.Register},
	{"dosing", dosing.Register},
	{"identity", identity.Register},
	{"inventory", inventory.Register},
	{"journal", journal.Register},
	{"measurements", measurements.Register},
	{"messaging", messaging.Register},
	{"notifications", notifications.Register},
	{"nutrition", nutrition.Register},
	{"protocol", protocol.Register},
}

// Register mounts every bounded context's operations on the API.
func Register(api huma.API) {
	for _, c := range contexts {
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
	// not a server to serve from.
	api := httpserver.NewAPI(chi.NewRouter())
	Register(api)

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
