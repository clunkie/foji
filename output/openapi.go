// Package output renders parsed inputs through the configured templates.
//
// The OpenAPI template context is large enough that it is split by concern:
// this file holds the generator entry point and the context plumbing, while
// the helpers exposed to templates live in openapi_type.go (type and name
// resolution), openapi_schema.go (schema and parameter inspection),
// openapi_response.go (request bodies and response selection), and
// openapi_auth.go (security). Every helper is part of the template API, so
// names here are load bearing.
package output

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/gofoji/foji/cfg"
	"github.com/gofoji/foji/input/openapi"
	"github.com/gofoji/foji/input/openapi/spec"
)

const OpenAPIFile = "OpenAPIFile"

func HasOpenAPIOutput(o cfg.Output) bool {
	return hasAnyOutput(o, OpenAPIFile)
}

func OpenAPI(p cfg.Process, fn cfg.FileHandler, l zerolog.Logger, groups openapi.FileGroups, simulate bool) error {
	g := newGen(p, fn, l, simulate)

	for _, ff := range groups {
		if g.err != nil {
			break
		}

		for _, f := range ff {
			ctx := OpenAPIFileContext{
				Context: NewContext(p, l),
				File:    f,
			}

			g.render(OpenAPIFile, &ctx)
		}
	}

	return g.err
}

type OpenAPIFileContext struct {
	Context
	Imports
	openapi.File
}

// wrapErr annotates a render failure with the runtime params in play, which
// identify the sub-template that was executing.
func (o *OpenAPIFileContext) wrapErr(err error) error {
	if len(o.RuntimeParams) > 0 {
		return fmt.Errorf("%w:%v", err, o.RuntimeParams)
	}

	return err
}

func (o *OpenAPIFileContext) Init() error {
	o.AbortError = nil
	o.Imports = nil

	return nil
}

func (o *OpenAPIFileContext) GoImports() []string {
	var out []string

	for _, i := range o.Imports {
		if i == o.PackageName() {
			continue
		}

		out = append(out, i)
	}

	return out
}

func (o *OpenAPIFileContext) WithParams(values ...any) (*OpenAPIFileContext, error) {
	ctx, err := o.Context.WithParams(values...)
	if err != nil {
		return nil, err
	}

	out := *o
	out.Context = *ctx

	return &out, nil
}

func (o *OpenAPIFileContext) ComponentSchemas() spec.Schemas {
	if o.API.Components == nil {
		return nil
	}

	return o.API.Components.Schemas
}

func (o *OpenAPIFileContext) ComponentParameters() spec.ParametersMap {
	if o.API.Components == nil {
		return nil
	}

	return o.API.Components.Parameters
}

// CheckAllTypes is a helper to iterate all property references for import requirements.
// This is expected to inject imports for unnecessary packages depending on the template
// generated, the post-processing should remove unused imports.
func (o *OpenAPIFileContext) CheckAllTypes(pkg string, types ...string) string {
	for _, s := range o.ComponentSchemas() {
		for key, schema := range s.Value.Properties {
			o.GetType(pkg, key, schema)
		}

		for _, nested := range s.Value.AllOf {
			for key, schema := range nested.Value.Properties {
				o.GetType(pkg, key, schema)
			}
		}
	}

	for _, s := range types {
		o.CheckPackage(s, pkg)
	}

	return ""
}
