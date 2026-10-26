package apispec

import (
	"strings"
	"testing"

	"guide/api"
)

// The spec's own YAML reader, pinned against the REAL committed document:
// every construct the guide's openapi.yaml uses must decode into the
// generic shapes the walker and the demo facts expect, and the parser's
// strictness must bite on the two authoring mistakes it exists to catch.

func TestSpecYAMLParsesTheRealDocument(t *testing.T) {
	document, err := ParseSpecYAML(api.OpenAPIYAML)
	if err != nil {
		t.Fatalf("the committed spec must parse: %v", err)
	}
	if document["openapi"] != "3.1.0" {
		t.Errorf("openapi = %v, want 3.1.0", document["openapi"])
	}

	info, ok := document["info"].(map[string]any)
	if !ok {
		t.Fatalf("info did not decode as a mapping: %T", document["info"])
	}
	if info["title"] != "Go Developers Guide — posts API" || info["version"] != "1.0.0" {
		t.Errorf("info = %v", info)
	}
	// The block scalar (description: |) survives as prose.
	if description, _ := info["description"].(string); !strings.Contains(description, "camelCase") {
		t.Errorf("info.description block scalar lost its content: %q", description)
	}

	servers, ok := document["servers"].([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("servers = %#v, want one entry", document["servers"])
	}
	server, _ := servers[0].(map[string]any)
	if server["url"] != "http://localhost:8100" {
		t.Errorf("server url = %v (a colon in a plain scalar must stay value, not key)", server["url"])
	}
	if _, has := server["description"]; !has {
		t.Error("sequence-of-map continuation keys (dash + 2) did not decode")
	}

	paths, ok := document["paths"].(map[string]any)
	if !ok || len(paths) != 2 {
		t.Fatalf("paths = %#v, want two path items", document["paths"])
	}
	collection, _ := paths["/api/posts"].(map[string]any)
	list, _ := collection["get"].(map[string]any)
	if list["operationId"] != "listPosts" {
		t.Errorf("operationId = %v", list["operationId"])
	}

	// Query parameters: sequence of maps with nested schema blocks.
	parameters, _ := list["parameters"].([]any)
	if len(parameters) != 4 {
		t.Fatalf("parameters = %d, want 4", len(parameters))
	}
	sortParam, _ := parameters[3].(map[string]any)
	if sortParam["name"] != "sort" {
		t.Fatalf("parameter 3 = %v, want sort last", sortParam)
	}
	schema, _ := sortParam["schema"].(map[string]any)
	enum, _ := schema["enum"].([]any)
	if len(enum) != 3 || enum[0] != "newest" {
		t.Errorf("sort enum = %#v, want the flow-sequence vocabulary", enum)
	}
	if schema["default"] != "newest" {
		t.Errorf("sort default = %v", schema["default"])
	}

	// Responses: quoted status keys resolve like any other.
	responses, _ := list["responses"].(map[string]any)
	if _, ok := responses["200"]; !ok {
		t.Error(`quoted key "200" did not decode`)
	}
	if _, ok := responses["422"]; !ok {
		t.Error(`quoted key "422" did not decode`)
	}

	components, _ := document["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if len(schemas) != 6 {
		t.Fatalf("schemas = %d, want 6", len(schemas))
	}

	// PostDraft: plain integers decode as int (bounds and counts rely on it).
	draft, _ := schemas["PostDraft"].(map[string]any)
	properties, _ := draft["properties"].(map[string]any)
	title, _ := properties["title"].(map[string]any)
	if title["minLength"] != 1 || title["maxLength"] != 120 {
		t.Errorf("title bounds = %#v, want ints 1/120", title)
	}
	tags, _ := properties["tags"].(map[string]any)
	if tags["maxItems"] != 8 {
		t.Errorf("tags maxItems = %v, want int 8", tags["maxItems"])
	}

	// PostPage: required is a flow sequence of strings.
	page, _ := schemas["PostPage"].(map[string]any)
	required, _ := page["required"].([]any)
	if len(required) != 7 || required[0] != "items" {
		t.Errorf("PostPage required = %#v", required)
	}
	if page["additionalProperties"] != false {
		t.Errorf("additionalProperties = %v, want bool false", page["additionalProperties"])
	}

	// ErrorBody: the oneOf union with both branches, and $ref values
	// decode unquoted through the quoted-scalar branch.
	body, _ := schemas["ErrorBody"].(map[string]any)
	bodyProps, _ := body["properties"].(map[string]any)
	code, _ := bodyProps["code"].(map[string]any)
	codes, _ := code["enum"].([]any)
	if len(codes) != 8 || codes[0] != "validation_failed" {
		t.Errorf("code enum = %#v", codes)
	}
	details, _ := bodyProps["details"].(map[string]any)
	if details["x-go-type"] != "any" {
		t.Errorf("x-go-type = %v", details["x-go-type"])
	}
	branches, _ := details["oneOf"].([]any)
	if len(branches) != 2 {
		t.Fatalf("oneOf branches = %d, want 2", len(branches))
	}
	arrayBranch, _ := branches[0].(map[string]any)
	arrayItems, _ := arrayBranch["items"].(map[string]any)
	if arrayItems["$ref"] != "#/components/schemas/ValidationIssue" {
		t.Errorf("$ref = %v (quotes must be decoded away)", arrayItems["$ref"])
	}
	objectBranch, _ := branches[1].(map[string]any)
	additional, _ := objectBranch["additionalProperties"].(map[string]any)
	if additional["type"] != "string" {
		t.Errorf("map-branch value schema = %#v", additional)
	}
}

func TestSpecYAMLErrorsOnTheAuthoringMistakes(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"tab indentation", "a:\n\tb: 1\n", "tab in indentation"},
		{"key without colon", "a: 1\nbogus line\n", "no key colon"},
		{"unexpected deeper indent", "a: 1\n  b: 2\n", "unexpected indent"},
		{"scalar document", "just a scalar\n", "no key colon"},
		{"unterminated flow sequence", "a: [b, c\n", "unterminated flow sequence"},
		{"trailing content after quoted scalar", "a: \"b\" c\n", "trailing content after quoted scalar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSpecYAML([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestSpecYAMLDecodesScalarTypes(t *testing.T) {
	document, err := ParseSpecYAML([]byte("a: true\nb: false\nc: 7\nd: 3.1.0\ne: \"quoted: value\"\nf: plain text\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if document["a"] != true || document["b"] != false {
		t.Errorf("booleans = %v/%v", document["a"], document["b"])
	}
	if document["c"] != 7 {
		t.Errorf("integer = %v (%T)", document["c"], document["c"])
	}
	if document["d"] != "3.1.0" {
		t.Errorf("version-like scalar = %v", document["d"])
	}
	if document["e"] != "quoted: value" {
		t.Errorf("quoted scalar with colon = %v", document["e"])
	}
	if document["f"] != "plain text" {
		t.Errorf("plain scalar = %v", document["f"])
	}
}
