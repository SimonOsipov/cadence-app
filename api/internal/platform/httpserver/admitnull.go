package httpserver

import "github.com/danielgtaylor/huma/v2"

// AdmitNull rewrites a property that is a bare $ref into «this object or null».
//
// huma cannot express it: `nullable:"true"` over a $ref panics («nullable is not supported for
// field … which is type '#/components/schemas/…'»), and without the tag a generator reads the
// property as non-nullable — so a client types it MacrosBody and throws on the first patient
// with no nutrition context. Scalars need none of this.
func AdmitNull(api huma.API, schema string, properties ...string) {
	registry := api.OpenAPI().Components.Schemas
	target := registry.SchemaFromRef("#/components/schemas/" + schema)
	if target == nil {
		panic("AdmitNull: no schema named " + schema)
	}
	for _, name := range properties {
		property, ok := target.Properties[name]
		if !ok || property.Ref == "" {
			panic("AdmitNull: " + schema + "." + name + " is not a $ref")
		}
		target.Properties[name] = &huma.Schema{
			Description: property.Description,
			OneOf: []*huma.Schema{
				{Ref: property.Ref},
				// huma names every type but this one, because it never emits it
				// for a $ref — which is the whole reason this function exists.
				{Type: "null"},
			},
		}
	}
}
