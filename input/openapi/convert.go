package openapi

import (
	"maps"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"

	"github.com/gofoji/foji/input/openapi/spec"
)

// converter translates libopenapi's high-level document into the template-facing
// model in package spec.
//
// Schemas are cached by `$ref` so a recursive document (A -> B -> A) terminates,
// and so every reference to a component resolves to the same *spec.Schema, which
// is what templates assume when they compare or key off schema identity.
type converter struct {
	schemas map[string]*spec.Schema
}

// componentSchemaPrefix is the ref every `components.schemas` entry is addressed by.
const componentSchemaPrefix = "#/components/schemas/"

func newConverter() *converter {
	return &converter{schemas: map[string]*spec.Schema{}}
}

func (c *converter) document(doc *v3.Document) *spec.T {
	if doc == nil {
		return nil
	}

	out := &spec.T{
		Extensions: extensions(doc.Extensions),
		OpenAPI:    doc.Version,
		Paths:      c.paths(doc.Paths),
		Components: c.components(doc.Components),
		Security:   securityRequirements(doc.Security),
	}

	if doc.Info != nil {
		out.Info = &spec.Info{
			Title:       doc.Info.Title,
			Description: doc.Info.Description,
			Version:     doc.Info.Version,
		}
	}

	return out
}

/* Paths and operations */

func (c *converter) paths(in *v3.Paths) *spec.Paths {
	out := spec.NewPaths()
	if in == nil {
		return out
	}

	for name, item := range in.PathItems.FromOldest() {
		out.Set(name, c.pathItem(item))
	}

	out.Extensions = extensions(in.Extensions)

	return out
}

func (c *converter) pathItem(in *v3.PathItem) *spec.PathItem {
	if in == nil {
		return nil
	}

	return &spec.PathItem{
		Extensions:  extensions(in.Extensions),
		Ref:         refOf(in),
		Summary:     in.Summary,
		Description: in.Description,
		Get:         c.operation(in.Get),
		Put:         c.operation(in.Put),
		Post:        c.operation(in.Post),
		Delete:      c.operation(in.Delete),
		Options:     c.operation(in.Options),
		Head:        c.operation(in.Head),
		Patch:       c.operation(in.Patch),
		Trace:       c.operation(in.Trace),
		Parameters:  c.parameters(in.Parameters),
	}
}

func (c *converter) operation(in *v3.Operation) *spec.Operation {
	if in == nil {
		return nil
	}

	out := &spec.Operation{
		Extensions:  extensions(in.Extensions),
		Tags:        in.Tags,
		Summary:     in.Summary,
		Description: in.Description,
		OperationID: in.OperationId,
		Deprecated:  boolValue(in.Deprecated),
		Parameters:  c.parameters(in.Parameters),
		RequestBody: c.requestBody(in.RequestBody),
		Responses:   c.responses(in.Responses),
	}

	// A nil Security means "inherit the document's"; an empty-but-present one
	// means "this operation is explicitly unauthenticated". Only the low-level
	// model records whether the key was written at all.
	if declaresSecurity(in) {
		s := securityRequirements(in.Security)
		out.Security = &s
	}

	return out
}

func declaresSecurity(op *v3.Operation) bool {
	if len(op.Security) > 0 {
		return true
	}

	low := op.GoLow()

	return low != nil && !low.Security.IsEmpty()
}

/* Parameters, bodies and responses */

func (c *converter) parameters(in []*v3.Parameter) spec.Parameters {
	if len(in) == 0 {
		return nil
	}

	out := make(spec.Parameters, 0, len(in))
	for _, p := range in {
		out = append(out, c.parameter(p))
	}

	return out
}

