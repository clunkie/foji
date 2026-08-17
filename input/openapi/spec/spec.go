// Package spec holds the OpenAPI object model that foji templates bind to.
//
// Templates reach into these types directly (`.Ref`, `.Value`, `.Type.Is`,
// `.Paths.Map`, `.Operations`, ...), so the shape of this package is a public
// contract: changing a field name or type here changes every generated service.
// The model is deliberately decoupled from the parser so the underlying OpenAPI
// library can be swapped without touching templates. It is populated from
// libopenapi in convert.go.
package spec

import (
	"cmp"
	"net/http"
	"slices"
	"strings"
)

// T is a parsed OpenAPI document.
type T struct {
	Extensions map[string]any

	OpenAPI    string
	Info       *Info
	Paths      *Paths
	Components *Components
	Security   SecurityRequirements
}

// Info is the document's info block.
type Info struct {
	Title       string
	Description string
	Version     string
}

// Types is the set of JSON Schema types declared by a schema. OpenAPI 3.0 allows
// exactly one; 3.1 allows several, which is why this is a slice.
type Types []string

// Is reports whether the schema declares exactly the given type.
func (types Types) Is(typ string) bool {
	return len(types) == 1 && types[0] == typ
}

// Includes reports whether the given type is among those declared. An absent
// type declaration permits everything.
func (types Types) Includes(typ string) bool {
	if len(types) == 0 {
		return true
	}

	return slices.Contains(types, typ)
}

// Permits reports whether the given type is allowed by this declaration.
func (types Types) Permits(typ string) bool {
	if len(types) == 0 {
		return true
	}

	return types.Includes(typ)
}

// Slice returns the declared types.
func (types Types) Slice() []string {
	return types
}

// SchemaRef is a schema together with the `$ref` it was reached by, if any.
// Ref is empty for inline schemas; Value is always the resolved schema.
type SchemaRef struct {
	Ref   string
	Value *Schema
}

type (
	SchemaRefs []*SchemaRef
	Schemas    map[string]*SchemaRef
)

// Schema is a JSON Schema as used by OpenAPI.
type Schema struct {
	Extensions map[string]any

	OneOf SchemaRefs
	AnyOf SchemaRefs
	AllOf SchemaRefs
	Not   *SchemaRef

	Type        Types
	Title       string
	Format      string
	Description string
	Enum        []any
	Default     any
	Example     any

	UniqueItems bool

	// ExclusiveMin/Max are normalized to the 3.0 boolean form: a 3.1 numeric
	// `exclusiveMinimum` is folded into Min plus this flag.
	ExclusiveMin bool
	ExclusiveMax bool

	Nullable   bool
	ReadOnly   bool
	WriteOnly  bool
	Deprecated bool

	// Number
	Min        *float64
	Max        *float64
	MultipleOf *float64

	// String
	MinLength uint64
	MaxLength *uint64
	Pattern   string

	// Array
	MinItems uint64
	MaxItems *uint64
	Items    *SchemaRef

	// Object
	Required   []string
	Properties Schemas
	MinProps   uint64
	MaxProps   *uint64
}

// ParameterRef is a parameter together with the `$ref` it was reached by, if any.
type ParameterRef struct {
	Ref   string
	Value *Parameter
}

type (
	Parameters    []*ParameterRef
	ParametersMap map[string]*ParameterRef
)

// Parameter is an operation or path parameter.
type Parameter struct {
	Extensions map[string]any

	Name        string
	In          string
	Description string
	Required    bool
	Deprecated  bool
	Style       string
	Explode     *bool
	Schema      *SchemaRef
	Content     Content
}

// MediaType is one entry of a content map.
type MediaType struct {
	Extensions map[string]any

	Schema *SchemaRef
}

// Content maps mime types to their media type definitions.
type Content map[string]*MediaType

