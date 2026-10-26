package posts

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"guide/internal/core"
)

// Domain tests for the posts substrate: draft normalization + validation,
// the list-query boundary rules, and the use cases' behavior through a
// fake repository (the domain never imports the data layer; the fake is
// the port's other side). The in-memory adapter's own tests live beside it
// in internal/data/posts.

// fakeRepository is a minimal PostsRepository double: it records calls and
// answers from a canned store, good enough to pin the use cases' rule:
// normalize first, validate second, store only when clean, and pass typed
// misses through untouched.
type fakeRepository struct {
	listed   []ListQuery
	stored   []Draft
	replaced map[string]Draft
	deleted  []string
	misses   bool
}

func (f *fakeRepository) List(_ context.Context, query ListQuery) (PostPage, error) {
	f.listed = append(f.listed, query)
	return PostPage{Items: nil, Page: query.Page, Limit: query.Limit}, nil
}

func (f *fakeRepository) GetByID(_ context.Context, id string) (Post, error) {
	if f.misses {
		return Post{}, core.NotFound("Post missing.")
	}
	return Post{ID: id, Title: "cached"}, nil
}

func (f *fakeRepository) Create(_ context.Context, draft Draft) (Post, error) {
	f.stored = append(f.stored, draft)
	return Post{ID: "p-new", Title: draft.Title}, nil
}

func (f *fakeRepository) Replace(_ context.Context, id string, draft Draft) (Post, error) {
	if f.misses {
		return Post{}, core.NotFound("Post missing.")
	}
	if f.replaced == nil {
		f.replaced = map[string]Draft{}
	}
	f.replaced[id] = draft
	return Post{ID: id, Title: draft.Title}, nil
}

func (f *fakeRepository) Delete(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	if f.misses {
		return core.NotFound("Post missing.")
	}
	return nil
}

