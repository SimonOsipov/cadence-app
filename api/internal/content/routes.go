package content

import "github.com/danielgtaylor/huma/v2"

// Register mounts this context's operations on the API.
//
// It is empty until the context has its first endpoint. It exists now so that
// the endpoint arrives as a call in a place that is already wired, reviewed and
// covered by the registry test in internal/router — rather than as a call plus
// a decision about where routing lives, made under the pressure of a feature.
func Register(huma.API) {}
