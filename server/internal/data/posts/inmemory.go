// Package postsmemory is the in-memory PostsRepository adapter: a
// mutex-guarded map seeded with deterministic guide posts. It is the SWAP
// SEAM the databases topic replaces — a PostgreSQL adapter will implement
// the same posts.PostsRepository port and be wired at the composition root
// instead, and neither the domain nor the handlers will learn which store
// answered. Until then it is the honest stand-in: real pagination, real
// filtering, real typed misses, safe for concurrent use (-race clean).
package postsmemory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// seedCreatedAt anchors the six seed posts on fixed January 2026 dates so
// every boot lists in the same order — the demo, the terminal companion,
// and the smoke tests all depend on that determinism.
var seedCreatedAt = []time.Time{
	time.Date(2026, 1, 5, 9, 15, 0, 0, time.UTC),
	time.Date(2026, 1, 7, 14, 40, 0, 0, time.UTC),
	time.Date(2026, 1, 10, 8, 5, 0, 0, time.UTC),
	time.Date(2026, 1, 12, 17, 25, 0, 0, time.UTC),
	time.Date(2026, 1, 15, 11, 50, 0, 0, time.UTC),
	time.Date(2026, 1, 18, 16, 30, 0, 0, time.UTC),
}

// seedPosts is the deterministic starting collection, parallel to
// seedCreatedAt (post i was created at seedCreatedAt[i]).
var seedPosts = []posts.Draft{
	{
		Title: "Why Go for backends",
		Body: "One static binary, a stdlib that ships an HTTP server, and " +
			"concurrency as a language feature instead of a framework — the " +
			"deployment story alone earns Go a spot in the backend conversation.",
		Tags: []string{"go", "architecture"},
	},
	{
		Title: "ServeMux patterns in Go 1.22",
		Body: "Method-plus-path patterns ended the era of third-party routers " +
			"for plain APIs: \"GET /api/posts/{id}\" registers exactly one route " +
			"and hands the wildcard to the handler via r.PathValue.",
		Tags: []string{"go", "http", "routing"},
	},
	{
		Title: "Errors are values",
		Body: "A Go error is just a value you can type, compare with errors.Is, " +
			"and wrap with %w — no exceptions unwinding the stack, no hidden " +
			"control flow. The whole error-handling story builds on that one " +
			"honest decision.",
		Tags: []string{"go", "errors"},
	},
	{
		Title: "Designing JSON APIs",
		Body: "Resources are nouns, methods are verbs, and status codes are " +
			"the contract's vocabulary. Get those three right and the endpoints " +
			"design themselves.",
		Tags: []string{"api", "http"},
	},
	{
		Title: "Pagination that tells the truth",
		Body: "A page envelope with totalItems and totalPages lets the client " +
			"render honest navigation; an empty past-the-end page is data about " +
			"where the collection ended, not an error.",
		Tags: []string{"api", "design"},
	},
	{
		Title: "Structured logging with slog",
		Body: "slog turns logs into key-value records the moment they are " +
			"written — JSON to stdout, one request id per line, and grepping " +
			"by struct field instead of by regex.",
		Tags: []string{"go", "observability"},
	},
}

// Repository is the in-memory PostsRepository: an RWMutex-guarded map plus
// the deterministic seed. Reads (List, GetByID) take the read lock;
// writes (Create, Replace, Delete) take the write lock — every method
// copies what it returns, so callers can never mutate stored state
// through a handed-out post.
type Repository struct {
	mu     sync.RWMutex
	store  map[string]posts.Post
	nextID int
	now    func() time.Time
}

// NewRepository builds the seeded repository on the production clock
// (UTC now for every write it stamps).
func NewRepository() *Repository {
	return newRepository(func() time.Time { return time.Now().UTC() })
}

// newRepository seeds the store on an injected clock — the seam that keeps
// repository tests deterministic.
func newRepository(now func() time.Time) *Repository {
	store := make(map[string]posts.Post, len(seedPosts))
	for i, draft := range seedPosts {
		id := fmt.Sprintf("p-%d", i+1)
		created := seedCreatedAt[i]
		store[id] = posts.Post{
			ID:        id,
			Title:     draft.Title,
			Body:      draft.Body,
			Tags:      append([]string(nil), draft.Tags...),
			CreatedAt: created,
			UpdatedAt: created,
		}
	}
	return &Repository{store: store, nextID: len(seedPosts) + 1, now: now}
}