func (c *converter) parameter(in *v3.Parameter) *spec.ParameterRef {
	if in == nil {
		return nil
	}

	return &spec.ParameterRef{
		Ref: refOf(in),
		Value: &spec.Parameter{
			Extensions:  extensions(in.Extensions),
			Name:        in.Name,
			In:          in.In,
			Description: in.Description,
			Required:    boolValue(in.Required),
			Deprecated:  in.Deprecated,
			Style:       in.Style,
			Explode:     in.Explode,
			Schema:      c.schemaRef(in.Schema),
			Content:     c.content(in.Content),
		},
	}
}

func (c *converter) requestBody(in *v3.RequestBody) *spec.RequestBodyRef {
	if in == nil {
		return nil
	}

	return &spec.RequestBodyRef{
		Ref: refOf(in),
		Value: &spec.RequestBody{
			Extensions:  extensions(in.Extensions),
			Description: in.Description,
			Required:    boolValue(in.Required),
			Content:     c.content(in.Content),
		},
	}
}

func (c *converter) responses(in *v3.Responses) *spec.Responses {
	out := spec.NewResponses()
	if in == nil {
		return out
	}

	for code, r := range in.Codes.FromOldest() {
		out.Set(code, c.response(r))
	}

	if in.Default != nil {
		out.Set("default", c.response(in.Default))
	}

	out.Extensions = extensions(in.Extensions)

	return out
}

func (c *converter) response(in *v3.Response) *spec.ResponseRef {
	if in == nil {
		return nil
	}

	return &spec.ResponseRef{
		Ref: refOf(in),
		Value: &spec.Response{
			Extensions:  extensions(in.Extensions),
			Description: in.Description,
			Headers:     c.headers(in.Headers),
			Content:     c.content(in.Content),
		},
	}
}

func (c *converter) content(in *orderedmap.Map[string, *v3.MediaType]) spec.Content {
	if orderedmap.Len(in) == 0 {
		return nil
	}

	out := make(spec.Content, orderedmap.Len(in))
	for mime, mt := range in.FromOldest() {
		out[mime] = &spec.MediaType{
			Extensions: extensions(mt.Extensions),
			Schema:     c.schemaRef(mt.Schema),
		}
	}

	return out
}

func (c *converter) headers(in *orderedmap.Map[string, *v3.Header]) spec.Headers {
	if orderedmap.Len(in) == 0 {
		return nil
	}

	out := make(spec.Headers, orderedmap.Len(in))
	for name, h := range in.FromOldest() {
		out[name] = &spec.HeaderRef{
			Ref: refOf(h),
			Value: &spec.Header{
				Extensions:  extensions(h.Extensions),
				Description: h.Description,
				Required:    h.Required,
				Deprecated:  h.Deprecated,
				Schema:      c.schemaRef(h.Schema),
			},
		}
	}

	return out
}

/* Components and security */

func (c *converter) components(in *v3.Components) *spec.Components {
	if in == nil {
		return nil
	}

	out := &spec.Components{Extensions: extensions(in.Extensions)}

	if orderedmap.Len(in.Schemas) > 0 {
		out.Schemas = make(spec.Schemas, orderedmap.Len(in.Schemas))
		for name, s := range in.Schemas.FromOldest() {
			out.Schemas[name] = c.componentSchema(name, s)
		}
	}

	if orderedmap.Len(in.Parameters) > 0 {
		out.Parameters = make(spec.ParametersMap, orderedmap.Len(in.Parameters))
		for name, p := range in.Parameters.FromOldest() {
			out.Parameters[name] = c.parameter(p)
		}
	}

	if orderedmap.Len(in.RequestBodies) > 0 {
		out.RequestBodies = make(spec.RequestBodies, orderedmap.Len(in.RequestBodies))
		for name, b := range in.RequestBodies.FromOldest() {
			out.RequestBodies[name] = c.requestBody(b)
		}
	}

	if orderedmap.Len(in.Responses) > 0 {
		out.Responses = make(spec.ResponseBodies, orderedmap.Len(in.Responses))
		for name, r := range in.Responses.FromOldest() {
			out.Responses[name] = c.response(r)
		}
	}

	if orderedmap.Len(in.Headers) > 0 {
		out.Headers = c.headers(in.Headers)
	}

	if orderedmap.Len(in.SecuritySchemes) > 0 {
		out.SecuritySchemes = make(spec.SecuritySchemes, orderedmap.Len(in.SecuritySchemes))
		for name, s := range in.SecuritySchemes.FromOldest() {
			out.SecuritySchemes[name] = &spec.SecuritySchemeRef{
				Ref: refOf(s),
				Value: &spec.SecurityScheme{
					Extensions:   extensions(s.Extensions),
					Type:         s.Type,
					Description:  s.Description,
					Name:         s.Name,
					In:           s.In,
					Scheme:       s.Scheme,
					BearerFormat: s.BearerFormat,
				},
			}
		}
	}

	return out
}