// Get resolves a mime type against the content map, falling back from the full
// mime type to its `x/y` form, then `x/*`, then `*/*`.
func (content Content) Get(mime string) *MediaType {
	if mime == "" {
		return content["*/*"]
	}

	if v := content[mime]; v != nil {
		return v
	}

	// Strip any parameters (`; charset=utf-8`) and retry.
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i]

		if v := content[mime]; v != nil {
			return v
		}
	}

	prefix, _, ok := strings.Cut(mime, "/")
	if !ok {
		// Not a valid mime type: don't let it fall through to the wildcard.
		return nil
	}

	if v := content[prefix+"/*"]; v != nil {
		return v
	}

	return content["*/*"]
}

// RequestBodyRef is a request body together with the `$ref` it was reached by.
type RequestBodyRef struct {
	Ref   string
	Value *RequestBody
}

type RequestBodies map[string]*RequestBodyRef

// RequestBody is an operation's request body.
type RequestBody struct {
	Extensions map[string]any

	Description string
	Required    bool
	Content     Content
}

// ResponseRef is a response together with the `$ref` it was reached by.
type ResponseRef struct {
	Ref   string
	Value *Response
}

type ResponseBodies map[string]*ResponseRef

// Response is a single operation response.
type Response struct {
	Extensions map[string]any

	Description string
	Headers     Headers
	Content     Content
}

// HeaderRef is a header together with the `$ref` it was reached by.
type HeaderRef struct {
	Ref   string
	Value *Header
}

type Headers map[string]*HeaderRef

// Header is a response header.
type Header struct {
	Extensions map[string]any

	Description string
	Required    bool
	Deprecated  bool
	Schema      *SchemaRef
}

// Responses holds an operation's responses keyed by status code, plus `default`.
type Responses struct {
	Extensions map[string]any

	m map[string]*ResponseRef
}

// NewResponses returns an empty Responses.
func NewResponses() *Responses {
	return &Responses{m: map[string]*ResponseRef{}}
}

// Set registers a response under a status code (or `default`).
func (responses *Responses) Set(key string, value *ResponseRef) {
	if responses.m == nil {
		responses.m = map[string]*ResponseRef{}
	}

	responses.m[key] = value
}

// Map returns the responses keyed by status code (and `default`).
func (responses *Responses) Map() map[string]*ResponseRef {
	if responses == nil || responses.m == nil {
		return map[string]*ResponseRef{}
	}

	return responses.m
}

// Len returns the number of responses.
func (responses *Responses) Len() int {
	if responses == nil {
		return 0
	}

	return len(responses.m)
}

// Value returns the response for a status code, or nil.
func (responses *Responses) Value(key string) *ResponseRef {
	if responses == nil {
		return nil
	}

	return responses.m[key]
}

// Default returns the `default` response, or nil.
func (responses *Responses) Default() *ResponseRef {
	return responses.Value("default")
}

// Operation is a single path + verb operation.
type Operation struct {
	Extensions map[string]any

	Tags        []string
	Summary     string
	Description string
	OperationID string
	Parameters  Parameters
	RequestBody *RequestBodyRef
	Responses   *Responses
	Deprecated  bool

	// Security is nil when the operation does not override document security,
	// and non-nil (possibly empty) when it does.
	Security *SecurityRequirements
}

// PathItem is the set of operations available on one path.
type PathItem struct {
	Extensions map[string]any

	Ref         string
	Summary     string
	Description string

	Connect *Operation
	Delete  *Operation
	Get     *Operation
	Head    *Operation
	Options *Operation
	Patch   *Operation
	Post    *Operation
	Put     *Operation
	Trace   *Operation

	Parameters Parameters
}

// Operations returns the defined operations keyed by uppercase HTTP method.
func (pathItem *PathItem) Operations() map[string]*Operation {
	operations := make(map[string]*Operation)

	for verb, op := range map[string]*Operation{
		http.MethodConnect: pathItem.Connect,
		http.MethodDelete:  pathItem.Delete,
		http.MethodGet:     pathItem.Get,
		http.MethodHead:    pathItem.Head,
		http.MethodOptions: pathItem.Options,
		http.MethodPatch:   pathItem.Patch,
		http.MethodPost:    pathItem.Post,
		http.MethodPut:     pathItem.Put,
		http.MethodTrace:   pathItem.Trace,
	} {
		if op != nil {
			operations[verb] = op
		}
	}

	return operations
}

