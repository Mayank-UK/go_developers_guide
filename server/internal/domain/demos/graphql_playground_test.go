package demos

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	postsdata "guide/internal/data/posts"
	"guide/internal/domain/posts"
)

// scratchFactory adapts the in-memory constructor into the port-typed
// factory the builder takes (Go has no function covariance — the closure
// is the one-line adapter the composition root writes too).
func scratchFactory() func() posts.PostsRepository {
	return func() posts.PostsRepository { return postsdata.NewInMemoryRepository() }
}

// The graphql topic's prose quotes this payload as the playground card;
// these tests pin what the prose claims: determinism, the executed runs'
// real values (page envelope, p-7 round-trip, extension codes), the
// loader experiment's N-versus-one counts, and the schema card being read
// off the real parsed schema.

func TestGraphQLPlaygroundPayloadIsDeterministic(t *testing.T) {
	first := BuildGraphQLPlaygroundPayload(scratchFactory())
	second := BuildGraphQLPlaygroundPayload(scratchFactory())
	if !reflect.DeepEqual(first, second) {
		t.Fatal("builder must be pure: two scratch worlds produced different payloads")
	}
}

func TestGraphQLPlaygroundRunsCoverEveryTeachingBeat(t *testing.T) {
	payload := BuildGraphQLPlaygroundPayload(scratchFactory())

	byID := make(map[string]GraphQLRunResult, len(payload.Runs))
	for _, run := range payload.Runs {
		if _, repeated := byID[run.ID]; repeated {
			t.Errorf("run %q repeats", run.ID)
		}
		byID[run.ID] = run
	}
	for _, id := range []string{
		"list-first-page", "read-one", "create-post", "update-post",
		"delete-post", "error-not-found", "error-validation", "introspection",
	} {
		if _, ok := byID[id]; !ok {
			t.Errorf("run %q missing", id)
		}
	}
	// Samples and runs share one source: same ids, same query texts.
	if len(payload.Samples) != len(payload.Runs) {
		t.Fatalf("samples = %d, runs = %d — they must share one spec list", len(payload.Samples), len(payload.Runs))
	}
	for i, sample := range payload.Samples {
		if payload.Runs[i].ID != sample.ID || payload.Runs[i].Query != sample.Query {
			t.Errorf("sample %q and run %q drifted apart", sample.ID, payload.Runs[i].ID)
		}
	}
}

func TestGraphQLPlaygroundRunsPinTheRealValues(t *testing.T) {
	payload := BuildGraphQLPlaygroundPayload(scratchFactory())
	runs := make(map[string]GraphQLRunResult, len(payload.Runs))
	for _, run := range payload.Runs {
		runs[run.ID] = run
	}

	// The paginated query: honest metadata over the six-post seed.
	list := decodeRunData(t, runs["list-first-page"])
	page, _ := list["posts"].(map[string]any)
	if page == nil || page["totalItems"].(float64) != 6 || page["totalPages"].(float64) != 3 {
		t.Errorf("list run page = %v, want totals 6/3", list)
	}
	if page["hasNextPage"] != true || page["hasPrevPage"] != false {
		t.Errorf("list run flags = %v, want true/false", page)
	}
	items, _ := page["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != "p-1" {
		t.Errorf("list run items = %v, want the two oldest starting p-1", items)
	}

	// The mutation round-trip on the scratch world: p-7 created, replaced, deleted.
	if created := decodeRunData(t, runs["create-post"])["createPost"].(map[string]any); created["id"] != "p-7" {
		t.Errorf("createPost = %v, want p-7 on the fresh scratch world", created)
	}
	if updated := decodeRunData(t, runs["update-post"])["updatePost"].(map[string]any); updated["title"] != "Edited through the graph" {
		t.Errorf("updatePost = %v, want the replaced title", updated)
	}
	if deleted := decodeRunData(t, runs["delete-post"])["deletePost"]; deleted != "p-7" {
		t.Errorf("deletePost = %v, want the removed id p-7", deleted)
	}

	// Introspection through the same execution path the endpoint serves.
	introspected := decodeRunData(t, runs["introspection"])["__schema"].(map[string]any)
	if introspected["queryType"].(map[string]any)["name"] != "Query" {
		t.Errorf("__schema = %v, want the Query root", introspected)
	}
}

func TestGraphQLPlaygroundErrorsCarryTheTaxonomyCodes(t *testing.T) {
	payload := BuildGraphQLPlaygroundPayload(scratchFactory())
	runs := make(map[string]GraphQLRunResult, len(payload.Runs))
	for _, run := range payload.Runs {
		runs[run.ID] = run
	}

	miss := runs["error-not-found"]
	if len(miss.Errors) != 1 || miss.Errors[0].Extensions["code"] != "not_found" {
		t.Fatalf("not-found run errors = %+v, want one entry coded not_found", miss.Errors)
	}
	if data := decodeRunData(t, miss); data["post"] != nil {
		t.Errorf("not-found run data.post = %v, want null beside the error", data["post"])
	}
	if !strings.Contains(miss.Errors[0].Message, "p-999") {
		t.Errorf("not-found message = %q, want the id named", miss.Errors[0].Message)
	}

	violation := runs["error-validation"]
	if len(violation.Errors) != 1 || violation.Errors[0].Extensions["code"] != "validation_failed" {
		t.Fatalf("validation run errors = %+v, want one entry coded validation_failed", violation.Errors)
	}
	details, ok := violation.Errors[0].Extensions["details"].([]posts.Issue)
	if !ok || len(details) == 0 {
		t.Errorf("validation extensions.details = %v, want the issue list riding along", violation.Errors[0].Extensions)
	}

	// Clean runs carry an empty errors list — [], never null.
	if runs := payload.Runs; runs[0].Errors == nil {
		t.Error("clean runs must materialize errors as [], not nil")
	}
}

