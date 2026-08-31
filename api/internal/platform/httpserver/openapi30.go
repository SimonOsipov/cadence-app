package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/yaml"
	"github.com/go-chi/chi/v5"
)

// serveThirty re-registers the two 3.0 documents so that they are valid 3.0.
//
// huma's downgrade rewrites a `type` *array* into `nullable: true` and misses a
// scalar `"type": "null"`, which is the only spelling 3.1 has for «this $ref or
// null» — so every property AdmitNull publishes reaches a 3.0.3 document
// carrying a type that 3.0 has no such value for. There is no spelling that is
// correct in both versions: 3.0 wants allOf + nullable, 3.1 wants oneOf + null.
//
// chi's last registration for a method and pattern wins, silently, so this
// replaces huma's handlers rather than colliding with them. That is undocumented
// behaviour of chi v5.3.1, the version this module builds against, measured on it
// — TestTheThirtyDocumentIsValid fails if a future chi keeps the first instead.
func serveThirty(router *chi.Mux, api huma.API) {
	var (
		once     sync.Once
		asJSON   []byte
		asYAML   []byte
		buildErr error
	)

	build := func() {
		once.Do(func() {
			asJSON, asYAML, buildErr = thirtyDocuments(api)
		})
	}

	serve := func(contentType string, pick func() []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			build()
			if buildErr != nil {
				http.Error(w, "the 3.0 document cannot be built", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(pick())
		}
	}

	router.Get(OpenAPIPath+"-3.0.json", serve("application/openapi+json", func() []byte { return asJSON }))
	router.Get(OpenAPIPath+"-3.0.yaml", serve("application/openapi+yaml", func() []byte { return asYAML }))
}

func thirtyDocuments(api huma.API) (asJSON, asYAML []byte, err error) {
	downgraded, err := api.OpenAPI().Downgrade()
	if err != nil {
		return nil, nil, fmt.Errorf("downgrading the OpenAPI document: %w", err)
	}

	var document any
	if err := json.Unmarshal(downgraded, &document); err != nil {
		return nil, nil, fmt.Errorf("reading the downgraded document: %w", err)
	}

	spellNullableForThirty(document)

	asJSON, err = json.Marshal(document)
	if err != nil {
		return nil, nil, fmt.Errorf("writing the 3.0 document: %w", err)
	}

	var buf bytes.Buffer
	if err := yaml.Convert(&buf, bytes.NewReader(asJSON)); err != nil {
		return nil, nil, fmt.Errorf("converting the 3.0 document to YAML: %w", err)
	}

	return asJSON, buf.Bytes(), nil
}

// spellNullableForThirty rewrites every «oneOf: [X, null]» into «allOf: [X],
// nullable: true», in place and at any depth.
//
// Only a two-member oneOf with exactly one null member is touched: a longer
// union has no 3.0 spelling at all, and silently collapsing one would publish a
// contract looser than the server's.
func spellNullableForThirty(node any) {
	switch value := node.(type) {
	case map[string]any:
		if members, ok := value["oneOf"].([]any); ok && len(members) == 2 {
			if kept, found := theNonNullOf(members); found {
				delete(value, "oneOf")
				value["allOf"] = []any{kept}
				value["nullable"] = true
			}
		}
		for _, child := range value {
			spellNullableForThirty(child)
		}
	case []any:
		for _, item := range value {
			spellNullableForThirty(item)
		}
	}
}

func theNonNullOf(members []any) (any, bool) {
	var kept any
	nulls := 0

	for _, member := range members {
		schema, ok := member.(map[string]any)
		if ok && len(schema) == 1 && schema["type"] == "null" {
			nulls++
			continue
		}
		kept = member
	}

	return kept, nulls == 1 && kept != nil
}
