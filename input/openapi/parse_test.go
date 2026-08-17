package openapi

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gofoji/foji/input"
)

func TestParse_Success(t *testing.T) {
	spec := []byte(`openapi: "3.0.2"
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
        name:
          type: string
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
	require.Len(t, result, 1)
	require.Len(t, result[0], 1)

	f := result[0][0]
	assert.Equal(t, "test.yaml", f.Input.Source)
	assert.NotNil(t, f.API)
	assert.Equal(t, "Test", f.API.Info.Title)
	assert.NotNil(t, f.API.Paths.Find("/test"))
	assert.Contains(t, f.API.Components.Schemas, "Foo")
}

func TestParse_MultipleFiles(t *testing.T) {
	spec1 := []byte(`openapi: "3.0.2"
info:
  title: Spec1
  version: "1.0"
paths: {}
`)
	spec2 := []byte(`openapi: "3.0.2"
info:
  title: Spec2
  version: "2.0"
paths: {}
`)

	inGroups := []input.FileGroup{
		{
			Files: []input.File{
				{Source: "a.yaml", Name: "a.yaml", Content: spec1},
				{Source: "b.yaml", Name: "b.yaml", Content: spec2},
			},
		},
	}

	result, err := Parse(context.Background(), zerolog.Nop(), inGroups)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Len(t, result[0], 2)
	assert.Equal(t, "Spec1", result[0][0].API.Info.Title)
	assert.Equal(t, "Spec2", result[0][1].API.Info.Title)
}

func TestParse_MultipleGroups(t *testing.T) {
	spec := []byte(`openapi: "3.0.2"
info:
  title: Test
  version: "1.0"
paths: {}
`)

	inGroups := []input.FileGroup{
		{Files: []input.File{{Source: "a.yaml", Content: spec}}},
		{Files: []input.File{{Source: "b.yaml", Content: spec}}},
	}

	result, err := Parse(context.Background(), zerolog.Nop(), inGroups)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Len(t, result[0], 1)
	require.Len(t, result[1], 1)
}

func TestParse_EmptyGroups(t *testing.T) {
	result, err := Parse(context.Background(), zerolog.Nop(), nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParse_EmptyFileGroup(t *testing.T) {
	inGroups := []input.FileGroup{{}}

	result, err := Parse(context.Background(), zerolog.Nop(), inGroups)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Empty(t, result[0])
}

// A self-referential schema must resolve to a single shared *spec.Schema rather
// than recursing forever while being converted.
func TestParse_RecursiveRef(t *testing.T) {
	spec := []byte(`openapi: "3.0.2"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Node:
      type: object
      properties:
        name:
          type: string
        children:
          type: array
          items:
            $ref: "#/components/schemas/Node"
`)

	result, err := Parse(context.Background(), zerolog.Nop(), []input.FileGroup{
		{Files: []input.File{{Source: "test.yaml", Content: spec}}},
	})
	require.NoError(t, err)

	node := result[0][0].API.Components.Schemas["Node"]
	require.NotNil(t, node)

	children := node.Value.Properties["children"]
	require.NotNil(t, children)

	items := children.Value.Items
	require.NotNil(t, items)
	assert.Equal(t, "#/components/schemas/Node", items.Ref)
	assert.Same(t, node.Value, items.Value, "a repeated $ref must resolve to the same schema")
}

// OpenAPI 3.0 spells exclusive bounds as a boolean modifier and 3.1 as the bound
// itself; both must reach templates in the boolean form.
func TestParse_ExclusiveBounds(t *testing.T) {
	tests := map[string]struct {
		version    string
		constraint string
		wantMin    float64
	}{
		"3.0 boolean modifier": {
			version:    `"3.0.2"`,
			constraint: "minimum: 2\n          exclusiveMinimum: true",
			wantMin:    2,
		},
		"3.1 numeric bound": {
			version:    `"3.1.0"`,
			constraint: "exclusiveMinimum: 5",
			wantMin:    5,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec := []byte(`openapi: ` + test.version + `
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Bounded:
      type: object
      properties:
        value:
          type: integer
          ` + test.constraint + `
`)

			result, err := Parse(context.Background(), zerolog.Nop(), []input.FileGroup{
				{Files: []input.File{{Source: "test.yaml", Content: spec}}},
			})
			require.NoError(t, err)

			value := result[0][0].API.Components.Schemas["Bounded"].Value.Properties["value"]
			require.NotNil(t, value.Value.Min)
			assert.InDelta(t, test.wantMin, *value.Value.Min, 0)
			assert.True(t, value.Value.ExclusiveMin)
		})
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	inGroups := []input.FileGroup{
		{
			Files: []input.File{
				{Source: "bad.yaml", Content: []byte("not: valid: openapi: {{{}}")},
			},
		},
	}

	_, err := Parse(context.Background(), zerolog.Nop(), inGroups)
	assert.Error(t, err)
}

func TestParse_InvalidOpenAPIContent(t *testing.T) {
	// Completely broken content that the loader can't parse
	inGroups := []input.FileGroup{
		{
			Files: []input.File{
				{Source: "bad.yaml", Content: []byte(`[[[invalid`)},
			},
		},
	}

	_, err := Parse(context.Background(), zerolog.Nop(), inGroups)
	assert.Error(t, err)
}
