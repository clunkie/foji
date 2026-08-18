package output

import (
	"fmt"
	"strings"

	"github.com/codemodus/kace"

	"github.com/gofoji/foji/input/openapi/spec"
)

/* Type naming, type resolution, and extension helpers. */

func (o *OpenAPIFileContext) RefToName(ref string) string {
	modelPackage := o.PackageName()
	parts := strings.Split(ref, "/")

	return modelPackage + "." + o.ToCase(parts[len(parts)-1])
}

func (o *OpenAPIFileContext) GetTypeName(pkg string, s *spec.SchemaRef) string {
	ref := o.RefToName(s.Ref)

	if t, ok := o.Maps.Type[ref]; ok {
		return o.CheckPackage(t, pkg)
	}

	return o.CheckPackage(ref, pkg)
}

func (o *OpenAPIFileContext) TypeOnly(name string) string {
	tt := strings.Split(name, ".")

	return tt[len(tt)-1]
}

// getXGoType maps x-go-type declarations to an actual type definition.
// Supports formats:
//
//	x-go-type: full/path/to.type
//	x-go-type: int
func (o *OpenAPIFileContext) getXGoType(currentPackage string, goType any) string {
	if s, ok := goType.(string); ok {
		return o.CheckPackage(s, currentPackage)
	}

	return fmt.Sprintf("INVALID x-go-type: %v", goType)
}

// HasExtensionValue checks if an extension exists and has a truthy value.
// For boolean extensions, it returns the boolean value.
// For other extensions, it returns true if they exist.
func HasExtensionValue(extensions map[string]any, ext string) bool {
	v, ok := extensions[ext]
	if !ok {
		return false
	}

	if b, isBool := v.(bool); isBool {
		return b
	}

	return true
}

func (o *OpenAPIFileContext) OpHasExtension(op *spec.Operation, ext string) bool {
	return HasExtensionValue(op.Extensions, ext)
}

func (o *OpenAPIFileContext) SecurityHasExtension(scheme *spec.SecuritySchemeRef, ext string) bool {
	return HasExtensionValue(scheme.Value.Extensions, ext)
}

func (o *OpenAPIFileContext) HasExtension(s *spec.SchemaRef, ext string) bool {
	_, ok := s.Value.Extensions[ext]

	return ok
}

//nolint:cyclop
func (o *OpenAPIFileContext) GetType(currentPackage, name string, s *spec.SchemaRef) string {
	if s == nil {
		return ""
	}

	if override, ok := s.Value.Extensions["x-go-type"]; ok {
		return o.getXGoType(currentPackage, override)
	}

	if t, ok := o.Maps.Type[name]; ok {
		return o.CheckPackage(t, currentPackage)
	}

	schemaType := ""
	if len(s.Value.Type) == 1 {
		schemaType = s.Value.Type[0]
	}

	if s.Value.Format != "" {
		if t, ok := o.Maps.Type[schemaType+","+s.Value.Format]; ok {
			return o.CheckPackage(t, currentPackage)
		}
	}

	if s.Ref != "" {
		return o.GetTypeName(currentPackage, s)
	}

	if s.Value.Type.Is("array") {
		return "[]" + o.GetType(currentPackage, name, s.Value.Items)
	}

	if s.Value.Type.Is("string") && s.Value.Format == "binary" {
		return "forms.File"
	}

	if s.Value.Type.Is("object") || s.Value.Type.Is("") || len(s.Value.Type) == 0 {
		if len(o.SchemaProperties(s)) == 0 {
			if t, ok := o.Maps.Type[schemaType]; ok {
				return o.CheckPackage(t, currentPackage)
			}

			return "any"
		}

		name = o.PackageName() + "." + kace.Pascal(name)

		return o.CheckPackage(name, currentPackage)
	}

	if o.IsDefaultEnum(name, s) {
		return o.CheckPackage(o.EnumName(name), currentPackage)
	}

	if t, ok := o.Maps.Type[schemaType]; ok {
		return o.CheckPackage(t, currentPackage)
	}

	return fmt.Sprintf("unknown type: name(%s): type(%s)", name, s.Value.Type)
}

func (o *OpenAPIFileContext) EnumName(name string) string {
	// TODO: Support override via template
	return o.PackageName() + "." + kace.Pascal(name)
}

func (o *OpenAPIFileContext) EnumNew(name string) string {
	name = strings.TrimPrefix(name, "[]")

	pos := strings.Index(name, ".") + 1

	return name[:pos] + "New" + name[pos:]
}

func (o *OpenAPIFileContext) StripArray(name string) string {
	if strings.HasPrefix(name, "[]") {
		return name[2:]
	}

	return name
}