func TestDraftNormalizeCanonicalizesInput(t *testing.T) {
	got := Draft{
		Title: "  Trimmed Title\t",
		Body:  " body keeps inner spacing ",
		Tags:  []string{"  Go ", "HTTP", "", "go"},
	}.NormalizeDraft()
	want := Draft{Title: "Trimmed Title", Body: "body keeps inner spacing", Tags: []string{"go", "http", "go"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalized = %+v, want %+v", got, want)
	}
}

func TestDraftValidateCollectsEveryIssue(t *testing.T) {
	issues := Draft{Title: "", Body: strings.Repeat("x", maxBodyRunes+1), Tags: nil}.ValidateDraft()
	if len(issues) != 2 {
		t.Fatalf("issues = %d, want 2 (title + body)", len(issues))
	}
	if issues[0].Field != "title" || issues[1].Field != "body" {
		t.Errorf("issue fields = %q/%q, want title then body", issues[0].Field, issues[1].Field)
	}
	// Issues are JSON-structured for the envelope's details.
	raw, err := json.Marshal(issues)
	if err != nil {
		t.Fatalf("marshal issues: %v", err)
	}
	for _, key := range []string{`"field"`, `"rule"`, `"got"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("issue wire shape lacks %s: %s", key, raw)
		}
	}
}

func TestDraftValidateEnforcesTagRules(t *testing.T) {
	tooMany := make([]string, maxTags+1)
	for i := range tooMany {
		tooMany[i] = "tag"
	}
	if issues := (Draft{Title: "t", Body: "b", Tags: tooMany}).ValidateDraft(); len(issues) != 1 || issues[0].Field != "tags" {
		t.Errorf("tag count issues = %+v, want one tags issue", issues)
	}
	longTag := strings.Repeat("x", maxTagRunes+1)
	if issues := (Draft{Title: "t", Body: "b", Tags: []string{longTag}}).ValidateDraft(); len(issues) != 1 || issues[0].Field != "tags" {
		t.Errorf("tag length issues = %+v, want one tags issue", issues)
	}
	if issues := (Draft{Title: "t", Body: "b", Tags: []string{"ok", strings.Repeat("y", maxTagRunes)}}).ValidateDraft(); len(issues) != 0 {
		t.Errorf("boundary tag length flagged: %+v", issues)
	}
}

func TestNormalizeListQueryAppliesDefaults(t *testing.T) {
	query, err := NormalizeListQuery("", "", "", "")
	if err != nil {
		t.Fatalf("defaults must normalize cleanly: %v", err)
	}
	want := ListQuery{Page: 1, Limit: DefaultPageLimit, Tag: "", Sort: SortNewest}
	if !reflect.DeepEqual(query, want) {
		t.Errorf("defaults = %+v, want %+v", query, want)
	}
}

func TestNormalizeListQueryAcceptsExplicitValues(t *testing.T) {
	query, err := NormalizeListQuery("3", "25", "go", SortOldest)
	if err != nil {
		t.Fatalf("explicit values must normalize cleanly: %v", err)
	}
	want := ListQuery{Page: 3, Limit: 25, Tag: "go", Sort: SortOldest}
	if !reflect.DeepEqual(query, want) {
		t.Errorf("query = %+v, want %+v", query, want)
	}
	boundary, err := NormalizeListQuery("1", strconv.Itoa(MaxPageLimit), "", SortTitle)
	if err != nil || boundary.Limit != MaxPageLimit {
		t.Errorf("limit 50 (the ceiling) must be accepted, got %+v err %v", boundary, err)
	}
}

func TestNormalizeListQueryCollectsEveryViolation(t *testing.T) {
	_, err := NormalizeListQuery("0", "abc", "", "newest")
	var appErr *core.Error
	if !errors.As(err, &appErr) || appErr.Kind != core.KindValidationFailed {
		t.Fatalf("err = %v, want a validation_failed typed error", err)
	}
	issues, ok := appErr.Details.([]Issue)
	if !ok {
		t.Fatalf("details = %T, want []Issue", appErr.Details)
	}
	if len(issues) != 2 || issues[0].Field != "page" || issues[1].Field != "limit" {
		t.Errorf("issues = %+v, want page then limit", issues)
	}

	_, err = NormalizeListQuery("", strconv.Itoa(MaxPageLimit+1), "", "bogus")
	if err == nil {
		t.Fatal("over-limit + unknown sort must fail")
	}
	if !strings.Contains(err.Error(), "Invalid post list query") {
		t.Errorf("message = %q", err)
	}
	issues, ok = asIssues(t, err)
	if !ok || len(issues) != 2 || issues[1].Field != "sort" {
		t.Errorf("issues = %+v, want limit then sort", issues)
	}
}

func TestSortVocabularyIsClosed(t *testing.T) {
	options := SortOptions()
	if len(options) != 3 || options[0] != SortNewest {
		t.Fatalf("options = %v, want newest, oldest, title", options)
	}
	for _, option := range options {
		if !SortValid(option) {
			t.Errorf("option %q must validate", option)
		}
	}
	if SortValid("NEWEST") || SortValid("") || SortValid("-createdAt") {
		t.Error("the vocabulary is closed: cased and unknown values must not validate")
	}
}

func TestCreatePostUseCaseStoresOnlyCleanDrafts(t *testing.T) {
	repo := &fakeRepository{}
	create := NewCreatePostUseCase(repo)

	post, err := create.Execute(context.Background(), Draft{Title: "  Hello ", Body: "World", Tags: []string{" Go "}})
	if err != nil {
		t.Fatalf("clean draft must store: %v", err)
	}
	if post.ID != "p-new" {
		t.Errorf("stored post = %+v", post)
	}
	stored := repo.stored[0]
	if stored.Title != "Hello" || stored.Tags[0] != "go" {
		t.Errorf("stored draft must be normalized: %+v", stored)
	}

	_, err = create.Execute(context.Background(), Draft{Title: "", Body: ""})
	var appErr *core.Error
	if !errors.As(err, &appErr) || appErr.Kind != core.KindValidationFailed {
		t.Fatalf("dirty draft err = %v, want validation_failed", err)
	}
	if len(repo.stored) != 1 {
		t.Errorf("dirty draft must not reach the repository (stored %d)", len(repo.stored))
	}
}

func TestReplaceAndDeletePassTypedMissesThrough(t *testing.T) {
	repo := &fakeRepository{misses: true}
	replace := NewReplacePostUseCase(repo)
	delete := NewDeletePostUseCase(repo)
	get := NewGetPostUseCase(repo)

	if _, err := replace.Execute(context.Background(), "p-404", Draft{Title: "t", Body: "b"}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("replace miss = %v, want the typed NotFound sentinel match", err)
	}
	if err := delete.Execute(context.Background(), "p-404"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("delete miss = %v, want the typed NotFound sentinel match", err)
	}
	if _, err := get.Execute(context.Background(), "p-404"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("get miss = %v, want the typed NotFound sentinel match", err)
	}
}

func TestListUseCaseHandsTheQueryToThePort(t *testing.T) {
	repo := &fakeRepository{}
	list := NewListPostsUseCase(repo)
	query := ListQuery{Page: 2, Limit: 5, Tag: "api", Sort: SortTitle}
	if _, err := list.Execute(context.Background(), query); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(repo.listed) != 1 || repo.listed[0] != query {
		t.Errorf("port saw %+v, want %+v", repo.listed, query)
	}
}

// asIssues pulls the typed details out of a validation error.
func asIssues(t *testing.T, err error) ([]Issue, bool) {
	t.Helper()
	var appErr *core.Error
	if !errors.As(err, &appErr) {
		return nil, false
	}
	issues, ok := appErr.Details.([]Issue)
	return issues, ok
}
