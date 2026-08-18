package output

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/gofoji/foji/input/openapi/spec"
)

/* Schema and parameter inspection helpers. */

func hasValidation(s *spec.Schema) bool {
	return s.Min != nil || s.Max != nil || s.MultipleOf != nil || // Number
		s.MinLength > 0 || s.MaxLength != nil || len(s.Pattern) > 0 || // String
		s.MinItems > 0 || s.MaxItems != nil // Array
}

func (o *OpenAPIFileContext) HasValidation(s *spec.SchemaRef) bool {
	if hasValidation(s.Value) {
		return true
	}

	for _, p := range s.Value.Properties {
		if o.HasValidation(p) {
			return true
		}
	}

	return slices.ContainsFunc(s.Value.AllOf, o.HasValidation)
}

// IsDefaultEnum helper that checks if an enumerated type is overridden (specified externally).
func (o *OpenAPIFileContext) IsDefaultEnum(name string, s *spec.SchemaRef) bool {
	if len(s.Value.Enum) == 0 {
		return false
	}

	_, ok := o.Maps.Type[name]

	return !ok
}

func (o *OpenAPIFileContext) HasRequiredProperties(s *spec.SchemaRef) bool {
	return len(o.RequiredProperties(s)) > 0
}

// IsRequiredProperty helper that checks if a property is required.
func (o *OpenAPIFileContext) IsRequiredProperty(name string, s *spec.SchemaRef) bool {
	// property is required if it is listed in the schema's required properties
	if slices.Contains(s.Value.Required, name) {
		return true
	}

	// property is required if any of the allOf schemas require it
	for _, subSchema := range s.Value.AllOf {
		if slices.Contains(subSchema.Value.Required, name) {
			return true
		}
	}

	// property is required if there is at least one anyOf schema and they all require the field
	anyOfWithoutProp := false

	for _, subSchema := range s.Value.AnyOf {
		if !slices.Contains(subSchema.Value.Required, name) {
			anyOfWithoutProp = true
		}
	}

	if !anyOfWithoutProp && len(s.Value.AnyOf) > 0 {
		return true
	}

	// property is required if there is at least one oneOf schema and they all require the field
	oneOfWithoutProp := false

	for _, subSchema := range s.Value.OneOf {
		if !slices.Contains(subSchema.Value.Required, name) {
			oneOfWithoutProp = true
		}
	}

	if !oneOfWithoutProp && len(s.Value.OneOf) > 0 {
		return true
	}

	return false
}

func (o *OpenAPIFileContext) RequiredProperties(schema *spec.SchemaRef) spec.Schemas {
	out := spec.Schemas{}

	for name, ref := range o.SchemaProperties(schema) {
		if o.IsRequiredProperty(name, schema) {
			out[name] = ref
		}
	}

	return out
}

func (o *OpenAPIFileContext) SchemaPropertiesHaveDefaults(schema *spec.SchemaRef) bool {
	for _, v := range o.SchemaProperties(schema) {
		if v.Value.Default != nil {
			return true
		}
	}

	return false
}

func (o *OpenAPIFileContext) SchemaProperties(schema *spec.SchemaRef) spec.Schemas {
	out := spec.Schemas{}

	return schemaPropertiesWithEmbedded(schema, out)
}

func schemaPropertiesWithEmbedded(schema *spec.SchemaRef, out spec.Schemas) spec.Schemas {
	maps.Copy(out, schema.Value.Properties)

	for _, subSchema := range schema.Value.AllOf {
		schemaPropertiesWithEmbedded(subSchema, out)
	}

	return out
}

func (o *OpenAPIFileContext) SchemaEnums(schema *spec.SchemaRef) spec.Schemas {
	out := spec.Schemas{}

	for k, v := range o.SchemaProperties(schema) {
		if len(v.Value.Enum) > 0 {
			out[k] = v
		}
	}

	return out
}

func (o *OpenAPIFileContext) SchemaIsEnum(schema *spec.SchemaRef) bool {
	return len(schema.Value.Enum) > 0
}

func (o *OpenAPIFileContext) SchemaIsEnumArray(schema *spec.SchemaRef) bool {
	return schema.Value.Items != nil && len(schema.Value.Items.Value.Enum) > 0
}

func (o *OpenAPIFileContext) SchemaContainsAllOf(schema *spec.SchemaRef) bool {
	return schema != nil && len(schema.Value.AllOf) > 0
}

func (o *OpenAPIFileContext) SchemaIsComplex(schema *spec.SchemaRef) bool {
	if schema == nil || schema.Ref != "" {
		return false
	}

	if schema.Value.Type.Is("object") {
		return true
	}

	if len(schema.Value.AllOf) > 0 {
		return true
	}

	if !schema.Value.Type.Is("array") {
		return false
	}

	return schema.Value.Items.Ref == "" && schema.Value.Items.Value.Type.Is("object")
}

func (o *OpenAPIFileContext) SchemaIsObject(schema *spec.SchemaRef) bool {
	return schema.Value.Type.Is("object") || schema.Value.Type.Is("string") // to catch timestamps and uuids
}

func (o *OpenAPIFileContext) OpParams(path *spec.PathItem, op *spec.Operation) spec.Parameters {
	out := make(spec.Parameters, 0, len(path.Parameters)+len(op.Parameters))

	out = append(out, path.Parameters...)
	out = append(out, op.Parameters...)

	return out
}

func (o *OpenAPIFileContext) ParamIsOptionalType(param *spec.ParameterRef) bool {
	if param.Value.Required {
		return false
	}

	if param.Value.Schema.Value.Type.Is("array") {
		return false
	}

	t := o.GetType("", "", param.Value.Schema)
	if strings.HasPrefix(t, "map[") {
		return false
	}

	return param.Value.Schema.Value.Default == nil
}

func (o *OpenAPIFileContext) ParamIsEnum(param *spec.ParameterRef) bool {
	return o.SchemaIsEnum(param.Value.Schema)
}

func (o *OpenAPIFileContext) ParamIsEnumArray(param *spec.ParameterRef) bool {
	return o.SchemaIsEnumArray(param.Value.Schema)
}

func (o *OpenAPIFileContext) DefaultValues(val string) []string {
	if val == "" {
		return nil
	}

	if len(val) < 2 || val[0] != '[' || val[len(val)-1] != ']' {
		return []string{val}
	}

	csvReader := csv.NewReader(bytes.NewReader([]byte(val[1 : len(val)-1])))

	records, err := csvReader.ReadAll()
	if err != nil {
		o.AbortError = fmt.Errorf("error reading csv for default: %w: %q", err, val)
		return nil
	}

	if len(records) > 0 {
		return records[0]
	}

	return nil
}
