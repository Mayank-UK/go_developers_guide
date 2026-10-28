package posts

import (
	"context"
)

// The repository port — the seam this domain is consumed through and the
// only type in internal/data/posts is allowed to know about from here.
// Handlers and use cases depend on this interface, never on an
// implementation; the composition root (cmd/guide/main.go) wires the
// in-memory adapter today and the PostgreSQL adapter in the databases
// topic. A miss on an id is the typed core.NotFound error — ports speak
// the app's error taxonomy, not booleans that callers must interpret.

// List paging bounds: the defaults and the ceiling the list use case
// enforces before the repository is consulted.
const (
	DefaultPageLimit = 10
	MaxPageLimit     = 50
)

// Sort options for ListQuery — a closed vocabulary so clients branch on a
// known set and an unknown value is a 422 that can teach the list.
const (
	SortNewest = "newest" // CreatedAt descending (the default)
	SortOldest = "oldest" // CreatedAt ascending
	SortTitle  = "title"  // Title, case-insensitive; ID breaks ties
)

// SortOptions is the closed set, in wire order (default first).
func SortOptions() []string {
	return []string{SortNewest, SortOldest, SortTitle}
}

// SortValid reports whether s is one of the closed sort vocabulary.
func SortValid(s string) bool {
	switch s {
	case SortNewest, SortOldest, SortTitle:
		return true
	default:
		return false
	}
}

// ListQuery is a normalized, validated list request: page ≥ 1, limit
// 1–MaxPageLimit, an exact-match tag filter ("" means no filter), and a
// sort from the closed vocabulary. Build one through NormalizeListQuery,
// which is where those rules live.
type ListQuery struct {
	Page  int
	Limit int
	Tag   string
	Sort  string
}

// PostPage is the list response's domain shape: one page of items plus the
// honest metadata a client needs to render pagination — totals, the page
// count derived from them, and the neighbor flags. Items is never nil, so
// an empty page renders as [] on the wire. A page past the last is a valid
// empty page, not an error: the metadata says where the data ended.
type PostPage struct {
	Items      []Post
	Page       int
	Limit      int
	TotalItems int
	TotalPages int
	HasNext    bool
	HasPrev    bool
}

// PostsRepository is the storage port for the posts resource. All methods
// take ctx first (anything that can block takes context) and return typed
// core errors: core.NotFound for an unknown id, so one mapping at the
// handler produces the 404 envelope for every adapter.
type PostsRepository interface {
	// List returns one page of posts matching the query, sorted and
	// filtered by the repository, with the total match count for the page
	// metadata.
	List(ctx context.Context, query ListQuery) (PostPage, error)
	// GetByID returns one post or core.NotFound.
	GetByID(ctx context.Context, id string) (Post, error)
	// Create stores a draft and returns the stored post — the adapter
	// assigns the ID and stamps CreatedAt/UpdatedAt.
	Create(ctx context.Context, draft Draft) (Post, error)
	// Replace swaps a draft in for the post at id and returns the stored
	// result (CreatedAt preserved, UpdatedAt moved), or core.NotFound.
	Replace(ctx context.Context, id string, draft Draft) (Post, error)
	// Delete removes the post at id, or returns core.NotFound — the strict
	// stance this API teaches (the miss is the truth: the resource is not
	// there now).
	Delete(ctx context.Context, id string) error
}
