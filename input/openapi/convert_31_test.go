package openapi

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gofoji/foji/input"
)

// OpenAPI 3.1 expresses nullability as a "null" entry in the type array;
// 3.0 uses the nullable flag. The converter normalizes the 3.1 form to
// Type + Nullable so downstream consumers handle both dialects identically.
func TestParse_31NullableTypeArray(t *testing.T) {
	spec := []byte(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths:
  /test:
    get:
      operationId: testOp
      responses:
        "200":
          description: OK
components:
  schemas:
    Foo:
      type: object
      properties:
        plain:
          type: string
        maybe:
          type: [string, "null"]
        maybeTime:
          type: ["null", string]
          format: date-time
`)

	inGroups := []input.FileGroup{
		{
			Files: []input.File{
				{Source: "test.yaml", Name: "test.yaml", Content: spec},
			},
		},
	}

	result, err := Parse(context.Background(), zerolog.Nop(), inGroups)
	require.NoError(t, err)

	foo := result[0][0].API.Components.Schemas["Foo"]
	require.NotNil(t, foo)

	plain := foo.Value.Properties["plain"]
	require.NotNil(t, plain)
	assert.Equal(t, []string{"string"}, []string(plain.Value.Type))
	assert.False(t, plain.Value.Nullable)

	for _, name := range []string{"maybe", "maybeTime"} {
		prop := foo.Value.Properties[name]
		require.NotNil(t, prop, name)
		assert.Equal(t, []string{"string"}, []string(prop.Value.Type), name)
		assert.True(t, prop.Value.Nullable, name)
	}

	assert.Equal(t, "date-time", foo.Value.Properties["maybeTime"].Value.Format)
}
