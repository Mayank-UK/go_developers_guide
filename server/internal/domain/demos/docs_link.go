// Topic 12 api-documentation's live demo: the docs-link payload — the
// OpenAPI contract as DATA, parsed from the actual spec file at call time.
// Every fact on this card (title, version, servers, the path inventory,
// the schema inventory) is DERIVED from server/api/openapi.yaml via
// the go:embed directive (never hand-copied), so the demo cannot drift
// from the
// document; the drift rows summarize what the spec-drift test enforces;
// and the raw spec text rides along so the view can show the very bytes
// the route serves. Deterministic by construction — no clock, no
// randomness, the same parse on every call.
package demos

import (
	"fmt"
	"sort"
	"strings"

	"guide/api"
	"guide/internal/apispec"
)

// DocsLinkSpecMeta is the spec's own header facts, parsed from the YAML:
// what the document calls itself, which servers it names, and how big its
// surface is.
type DocsLinkSpecMeta struct {
	Title           string   `json:"title"`
	Version         string   `json:"version"`
	OpenAPIVersion  string   `json:"openapiVersion"`
	SpecPath        string   `json:"specPath"`
	SpecURL         string   `json:"specURL"`
	Servers         []string `json:"servers"`
	PathCount       int      `json:"pathCount"`
	OperationCount  int      `json:"operationCount"`
	SchemaCount     int      `json:"schemaCount"`
	ValidationIssue string   `json:"validationIssue"`
}

