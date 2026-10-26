package demos

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The api-documentation topic's prose quotes this payload as the spec
// facts card; these tests pin what the prose claims — every fact is
// PARSED from the real spec file (never hand-copied), the operations
// table is exactly the five real posts routes, and the wire is camelCase
// with no nulls.

func TestAPIDocsLinkPayloadIsDeterministic(t *testing.T) {
	first, err := BuildAPIDocsLinkPayload()
	if err != nil {
		t.Fatalf("builder must parse the committed spec: %v", err)
	}
	second, err := BuildAPIDocsLinkPayload()
	if err != nil {
		t.Fatalf("builder must parse the committed spec: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("builder must be pure: two calls produced different payloads")
	}
}

func TestAPIDocsLinkSpecFactsComeFromTheRealFile(t *testing.T) {
	payload, err := BuildAPIDocsLinkPayload()
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	spec := payload.Spec
	if spec.Title != "Go Developers Guide — posts API" {
		t.Errorf("title = %q, want the spec's info.title", spec.Title)
	}
	if spec.Version == "" || spec.OpenAPIVersion != "3.1.0" {
		t.Errorf("version/openapiVersion = %q/%q, want a set version and 3.1.0", spec.Version, spec.OpenAPIVersion)
	}
	if !reflect.DeepEqual(spec.Servers, []string{"http://localhost:8100"}) {
		t.Errorf("servers = %v, want the devcontainer server from the spec", spec.Servers)
	}
	if spec.SpecPath != "server/api/openapi.yaml" || spec.SpecURL != "/api/openapi.yaml" {
		t.Errorf("spec locations = %q / %q", spec.SpecPath, spec.SpecURL)
	}
}

func TestAPIDocsLinkOperationsAreTheRealFiveRoutes(t *testing.T) {
	payload, err := BuildAPIDocsLinkPayload()
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	if payload.Spec.PathCount != 2 || payload.Spec.OperationCount != 5 {
		t.Errorf("path/operation counts = %d/%d, want 2 paths and 5 operations", payload.Spec.PathCount, payload.Spec.OperationCount)
	}
	want := []struct{ method, path, id string }{
		{"GET", "/api/posts", "listPosts"},
		{"POST", "/api/posts", "createPost"},
		{"GET", "/api/posts/{id}", "getPost"},
		{"PUT", "/api/posts/{id}", "replacePost"},
		{"DELETE", "/api/posts/{id}", "deletePost"},
	}
	if len(payload.Operations) != len(want) {
		t.Fatalf("operations = %d, want %d", len(payload.Operations), len(want))
	}
	for i, expected := range want {
		operation := payload.Operations[i]
		if operation.Method != expected.method || operation.Path != expected.path || operation.ID != expected.id {
			t.Errorf("operation %d = %s %s (%s), want %s %s (%s)",
				i, operation.Method, operation.Path, operation.ID, expected.method, expected.path, expected.id)
		}
		if operation.Summary == "" {
			t.Errorf("operation %s must carry the spec's summary", operation.ID)
		}
	}
}

func TestAPIDocsLinkSchemasCoverTheContract(t *testing.T) {
	payload, err := BuildAPIDocsLinkPayload()
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	if payload.Spec.SchemaCount != len(payload.Schemas) {
		t.Errorf("schemaCount = %d, schemas = %d — the count must be derived, not asserted", payload.Spec.SchemaCount, len(payload.Schemas))
	}
	names := make(map[string]bool, len(payload.Schemas))
	for _, schema := range payload.Schemas {
		if schema.Name == "" || schema.Description == "" {
			t.Errorf("schema %q needs its spec description", schema.Name)
		}
		names[schema.Name] = true
	}
	for _, required := range []string{"Post", "PostDraft", "PostPage", "ErrorEnvelope", "ErrorBody", "ValidationIssue"} {
		if !names[required] {
			t.Errorf("spec schema inventory lacks %s: %v", required, names)
		}
	}
}

func TestAPIDocsLinkCarriesTheDriftStoryAndRawSpec(t *testing.T) {
	payload, err := BuildAPIDocsLinkPayload()
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	if len(payload.Drift) != 4 {
		t.Errorf("drift rows = %d, want the four enforced guarantees", len(payload.Drift))
	}
	for _, row := range payload.Drift {
		if row.Check == "" || row.Compares == "" || row.Settled == "" {
			t.Errorf("drift row must be complete: %+v", row)
		}
	}
	serving := payload.Serving
	if serving.SpecURL != "/api/openapi.yaml" || serving.ContentType != "application/yaml" || !serving.ServedVerbatim {
		t.Errorf("serving story drifted: %+v", serving)
	}
	if !strings.Contains(serving.SwaggerStory, "Swagger UI") {
		t.Error("the serving story must name Swagger UI")
	}
	// The raw spec text is the very document the facts were parsed from.
	if !strings.Contains(payload.SpecText, "openapi: 3.1.0") || !strings.Contains(payload.SpecText, "operationId: listPosts") {
		t.Error("specText must be the verbatim spec, not a summary")
	}
}

func TestAPIDocsLinkWireShapeIsCamelCaseWithNoNulls(t *testing.T) {
	payload, err := BuildAPIDocsLinkPayload()
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, key := range []string{
		`"openapiVersion"`, `"specPath"`, `"specURL"`, `"pathCount"`, `"operationCount"`,
		`"schemaCount"`, `"validationIssue"`, `"swaggerStory"`, `"specText"`, `"servedVerbatim"`,
	} {
		if !strings.Contains(body, key) {
			t.Errorf("wire shape lacks %s", key)
		}
	}
	if strings.Contains(body, ":null") {
		t.Errorf("collections must marshal as [], never null: %.200s", body)
	}
}
