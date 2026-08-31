package httpserver_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

type inner struct {
	Value float64 `json:"value"`
}

type outerOutput struct {
	Body outerBody
}

type outerBody struct {
	Optional *inner `json:"optional"`
	Required inner  `json:"required"`
	Scalar   *int   `json:"scalar"`
}

// A document with one operation, which is the least that puts a schema in the registry: huma
// registers a body's type when the operation carrying it is registered, not before.
func withOuterBody(t *testing.T) huma.API {
	t.Helper()

	api := httpserver.NewAPI(chi.NewRouter())
	huma.Register(api, huma.Operation{
		OperationID: "under-test", Method: http.MethodGet, Path: "/under-test",
	}, func(context.Context, *struct{}) (*outerOutput, error) { return nil, nil })

	return api
}

func property(t *testing.T, api huma.API, schema, name string) *huma.Schema {
	t.Helper()

	target := api.OpenAPI().Components.Schemas.SchemaFromRef("#/components/schemas/" + schema)
	if target == nil {
		t.Fatalf("no schema named %s", schema)
	}

	return target.Properties[name]
}

func TestARefIsRewrittenIntoTheObjectOrNull(t *testing.T) {
	api := withOuterBody(t)

	// The state it starts in, without which the assertion below holds over a rewrite that
	// never happened as readily as over one that did.
	if before := property(t, api, "OuterBody", "optional"); before.Ref == "" {
		t.Fatalf("the property is %+v before the rewrite, not a bare $ref", before)
	}

	httpserver.AdmitNull(api, "OuterBody", "optional")

	after := property(t, api, "OuterBody", "optional")
	if len(after.OneOf) != 2 {
		t.Fatalf("the property is %+v", after)
	}
	if after.Ref != "" {
		t.Errorf("the property is still a bare $ref: %+v", after)
	}
	if !strings.HasSuffix(after.OneOf[0].Ref, "/Inner") || after.OneOf[1].Type != "null" {
		t.Errorf("the property is %+v and %+v", after.OneOf[0], after.OneOf[1])
	}

	// Untouched: the rewrite is per property, and a neighbour turned nullable would make
	// every client's required field optional.
	if required := property(t, api, "OuterBody", "required"); required.Ref == "" {
		t.Errorf("the neighbouring property became %+v", required)
	}
}

// The two panics, and they are the point of the function rather than its edges: a schema
// renamed or a property that stopped being a $ref would otherwise publish a non-nullable
// object, and a generated client throws on the first null. Failing at startup is the answer.
func TestAdmitNullRefusesWhatItCannotRewrite(t *testing.T) {
	for _, refused := range []struct {
		name             string
		schema, property string
		says             string
	}{
		{"a schema nobody registered", "NoSuchBody", "optional", "no schema named NoSuchBody"},
		{"a property the schema has not got", "OuterBody", "absent", "OuterBody.absent is not a $ref"},
		{"a scalar, which needs none of this", "OuterBody", "scalar", "OuterBody.scalar is not a $ref"},
	} {
		t.Run(refused.name, func(t *testing.T) {
			api := withOuterBody(t)

			defer func() {
				raised, ok := recover().(string)
				if !ok {
					t.Fatalf("%s.%s was accepted", refused.schema, refused.property)
				}
				if !strings.Contains(raised, refused.says) {
					t.Errorf("the panic says %q, want it to name %q", raised, refused.says)
				}
			}()

			httpserver.AdmitNull(api, refused.schema, refused.property)
		})
	}
}