func securityRequirements(in []*highbase.SecurityRequirement) spec.SecurityRequirements {
	if len(in) == 0 {
		return nil
	}

	out := make(spec.SecurityRequirements, 0, len(in))

	for _, req := range in {
		// An empty requirement (`- {}`) marks the operation as optionally
		// authenticated and must survive as an empty entry, not be dropped.
		group := make(spec.SecurityRequirement, orderedmap.Len(req.Requirements))
		maps.Insert(group, req.Requirements.FromOldest())

		out = append(out, group)
	}

	return out
}

/* Schemas */

func (c *converter) schemaRefs(in []*highbase.SchemaProxy) spec.SchemaRefs {
	if len(in) == 0 {
		return nil
	}

	out := make(spec.SchemaRefs, 0, len(in))
	for _, s := range in {
		out = append(out, c.schemaRef(s))
	}

	return out
}

// componentSchema converts a `components.schemas` entry, registering it under the
// ref that points at it so that the definition and every `$ref` to it share one
// *spec.Schema. Like the previous loader, the definition itself carries no Ref —
// that is how templates tell a declaration apart from a use of one.
func (c *converter) componentSchema(name string, in *highbase.SchemaProxy) *spec.SchemaRef {
	if in == nil || in.IsReference() {
		// The component is an alias for another schema; treat it as any other ref.
		return c.schemaRef(in)
	}

	canonical := componentSchemaPrefix + name

	if cached, ok := c.schemas[canonical]; ok {
		return &spec.SchemaRef{Value: cached}
	}

	out := &spec.Schema{}
	c.schemas[canonical] = out

	if resolved := in.Schema(); resolved != nil {
		c.fillSchema(out, resolved)
	}

	return &spec.SchemaRef{Value: out}
}

func (c *converter) schemaRef(in *highbase.SchemaProxy) *spec.SchemaRef {
	if in == nil {
		return nil
	}

	var ref string
	if in.IsReference() {
		ref = in.GetReference()
	}

	if ref != "" {
		if cached, ok := c.schemas[ref]; ok {
			return &spec.SchemaRef{Ref: ref, Value: cached}
		}
	}

	out := &spec.Schema{}

	// Register before recursing so a schema that references itself terminates.
	if ref != "" {
		c.schemas[ref] = out
	}

	if resolved := in.Schema(); resolved != nil {
		c.fillSchema(out, resolved)
	}

	return &spec.SchemaRef{Ref: ref, Value: out}
}

