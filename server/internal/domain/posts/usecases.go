package posts

import (
	"context"
	"fmt"
	"strconv"

	"guide/internal/core"
)

// The use cases: one verb each, thin over the repository port, carrying
// every rule the verb owns. The HTTP handlers stay thin because THESE
// hold the behavior — normalize and validate the draft, translate a miss
// into the typed taxonomy, and hand the repository exactly one canonical
// operation. Constructed once at the composition root.

// NormalizeListQuery is the list endpoint's boundary rule: parse and judge
// the raw query-string values ("" means "not sent" and takes the default),
// collecting every issue the way draft validation does — one 422 names all
// broken parameters and the vocabulary it violated. Unknown query KEYS are
// the handler's business (ignored there, deliberately); unknown VALUES are
// judged here.
func NormalizeListQuery(page, limit, tag, sort string) (ListQuery, error) {
	var issues []Issue
	normalized := ListQuery{Sort: SortNewest}

	if page == "" {
		normalized.Page = 1
	} else if parsed, err := strconv.Atoi(page); err != nil || parsed < 1 {
		issues = append(issues, Issue{
			Field: "page",
			Rule:  "an integer ≥ 1 when sent; omit for the first page",
			Got:   strconv.Quote(page),
		})
	} else {
		normalized.Page = parsed
	}

	if limit == "" {
		normalized.Limit = DefaultPageLimit
	} else if parsed, err := strconv.Atoi(limit); err != nil || parsed < 1 || parsed > MaxPageLimit {
		issues = append(issues, Issue{
			Field: "limit",
			Rule:  fmt.Sprintf("an integer 1–%d when sent; omit for %d", MaxPageLimit, DefaultPageLimit),
			Got:   strconv.Quote(limit),
		})
	} else {
		normalized.Limit = parsed
	}

	// The tag filter is exact-match against normalized (lowercase) tags;
	// the handler trims it, so only emptiness means "no filter" here.
	normalized.Tag = tag

	if sort == "" {
		// SortNewest is already the default.
	} else if !SortValid(sort) {
		issues = append(issues, Issue{
			Field: "sort",
			Rule:  fmt.Sprintf("one of %q, %q, or %q; omit for newest first", SortNewest, SortOldest, SortTitle),
			Got:   strconv.Quote(sort),
		})
	} else {
		normalized.Sort = sort
	}

	if len(issues) > 0 {
		return ListQuery{}, core.ValidationFailed("Invalid post list query.", issues)
	}
	return normalized, nil
}

// ListPostsUseCase returns one honest page of posts for a normalized query.
type ListPostsUseCase struct {
	repository PostsRepository
}

// NewListPostsUseCase wires the storage port.
func NewListPostsUseCase(repository PostsRepository) *ListPostsUseCase {
	return &ListPostsUseCase{repository: repository}
}

// Execute lists through the port; the repository owns filtering, sorting,
// and the page math's raw material.
func (uc *ListPostsUseCase) Execute(ctx context.Context, query ListQuery) (PostPage, error) {
	return uc.repository.List(ctx, query)
}

// GetPostUseCase fetches one post by id.
type GetPostUseCase struct {
	repository PostsRepository
}

// NewGetPostUseCase wires the storage port.
func NewGetPostUseCase(repository PostsRepository) *GetPostUseCase {
	return &GetPostUseCase{repository: repository}
}

// Execute returns the post or the typed miss — the handler maps it to the
// 404 envelope without inspecting the repository's identity.
func (uc *GetPostUseCase) Execute(ctx context.Context, id string) (Post, error) {
	return uc.repository.GetByID(ctx, id)
}

// CreatePostUseCase validates and stores a new post.
type CreatePostUseCase struct {
	repository PostsRepository
}

// NewCreatePostUseCase wires the storage port.
func NewCreatePostUseCase(repository PostsRepository) *CreatePostUseCase {
	return &CreatePostUseCase{repository: repository}
}

// Execute normalizes the draft (so rules judge canonical text), validates
// it (collecting every issue into one 422), and stores it only when clean.
func (uc *CreatePostUseCase) Execute(ctx context.Context, draft Draft) (Post, error) {
	normalized := draft.NormalizeDraft()
	if issues := normalized.ValidateDraft(); len(issues) > 0 {
		return Post{}, core.ValidationFailed("Invalid post body.", issues)
	}
	return uc.repository.Create(ctx, normalized)
}

// ReplacePostUseCase fully replaces a post (PUT semantics: the draft is
// the whole next state, absent fields do not mean "keep the old value").
type ReplacePostUseCase struct {
	repository PostsRepository
}

// NewReplacePostUseCase wires the storage port.
func NewReplacePostUseCase(repository PostsRepository) *ReplacePostUseCase {
	return &ReplacePostUseCase{repository: repository}
}

// Execute validates the replacement draft exactly as create does — one
// rule set, two verbs — then swaps it in; a miss is the typed NotFound.
func (uc *ReplacePostUseCase) Execute(ctx context.Context, id string, draft Draft) (Post, error) {
	normalized := draft.NormalizeDraft()
	if issues := normalized.ValidateDraft(); len(issues) > 0 {
		return Post{}, core.ValidationFailed("Invalid post body.", issues)
	}
	return uc.repository.Replace(ctx, id, normalized)
}

// DeletePostUseCase removes a post under the strict stance: a miss is a
// typed NotFound (404), not a silent 204.
type DeletePostUseCase struct {
	repository PostsRepository
}

// NewDeletePostUseCase wires the storage port.
func NewDeletePostUseCase(repository PostsRepository) *DeletePostUseCase {
	return &DeletePostUseCase{repository: repository}
}

// Execute deletes or reports the miss.
func (uc *DeletePostUseCase) Execute(ctx context.Context, id string) error {
	return uc.repository.Delete(ctx, id)
}
