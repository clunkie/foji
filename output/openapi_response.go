package output

import (
	"maps"
	"slices"
	"strings"

	"github.com/codemodus/kace"

	"github.com/gofoji/foji/input/openapi/spec"
)

/* Request body and response selection helpers. */

var knownInterfaces = []string{"string", "io.Reader"}

func (o *OpenAPIFileContext) GetRequestBody(op *spec.Operation) *OpBody {
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		mediaType := op.RequestBody.Value.Content.Get(ApplicationJSON)
		if mediaType != nil {
			return &OpBody{MimeType: ApplicationJSON, Schema: mediaType.Schema}
		}

		mediaType = op.RequestBody.Value.Content.Get(TextPlain)
		if mediaType != nil {
			return &OpBody{MimeType: TextPlain, Schema: mediaType.Schema}
		}

		mediaType = op.RequestBody.Value.Content.Get(ApplicationForm)
		if mediaType != nil {
			return &OpBody{MimeType: ApplicationForm, Schema: mediaType.Schema}
		}

		mediaType = op.RequestBody.Value.Content.Get(MultipartForm)
		if mediaType != nil {
			return &OpBody{MimeType: MultipartForm, Schema: mediaType.Schema}
		}
	}

	return nil
}

func (o *OpenAPIFileContext) GetRequestBodySchemas(op *spec.Operation) []OpBody {
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}

	var out []OpBody

	for k, v := range op.RequestBody.Value.Content {
		if v.Schema == nil {
			continue
		}

		if k == ApplicationJSON || k == ApplicationForm || k == MultipartForm {
			out = append(out, OpBody{MimeType: MimeType(k), Schema: v.Schema})
		}
	}

	return out
}

func happyStatusCode(key string) bool {
	if len(key) != 3 { //nolint:mnd
		return false
	}

	return key[0] == '2' || key[0] == '3'
}

func (o *OpenAPIFileContext) GetOpHappyResponse(pkg string, op *spec.Operation) OpResponse {
	supportedResponseContentTypes := []string{ApplicationJSON, ApplicationJSONL, TextPlain, TextHTML, TextCSV}

	// Responses are held in a map, so order here by "happy key" to make sure we
	// choose a happy response deterministically

	happyKeys := []string{}

	for key := range op.Responses.Map() {
		if happyStatusCode(key) {
			happyKeys = append(happyKeys, key)
		}
	}

	slices.Sort(happyKeys)

	for _, key := range happyKeys {
		r := op.Responses.Map()[key]
		for _, mimeType := range supportedResponseContentTypes {
			mediaType := r.Value.Content.Get(mimeType)
			if mediaType != nil {
				mime := MimeType(mimeType)
				t := o.GetType(pkg, kace.Pascal(op.OperationID)+" Response", mediaType.Schema)

				if t == "" {
					// Unknown type, use []byte by default
					t = "[]byte"
				}

				var goType string

				if strings.HasPrefix(t, "[]") || strings.HasPrefix(t, "map[") || slices.Contains(knownInterfaces, t) {
					goType = t
				} else {
					goType = "*" + t
				}

				if r.Value.Headers != nil {
					return OpResponse{Key: key, MimeType: mime, MediaType: mediaType, GoType: goType, Headers: mapKeysSorted(r.Value.Headers)}
				}

				return OpResponse{Key: key, MimeType: mime, MediaType: mediaType, GoType: goType, Headers: []string{}}
			}
		}
	}

	// No response with a supported content type found, but maybe there's a response with headers only
	for _, key := range happyKeys {
		r := op.Responses.Map()[key]
		if len(r.Value.Headers) > 0 {
			return OpResponse{Key: key, MimeType: "", MediaType: nil, GoType: "", Headers: mapKeysSorted(r.Value.Headers)}
		}
	}

	happyKey := "200"
	if len(happyKeys) > 0 {
		// If none of the responses have a supported content type, use the first "happy" response
		happyKey = happyKeys[0]
	}

	return OpResponse{Key: happyKey, MimeType: "", MediaType: nil, GoType: ""}
}

func mapKeysSorted[T any](in map[string]T) []string {
	out := slices.Collect(maps.Keys(in))
	slices.Sort(out)

	return out
}

func (o *OpenAPIFileContext) GetOpHappyResponseKey(op *spec.Operation) string {
	// passing "" as pkg because here we only need the Key part for which pkg is not needed
	opResponse := o.GetOpHappyResponse("", op)
	return opResponse.Key
}

func (o *OpenAPIFileContext) GetOpHappyResponseMimeType(op *spec.Operation) string {
	// passing "" as pkg because here we only need the MimeType part for which pkg is not needed
	opResponse := o.GetOpHappyResponse("", op)
	return opResponse.String()
}

func (o *OpenAPIFileContext) GetOpHappyResponseType(pkg string, op *spec.Operation) string {
	opResponse := o.GetOpHappyResponse(pkg, op)
	return opResponse.GoType
}

func (o *OpenAPIFileContext) GetOpHappyResponseHeaders(pkg string, op *spec.Operation) []string {
	opResponse := o.GetOpHappyResponse(pkg, op)
	return opResponse.Headers
}