// List implements the port: filter by exact tag match, sort by the closed
// vocabulary, slice the requested page, and return the page metadata with
// the TOTAL matched count (not the page count) so totals stay true under
// every limit. Every comparison ties back to the id, keeping the order
// deterministic even for equal timestamps or titles.
func (r *Repository) List(_ context.Context, query posts.ListQuery) (posts.PostPage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filtered := make([]posts.Post, 0, len(r.store))
	for _, post := range r.store {
		if query.Tag != "" && !containsTag(post.Tags, query.Tag) {
			continue
		}
		filtered = append(filtered, copyPost(post))
	}
	sortPosts(filtered, query.Sort)

	total := len(filtered)
	totalPages := (total + query.Limit - 1) / query.Limit
	start := (query.Page - 1) * query.Limit
	end := start + query.Limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	// Materialized (never a nil slice, even for an empty page) and copied
	// (never aliased to the store's backing array).
	items := make([]posts.Post, 0, end-start)
	items = append(items, filtered[start:end]...)
	return posts.PostPage{
		Items:      items,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalItems: total,
		TotalPages: totalPages,
		HasNext:    query.Page < totalPages,
		HasPrev:    query.Page > 1,
	}, nil
}

// GetByID implements the port: the stored copy, or the typed miss.
func (r *Repository) GetByID(_ context.Context, id string) (posts.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	post, ok := r.store[id]
	if !ok {
		return posts.Post{}, core.Newf(core.KindNotFound, "Post %q not found.", id)
	}
	return copyPost(post), nil
}

// Create implements the port: assign the next id, stamp both timestamps,
// store a private copy.
func (r *Repository) Create(_ context.Context, draft posts.Draft) (posts.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	post := posts.Post{
		ID:        fmt.Sprintf("p-%d", r.nextID),
		Title:     draft.Title,
		Body:      draft.Body,
		Tags:      append([]string(nil), draft.Tags...),
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.nextID++
	r.store[post.ID] = copyPost(post)
	return post, nil
}

// Replace implements the port: full replacement with CreatedAt preserved
// and UpdatedAt moved to now — the adapter owns the stamping contract the
// port documents.
func (r *Repository) Replace(_ context.Context, id string, draft posts.Draft) (posts.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.store[id]
	if !ok {
		return posts.Post{}, core.Newf(core.KindNotFound, "Post %q not found.", id)
	}
	replaced := posts.Post{
		ID:        existing.ID,
		Title:     draft.Title,
		Body:      draft.Body,
		Tags:      append([]string(nil), draft.Tags...),
		CreatedAt: existing.CreatedAt,
		UpdatedAt: r.now(),
	}
	r.store[id] = copyPost(replaced)
	return replaced, nil
}

// Delete implements the port under the strict stance: remove and answer
// nil, or report the typed miss — deleting a vanished post is a 404, not
// a silent success.
func (r *Repository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.store[id]; !ok {
		return core.Newf(core.KindNotFound, "Post %q not found.", id)
	}
	delete(r.store, id)
	return nil
}

// containsTag reports an exact membership test against the post's
// normalized tags.
func containsTag(tags []string, tag string) bool {
	for _, candidate := range tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

// sortPosts orders the slice per the sort vocabulary. Every comparator
// ends on the id so equal keys never leave the order to map iteration.
func sortPosts(slice []posts.Post, sort string) {
	switch sort {
	case posts.SortOldest:
		sortSlice(slice, func(a, b posts.Post) int {
			if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
				return c
			}
			return strings.Compare(a.ID, b.ID)
		})
	case posts.SortTitle:
		sortSlice(slice, func(a, b posts.Post) int {
			if c := strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)); c != 0 {
				return c
			}
			return strings.Compare(a.ID, b.ID)
		})
	default: // SortNewest — the API's default ordering.
		sortSlice(slice, func(a, b posts.Post) int {
			if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
				return c
			}
			return strings.Compare(b.ID, a.ID)
		})
	}
}

// sortSlice is sort.Slice with the argument order the comparator wants.
func sortSlice(slice []posts.Post, less func(a, b posts.Post) int) {
	sort.Slice(slice, func(i, j int) bool { return less(slice[i], slice[j]) < 0 })
}

// copyPost deep-copies tags so a returned post shares no mutable state
// with the store.
func copyPost(post posts.Post) posts.Post {
	post.Tags = append([]string(nil), post.Tags...)
	return post
}