// GetOperation returns the operation for an uppercase HTTP method, or nil.
func (pathItem *PathItem) GetOperation(method string) *Operation {
	return pathItem.Operations()[method]
}

// Paths holds the document's path items keyed by path template.
type Paths struct {
	Extensions map[string]any

	m map[string]*PathItem
}

// NewPaths returns an empty Paths.
func NewPaths() *Paths {
	return &Paths{m: map[string]*PathItem{}}
}

// Set registers a path item under a path template.
func (paths *Paths) Set(key string, value *PathItem) {
	if paths.m == nil {
		paths.m = map[string]*PathItem{}
	}

	paths.m[key] = value
}

// Map returns the path items keyed by path template.
func (paths *Paths) Map() map[string]*PathItem {
	if paths == nil || paths.m == nil {
		return map[string]*PathItem{}
	}

	return paths.m
}

// Len returns the number of paths.
func (paths *Paths) Len() int {
	if paths == nil {
		return 0
	}

	return len(paths.m)
}

// Value returns the path item registered under the exact path template, or nil.
func (paths *Paths) Value(key string) *PathItem {
	if paths == nil {
		return nil
	}

	return paths.m[key]
}

// Find returns the path item for a path template, treating templated segments as
// equivalent regardless of the variable names used ("/a/{id}" matches "/a/{x}").
func (paths *Paths) Find(key string) *PathItem {
	if pathItem := paths.Value(key); pathItem != nil {
		return pathItem
	}

	normalized, count := normalizeTemplatedPath(key)

	for path, pathItem := range paths.Map() {
		candidate, candidateCount := normalizeTemplatedPath(path)
		if candidateCount == count && candidate == normalized {
			return pathItem
		}
	}

	return nil
}

// InMatchingOrder returns the path templates ordered most-specific first, by
// ascending count of template variables then descending lexicographic order.
func (paths *Paths) InMatchingOrder() []string {
	if paths.Len() == 0 {
		return nil
	}

	byVarCount := make(map[int][]string)
	maxVars := 0

	for path := range paths.Map() {
		count := strings.Count(path, "}")
		byVarCount[count] = append(byVarCount[count], path)

		if count > maxVars {
			maxVars = count
		}
	}

	ordered := make([]string, 0, paths.Len())

	for c := 0; c <= maxVars; c++ {
		ps, ok := byVarCount[c]
		if !ok {
			continue
		}

		slices.SortFunc(ps, func(a, b string) int { return cmp.Compare(b, a) })
		ordered = append(ordered, ps...)
	}

	return ordered
}

// normalizeTemplatedPath replaces every `{var}` segment with `{}` so that two
// path templates that differ only in variable naming compare equal. It also
// reports how many variables were found.
func normalizeTemplatedPath(path string) (string, int) {
	if !strings.ContainsRune(path, '{') {
		return path, 0
	}

	var (
		buf   strings.Builder
		count int
		depth int
	)

	for _, r := range path {
		switch {
		case r == '{':
			if depth == 0 {
				buf.WriteString("{}")

				count++
			}

			depth++
		case r == '}' && depth > 0:
			depth--
		case depth == 0:
			buf.WriteRune(r)
		}
	}

	return buf.String(), count
}

// Components holds the document's reusable objects.
type Components struct {
	Extensions map[string]any

	Schemas         Schemas
	Parameters      ParametersMap
	Headers         Headers
	RequestBodies   RequestBodies
	Responses       ResponseBodies
	SecuritySchemes SecuritySchemes
}

// SecuritySchemeRef is a security scheme together with the `$ref` it was reached by.
type SecuritySchemeRef struct {
	Ref   string
	Value *SecurityScheme
}

type SecuritySchemes map[string]*SecuritySchemeRef

// SecurityScheme describes one way of authenticating against the API.
type SecurityScheme struct {
	Extensions map[string]any

	Type         string
	Description  string
	Name         string
	In           string
	Scheme       string
	BearerFormat string
}

// SecurityRequirement maps a security scheme name to the scopes it must grant.
type SecurityRequirement map[string][]string

// SecurityRequirements is a disjunction: satisfying any one entry authorizes the
// request.
type SecurityRequirements []SecurityRequirement
