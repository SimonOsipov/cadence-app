package identity

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Register mounts this context's operations on the API.
//
// One entry per operation, in the context that owns it — the registry in
// internal/router knows the list of contexts, not the list of routes.
func Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/v1/me",
		Summary:     "The authenticated caller",
		Description: "Returns the identity carried by the presented token. " +
			"It reads no database: a role changed there takes effect when the token expires.",
		Tags: []string{"identity"},
	}, me)
}