// fillSchema is a flat field-by-field translation; splitting it would only hide
// the mapping it exists to document.
func (c *converter) fillSchema(out *spec.Schema, in *highbase.Schema) {
	out.Extensions = extensions(in.Extensions)
	out.Title = in.Title
	out.Format = in.Format
	out.Description = in.Description
	out.Pattern = in.Pattern
	out.Required = in.Required

	out.Type = spec.Types(in.Type)

	out.Enum = nodeValues(in.Enum)
	out.Default = nodeValue(in.Default)
	out.Example = nodeValue(in.Example)

	out.UniqueItems = boolValue(in.UniqueItems)
	out.Nullable = boolValue(in.Nullable)
	out.ReadOnly = boolValue(in.ReadOnly)
	out.WriteOnly = boolValue(in.WriteOnly)
	out.Deprecated = boolValue(in.Deprecated)

	out.Min = in.Minimum
	out.Max = in.Maximum
	out.MultipleOf = in.MultipleOf
	out.ExclusiveMin, out.Min = exclusiveBound(in.ExclusiveMinimum, out.Min)
	out.ExclusiveMax, out.Max = exclusiveBound(in.ExclusiveMaximum, out.Max)

	out.MinLength = uintValue(in.MinLength)
	out.MaxLength = uintPtr(in.MaxLength)
	out.MinItems = uintValue(in.MinItems)
	out.MaxItems = uintPtr(in.MaxItems)
	out.MinProps = uintValue(in.MinProperties)
	out.MaxProps = uintPtr(in.MaxProperties)

	out.AllOf = c.schemaRefs(in.AllOf)
	out.AnyOf = c.schemaRefs(in.AnyOf)
	out.OneOf = c.schemaRefs(in.OneOf)
	out.Not = c.schemaRef(in.Not)

	// In 3.1 `items` may be a boolean; only the schema form is meaningful here.
	if in.Items != nil && in.Items.N == 0 {
		out.Items = c.schemaRef(in.Items.A)
	}

	if orderedmap.Len(in.Properties) > 0 {
		out.Properties = make(spec.Schemas, orderedmap.Len(in.Properties))
		for name, p := range in.Properties.FromOldest() {
			out.Properties[name] = c.schemaRef(p)
		}
	}
}

// exclusiveBound normalizes both spellings of exclusiveMinimum/exclusiveMaximum
// onto the 3.0 shape templates expect: a boolean flag plus a bound. In 3.0 the
// keyword is a boolean modifier on an existing bound; in 3.1 it is the bound
// itself, so it replaces one.
func exclusiveBound(in *highbase.DynamicValue[bool, float64], bound *float64) (bool, *float64) {
	if in == nil {
		return false, bound
	}

	if in.N == 0 {
		return in.A, bound
	}

	value := in.B

	return true, &value
}

/* libopenapi plumbing */

type (
	lowReferencer interface {
		IsReference() bool
		GetReference() string
	}
	lowModel interface{ GoLowUntyped() any }
)

// refOf recovers the `$ref` that a high-level object was reached by. libopenapi
// resolves references while building the high model and records the original ref
// only on the low model, but templates need it to name the types they generate.
func refOf(in lowModel) string {
	low, ok := in.GoLowUntyped().(lowReferencer)
	if !ok || !low.IsReference() {
		return ""
	}

	return low.GetReference()
}

func extensions(in *orderedmap.Map[string, *yaml.Node]) map[string]any {
	if orderedmap.Len(in) == 0 {
		return nil
	}

	out := make(map[string]any, orderedmap.Len(in))
	for name, node := range in.FromOldest() {
		out[name] = nodeValue(node)
	}

	return out
}

// nodeValue decodes a raw YAML node into its natural Go value so that templates
// can print and compare defaults, enum members and extensions directly.
func nodeValue(in *yaml.Node) any {
	if in == nil {
		return nil
	}

	var out any

	err := in.Decode(&out)
	if err != nil {
		return nil
	}

	return out
}

func nodeValues(in []*yaml.Node) []any {
	if len(in) == 0 {
		return nil
	}

	out := make([]any, 0, len(in))
	for _, node := range in {
		out = append(out, nodeValue(node))
	}

	return out
}

func boolValue(in *bool) bool {
	return in != nil && *in
}

func uintValue(in *int64) uint64 {
	if in == nil || *in < 0 {
		return 0
	}

	return uint64(*in)
}

func uintPtr(in *int64) *uint64 {
	if in == nil || *in < 0 {
		return nil
	}

	out := uint64(*in)

	return &out
}
