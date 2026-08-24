package httpserver

import (
	"encoding/json"
	"testing"
)

// The document served today carries one shape — a two-member oneOf whose second
// member is null — so the guards around that shape are unmeasured by it. These
// are the shapes a later contract can grow, and each one the rewrite must leave
// alone.
func TestOnlyANullableRefIsRespelled(t *testing.T) {
	for _, c := range []struct {
		name  string
		given string
		want  string
	}{
		{
			name:  "a nullable $ref becomes 3.0's spelling",
			given: `{"oneOf":[{"$ref":"#/x"},{"type":"null"}]}`,
			want:  `{"allOf":[{"$ref":"#/x"}],"nullable":true}`,
		},
		{
			name:  "the null member may come first",
			given: `{"oneOf":[{"type":"null"},{"$ref":"#/x"}]}`,
			want:  `{"allOf":[{"$ref":"#/x"}],"nullable":true}`,
		},
		{
			name:  "a description survives, because it is what the property says",
			given: `{"description":"d","oneOf":[{"$ref":"#/x"},{"type":"null"}]}`,
			want:  `{"allOf":[{"$ref":"#/x"}],"description":"d","nullable":true}`,
		},
		{
			// Collapsing this would publish a contract looser than the server's:
			// allOf of one member drops the other alternative entirely.
			name:  "a three-member union is left alone",
			given: `{"oneOf":[{"$ref":"#/x"},{"$ref":"#/y"},{"type":"null"}]}`,
			want:  `{"oneOf":[{"$ref":"#/x"},{"$ref":"#/y"},{"type":"null"}]}`,
		},
		{
			name:  "a union with no null member is left alone",
			given: `{"oneOf":[{"$ref":"#/x"},{"$ref":"#/y"}]}`,
			want:  `{"oneOf":[{"$ref":"#/x"},{"$ref":"#/y"}]}`,
		},
		{
			// A member carrying more than the type is a schema in its own right,
			// not 3.1's way of writing «or null».
			name:  "a null member that says more than its type is left alone",
			given: `{"oneOf":[{"$ref":"#/x"},{"type":"null","title":"t"}]}`,
			want:  `{"oneOf":[{"$ref":"#/x"},{"title":"t","type":"null"}]}`,
		},
		{
			// Not a shape huma produces, and the reason the rewrite asks whether
			// a member survived at all: without that question this publishes
			// «allOf: [null]», which is not a schema.
			name:  "a union whose other member is a literal null is left alone",
			given: `{"oneOf":[null,{"type":"null"}]}`,
			want:  `{"oneOf":[null,{"type":"null"}]}`,
		},
		{
			name:  "it reaches a property nested inside a schema",
			given: `{"components":{"schemas":{"S":{"properties":{"p":{"oneOf":[{"$ref":"#/x"},{"type":"null"}]}}}}}}`,
			want:  `{"components":{"schemas":{"S":{"properties":{"p":{"allOf":[{"$ref":"#/x"}],"nullable":true}}}}}}`,
		},
		{
			name:  "it reaches a property inside an array",
			given: `{"a":[{"oneOf":[{"$ref":"#/x"},{"type":"null"}]}]}`,
			want:  `{"a":[{"allOf":[{"$ref":"#/x"}],"nullable":true}]}`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var document any
			if err := json.Unmarshal([]byte(c.given), &document); err != nil {
				t.Fatalf("the case is not JSON: %v", err)
			}

			spellNullableForThirty(document)

			got, err := json.Marshal(document)
			if err != nil {
				t.Fatalf("marshalling the result: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}