// DocsLinkOperation is one row of the spec's path inventory: an
// operationId, where it lives, and what it claims to do.
type DocsLinkOperation struct {
	ID      string `json:"id"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

// DocsLinkSchema is one named schema the spec defines under
// components/schemas — the inventory codegen turns into Go types.
type DocsLinkSchema struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// DocsLinkServing is the docs-serving story: where the spec lives at
// runtime, how it is served, and how interactive docs hang off it.
type DocsLinkServing struct {
	SpecURL        string `json:"specURL"`
	ContentType    string `json:"contentType"`
	ServedVerbatim bool   `json:"servedVerbatim"`
	SwaggerStory   string `json:"swaggerStory"`
}

// DocsLinkDriftRow summarizes one spec-vs-wire guarantee the drift test
// enforces: what is compared, and what settles it.
type DocsLinkDriftRow struct {
	Check    string `json:"check"`
	Compares string `json:"compares"`
	Settled  string `json:"settled"`
}

// APIDocsLinkPayload is the whole GET /api/demo/api/docs-link body. Every
// collection is non-nil so the wire renders [] — never null — and
// SpecText carries the spec verbatim for the view's raw pane.
type APIDocsLinkPayload struct {
	Spec       DocsLinkSpecMeta    `json:"spec"`
	Operations []DocsLinkOperation `json:"operations"`
	Schemas    []DocsLinkSchema    `json:"schemas"`
	Serving    DocsLinkServing     `json:"serving"`
	Drift      []DocsLinkDriftRow  `json:"drift"`
	SpecText   string              `json:"specText"`
}

// BuildAPIDocsLinkPayload parses the embedded OpenAPI document and shapes
// the demo card from it. A spec that fails to parse is a build-time
// contract break, not a demo malfunction: the builder reports it as an
// error the thin handler has no business swallowing.
func BuildAPIDocsLinkPayload() (APIDocsLinkPayload, error) {
	facts, err := parseSpecFacts(api.OpenAPIYAML)
	if err != nil {
		return APIDocsLinkPayload{}, fmt.Errorf("docs-link demo: %w", err)
	}
	return APIDocsLinkPayload{
		Spec:       facts.meta,
		Operations: facts.operations,
		Schemas:    facts.schemas,
		Serving: DocsLinkServing{
			SpecURL:        api.SpecURL,
			ContentType:    "application/yaml",
			ServedVerbatim: true,
			SwaggerStory: "the route serves the file verbatim, so any OpenAPI-aware tool can consume it — " +
				"point Swagger UI (or Redoc, or Postman) at the URL and it renders the interactive docs " +
				"from the same bytes this demo parsed; the guide deliberately ships no bundled UI page, " +
				"because the machine-readable document plus a viewer you already trust beats a second " +
				"hand-built docs surface to maintain",
		},
		Drift: []DocsLinkDriftRow{
			{
				Check:    "list response ↔ PostPage",
				Compares: "GET /api/posts body: every key, type, and bound against the schema",
				Settled:  "spec_drift_test.go drives the live handler and walks the JSON",
			},
			{
				Check:    "single post ↔ Post",
				Compares: "GET /api/posts/{id} body: field names, tags never null, RFC 3339 timestamps",
				Settled:  "the walker refuses unknown keys (additionalProperties: false)",
			},
			{
				Check:    "miss ↔ ErrorEnvelope (404)",
				Compares: "the 404 body: code enum, message, requestId present; details omitted",
				Settled:  "optional members must be absent, never null, to validate",
			},
			{
				Check:    "broken input ↔ ErrorEnvelope (422)",
				Compares: "the 422 body's details: issue array OR decoder map, per oneOf",
				Settled:  "both branches of the union are exercised by the test",
			},
		},
		SpecText: string(api.OpenAPIYAML),
	}, nil
}

// specFacts is everything the demo needs from one parse of the YAML.
type specFacts struct {
	meta       DocsLinkSpecMeta
	operations []DocsLinkOperation
	schemas    []DocsLinkSchema
}

// httpMethods is the method order the inventory renders in — the order a
// route table reads naturally (lowercase because OpenAPI path-item keys
// are lowercase; the card shows the uppercase verb).
var httpMethods = []string{"get", "post", "put", "patch", "delete"}

// parseSpecFacts walks the decoded YAML and derives every fact the card
// shows. Nothing is hard-coded from the document: rename a path or schema
// in the spec and the card follows.
func parseSpecFacts(raw []byte) (specFacts, error) {
	var document map[string]any
	document, err := apispec.ParseSpecYAML(raw)
	if err != nil {
		return specFacts{}, fmt.Errorf("parse %s: %w", api.SpecPath, err)
	}

	info, _ := document["info"].(map[string]any)
	openAPIVersion, _ := document["openapi"].(string)
	title, _ := info["title"].(string)
	version, _ := info["version"].(string)
	if title == "" || version == "" || openAPIVersion == "" {
		return specFacts{}, fmt.Errorf("parse %s: info.title, info.version, and openapi are required", api.SpecPath)
	}

	servers := stringSliceOf(document["servers"], "url")

	var operations []DocsLinkOperation
	paths, _ := document["paths"].(map[string]any)
	pathNames := make([]string, 0, len(paths))
	for name := range paths {
		pathNames = append(pathNames, name)
	}
	sort.Strings(pathNames)
	for _, pathName := range pathNames {
		item, _ := paths[pathName].(map[string]any)
		for _, method := range httpMethods {
			operation, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			id, _ := operation["operationId"].(string)
			summary, _ := operation["summary"].(string)
			operations = append(operations, DocsLinkOperation{
				ID: id, Method: strings.ToUpper(method), Path: pathName, Summary: summary,
			})
		}
	}
	if operations == nil {
		operations = []DocsLinkOperation{} // the wire renders [], never null
	}

	var schemas []DocsLinkSchema
	components, _ := document["components"].(map[string]any)
	defined, _ := components["schemas"].(map[string]any)
	schemaNames := make([]string, 0, len(defined))
	for name := range defined {
		schemaNames = append(schemaNames, name)
	}
	sort.Strings(schemaNames)
	for _, name := range schemaNames {
		schema, _ := defined[name].(map[string]any)
		description, _ := schema["description"].(string)
		schemas = append(schemas, DocsLinkSchema{Name: name, Description: description})
	}

	return specFacts{
		meta: DocsLinkSpecMeta{
			Title:          title,
			Version:        version,
			OpenAPIVersion: openAPIVersion,
			SpecPath:       api.SpecPath,
			SpecURL:        api.SpecURL,
			Servers:        servers,
			PathCount:      len(pathNames),
			OperationCount: len(operations),
			SchemaCount:    len(schemas),
			ValidationIssue: "the drift test walks live JSON against these schemas — " +
				"a spec that lies about the wire fails the suite",
		},
		operations: operations,
		schemas:    schemas,
	}, nil
}

// stringSliceOf coerces a YAML list of objects into the one field each
// entry contributes.
func stringSliceOf(value any, key string) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if entry, ok := item.(map[string]any); ok {
			if text, ok := entry[key].(string); ok {
				out = append(out, text)
			}
		}
	}
	return out
}