func TestGraphQLPlaygroundLoaderExperimentProvesNPlusOne(t *testing.T) {
	experiment := BuildGraphQLPlaygroundPayload(scratchFactory()).Loader

	if experiment.PostCount != 6 {
		t.Fatalf("post count = %d, want the six deterministic seeds", experiment.PostCount)
	}
	if experiment.NaivePortCalls != 7 {
		t.Errorf("naive port calls = %d (breakdown %+v), want 7 = 1 list + 6 getById", experiment.NaivePortCalls, experiment.NaiveBreakdown)
	}
	if experiment.NaiveBreakdown.List != 1 || experiment.NaiveBreakdown.GetByID != 6 {
		t.Errorf("naive breakdown = %+v, want list 1 + getById 6", experiment.NaiveBreakdown)
	}
	if experiment.LoaderPortCalls != 1 || experiment.LoaderBreakdown.List != 1 {
		t.Errorf("loader port calls = %d (breakdown %+v), want the single batched list", experiment.LoaderPortCalls, experiment.LoaderBreakdown)
	}
	if experiment.LoadsIssued != 12 || experiment.DuplicateLoads != 6 || !experiment.AllResolved {
		t.Errorf("loader run = %+v, want 12 loads (6 duplicates), all resolved", experiment)
	}
}

func TestGraphQLPlaygroundSchemaCardComesFromTheRealSchema(t *testing.T) {
	card := BuildGraphQLPlaygroundPayload(scratchFactory()).Schema

	wantOps := []string{
		"query posts(page: Int, limit: Int, tag: String, sort: String): PostPage!",
		"query post(id: ID!): Post",
		"mutation createPost(input: PostInput!): Post!",
		"mutation updatePost(id: ID!, input: PostInput!): Post",
		"mutation deletePost(id: ID!): ID",
	}
	if !reflect.DeepEqual(card.Operations, wantOps) {
		t.Errorf("operations = %v, want %v", card.Operations, wantOps)
	}

	byName := make(map[string]GraphQLTypeCard, len(card.Types))
	for _, typeCard := range card.Types {
		byName[typeCard.Name] = typeCard
	}
	post := byName["Post"]
	if post.Kind != "object" || len(post.Fields) != 6 {
		t.Errorf("Post card = %+v, want the six REST wire fields", post)
	}
	if page := byName["PostPage"]; page.Kind != "object" || len(page.Fields) != 7 {
		t.Errorf("PostPage card = %+v, want the seven envelope fields", page)
	}
	if input := byName["PostInput"]; input.Kind != "input" || len(input.Fields) != 3 {
		t.Errorf("PostInput card = %+v, want the three draft fields", input)
	}
}

func TestGraphQLPlaygroundEndpointAndWireShape(t *testing.T) {
	payload := BuildGraphQLPlaygroundPayload(scratchFactory())

	if payload.Endpoint.Path != "/api/graphql" || payload.Endpoint.Method != "POST" {
		t.Errorf("endpoint = %+v, want POST /api/graphql", payload.Endpoint)
	}
	if !strings.Contains(payload.Endpoint.RequestBody, `"query"`) {
		t.Errorf("request body shape = %q, must show the {query,…} document", payload.Endpoint.RequestBody)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, key := range []string{
		`"requestBody"`, `"liveNote"`, `"operations"`, `"totalItems"`, `"hasNextPage"`,
		`"loadsIssued"`, `"duplicateLoads"`, `"naiveBreakdown"`, `"getById"`, `"verdict"`,
		`"extensions"`,
	} {
		if !strings.Contains(body, key) {
			t.Errorf("wire shape lacks %s", key)
		}
	}
	// The structural payload (everything except the executed run
	// documents) must never marshal a null value: collections render [],
	// optionals are omitted. Inside runs[].data a null is legitimate
	// GraphQL — a nullable field answered null beside an error — so those
	// embedded documents are judged by their own tests instead.
	structural := struct {
		Endpoint GraphQLEndpointMeta     `json:"endpoint"`
		Schema   GraphQLSchemaCard       `json:"schema"`
		Samples  []GraphQLSample         `json:"samples"`
		Loader   GraphQLLoaderExperiment `json:"loader"`
	}{payload.Endpoint, payload.Schema, payload.Samples, payload.Loader}
	structuralRaw, err := json.Marshal(structural)
	if err != nil {
		t.Fatalf("marshal structural: %v", err)
	}
	if strings.Contains(string(structuralRaw), ":null") {
		t.Errorf("collections must marshal as [] and optional objects as {}, never null: %s", structuralRaw)
	}
}

// decodeRunData decodes one run's data document.
func decodeRunData(t *testing.T, run GraphQLRunResult) map[string]any {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal(run.Data, &data); err != nil {
		t.Fatalf("run %q data is not an object: %v (%s)", run.ID, err, run.Data)
	}
	return data
}

// Compile-time pin: the scratch factory the composition root injects is
// exactly the in-memory constructor's shape.
var _ = scratchFactory()
