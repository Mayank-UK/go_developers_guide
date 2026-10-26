package demos

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"guide/internal/domain/posts"
)

// The api-design topic's prose quotes this payload as the contract card;
// these tests pin what the prose claims: the five routes with their exact
// statuses and error cases, pagination constants pulled from the posts
// package (no drift), the envelope anatomy, and determinism.

func TestPostsWorkbenchPayloadIsDeterministic(t *testing.T) {
	first := BuildPostsWorkbenchPayload()
	second := BuildPostsWorkbenchPayload()
	if !reflect.DeepEqual(first, second) {
		t.Fatal("builder must be pure: two calls produced different payloads")
	}
}

func TestPostsWorkbenchRouteTableMatchesTheRealSurface(t *testing.T) {
	payload := BuildPostsWorkbenchPayload()
	want := []struct {
		method   string
		path     string
		status   int
		codes    []string
		statuses []int
	}{
		{"GET", "/api/posts", 200, []string{"validation_failed"}, []int{422}},
		{"POST", "/api/posts", 201, []string{"validation_failed"}, []int{422}},
		{"GET", "/api/posts/{id}", 200, []string{"not_found"}, []int{404}},
		{"PUT", "/api/posts/{id}", 200, []string{"not_found", "validation_failed"}, []int{404, 422}},
		{"DELETE", "/api/posts/{id}", 204, []string{"not_found"}, []int{404}},
	}
	if len(payload.Routes) != len(want) {
		t.Fatalf("routes = %d, want %d", len(payload.Routes), len(want))
	}
	for i, expected := range want {
		route := payload.Routes[i]
		if route.Method != expected.method || route.Path != expected.path {
			t.Errorf("route %d = %s %s, want %s %s", i, route.Method, route.Path, expected.method, expected.path)
		}
		if route.Success.Status != expected.status {
			t.Errorf("route %s success status = %d, want %d", route.ID, route.Success.Status, expected.status)
		}
		if len(route.Errors) != len(expected.codes) {
			t.Fatalf("route %s errors = %d, want %d", route.ID, len(route.Errors), len(expected.codes))
		}
		for j, code := range expected.codes {
			if route.Errors[j].Code != code || route.Errors[j].Status != expected.statuses[j] {
				t.Errorf("route %s error %d = %d/%s, want %d/%s", route.ID, j, route.Errors[j].Status, route.Errors[j].Code, expected.statuses[j], code)
			}
		}
	}
}

func TestPostsWorkbenchPaginationCannotDriftFromTheRules(t *testing.T) {
	pagination := BuildPostsWorkbenchPayload().Pagination
	if pagination.DefaultLimit != posts.DefaultPageLimit || pagination.MaxLimit != posts.MaxPageLimit {
		t.Errorf("limits = %d/%d, want the posts package's %d/%d", pagination.DefaultLimit, pagination.MaxLimit, posts.DefaultPageLimit, posts.MaxPageLimit)
	}
	if pagination.DefaultSort != posts.SortNewest || !reflect.DeepEqual(pagination.SortOptions, posts.SortOptions()) {
		t.Errorf("sort vocabulary = %v (%q), want the posts package's closed set", pagination.SortOptions, pagination.DefaultSort)
	}
	if len(pagination.EnvelopeFields) != 7 || pagination.EnvelopeFields[0] != "items" {
		t.Errorf("envelope fields = %v, want the seven metadata keys starting with items", pagination.EnvelopeFields)
	}
}

func TestPostsWorkbenchRecordsTheDecisions(t *testing.T) {
	payload := BuildPostsWorkbenchPayload()
	if payload.DeleteSemantics.OnMiss != 404 || payload.DeleteSemantics.Chosen == "" || payload.DeleteSemantics.Alternative == "" {
		t.Errorf("delete stance must record the choice AND the alternative: %+v", payload.DeleteSemantics)
	}
	if payload.Envelope.WireShape == "" || len(payload.Envelope.Members) != 4 {
		t.Errorf("envelope anatomy = %+v, want the four members", payload.Envelope)
	}
	if payload.Envelope.Rfc9457Verdict == "" || !strings.Contains(payload.Envelope.Rfc9457Verdict, "RFC 9457") {
		t.Error("the RFC 9457 comparison must be on the card")
	}
	if payload.Resource.CollectionPath != "/api/posts" || payload.Resource.ItemPath != "/api/posts/{id}" {
		t.Errorf("resource paths = %+v", payload.Resource)
	}
}

func TestPostsWorkbenchWireShapeIsCamelCaseWithNoNulls(t *testing.T) {
	raw, err := json.Marshal(BuildPostsWorkbenchPayload())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, key := range []string{
		`"resource"`, `"routes"`, `"pagination"`, `"filtering"`, `"versioning"`,
		`"deleteSemantics"`, `"envelope"`, `"totalItems"`, `"hasNextPage"`,
		`"defaultLimit"`, `"sortOptions"`, `"requestId"`, `"rfc9457Verdict"`,
	} {
		if !strings.Contains(body, key) {
			t.Errorf("wire shape lacks %s", key)
		}
	}
	// JSON null VALUES must never appear (the prose word "null" inside a
	// string is fine — match the value form `:null`).
	if strings.Contains(body, ":null") {
		t.Errorf("collections must marshal as [], never null: %s", body)
	}
}
