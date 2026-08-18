package output

import (
	"github.com/gofoji/foji/input/openapi/spec"
)

/* Auth focused helpers. */

func (o *OpenAPIFileContext) OpSecurity(op *spec.Operation) spec.SecurityRequirements {
	if op.Security != nil {
		return *op.Security
	}

	return o.API.Security
}

func hasAuthorization(security spec.SecurityRequirements) bool {
	for _, ss := range security {
		for _, scopes := range ss {
			if len(scopes) > 0 {
				return true
			}
		}
	}

	return false
}

func (o *OpenAPIFileContext) HasAuthentication() bool {
	return o.API.Components != nil && o.API.Components.SecuritySchemes != nil && len(o.API.Components.SecuritySchemes) > 0
}

func (o *OpenAPIFileContext) HasAuthorization() bool {
	if hasAuthorization(o.API.Security) {
		return true
	}

	for _, p := range o.API.Paths.InMatchingOrder() {
		path := o.API.Paths.Value(p)
		for _, op := range path.Operations() {
			if op.Security != nil && hasAuthorization(*op.Security) {
				return true
			}
		}
	}

	return false
}

func (o *OpenAPIFileContext) IsSimpleAuth(op *spec.Operation) bool {
	s := o.OpSecurity(op)
	if len(s) == 0 {
		return true
	}

	var authName *string

	isDifferentAuth := func(key string) bool {
		if authName == nil {
			authName = &key
			return false
		}

		return *authName != key
	}

	for _, group := range s {
		if len(group) == 0 {
			if isDifferentAuth("") {
				return false
			}
		}

		for key := range group {
			if isDifferentAuth(key) {
				return false
			}
		}
	}

	return true
}

func (o *OpenAPIFileContext) HasComplexAuth() bool {
	for _, p := range o.API.Paths.InMatchingOrder() {
		path := o.API.Paths.Value(p)
		for _, op := range path.Operations() {
			if !o.IsSimpleAuth(op) {
				return true
			}
		}
	}

	return false
}

// hasAuthScheme reports whether any declared security scheme uses the named
// HTTP auth scheme.
func (o *OpenAPIFileContext) hasAuthScheme(scheme string) bool {
	for _, ss := range o.API.Components.SecuritySchemes {
		if ss != nil && ss.Value != nil && ss.Value.Scheme == scheme {
			return true
		}
	}

	return false
}

func (o *OpenAPIFileContext) HasBasicAuth() bool {
	return o.hasAuthScheme("basic")
}

func (o *OpenAPIFileContext) HasBearerAuth() bool {
	return o.hasAuthScheme("bearer")
}

func (o *OpenAPIFileContext) HasAnyAuth(op *spec.Operation) bool {
	s := o.OpSecurity(op)
	if len(s) == 0 {
		return false
	}

	for _, group := range s {
		if len(group) > 0 {
			return true
		}
	}

	return false
}

func (o *OpenAPIFileContext) RequiresAuthUser(op *spec.Operation) bool {
	s := o.OpSecurity(op)
	if len(s) == 0 {
		return false
	}

	for _, group := range s {
		if len(group) == 0 {
			return false
		}
	}

	return true
}
