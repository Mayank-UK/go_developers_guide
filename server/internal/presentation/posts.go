package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// Posts API handlers — the topic 11 reference for THIN handlers over use
// cases: parse and shape-check the input, call exactly one use case, map
// the result onto a status and body. Every rule (bounds, vocabularies,
// normalization) lives in the domain; every failure travels the central
// error writer as the one envelope; the handlers know no storage exists.
//
// The five routes these types serve (wired in cmd/guide/main.go):
//
//	GET    /api/posts       list (pagination + filtering + sort)
//	POST   /api/posts       create           → 201 + Location
//	GET    /api/posts/{id}  read one         → 200 | 404 envelope
//	PUT    /api/posts/{id}  full replace     → 200 | 404/422
//	DELETE /api/posts/{id}  delete (strict)  → 204 | 404 envelope

// maxPostBodyBytes bounds create/replace bodies: 5000 runes of body text
// plus JSON structure fits far under 64 KiB, so anything larger is broken
// or hostile — refusal, not memory pressure.
const maxPostBodyBytes = 64 << 10

// postResponse is the wire shape of one post: camelCase, tags never null
// (the slice is materialized even when empty), timestamps as RFC 3339
// (time.Time's own JSON encoding).
type postResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// newPostResponse shapes a domain post for the wire; the only translation
// is materializing tags as [] and letting the JSON tags rename fields.
func newPostResponse(post posts.Post) postResponse {
	return postResponse{
		ID:        post.ID,
		Title:     post.Title,
		Body:      post.Body,
		Tags:      append([]string(nil), post.Tags...),
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

// postPageResponse is the list envelope: the page's items plus the honest
// pagination metadata the client renders navigation from.
type postPageResponse struct {
	Items       []postResponse `json:"items"`
	Page        int            `json:"page"`
	Limit       int            `json:"limit"`
	TotalItems  int            `json:"totalItems"`
	TotalPages  int            `json:"totalPages"`
	HasNextPage bool           `json:"hasNextPage"`
	HasPrevPage bool           `json:"hasPrevPage"`
}

// newPostPageResponse shapes a domain page for the wire, materializing
// items as [] for an empty page.
func newPostPageResponse(page posts.PostPage) postPageResponse {
	items := make([]postResponse, 0, len(page.Items))
	for _, post := range page.Items {
		items = append(items, newPostResponse(post))
	}
	return postPageResponse{
		Items:       items,
		Page:        page.Page,
		Limit:       page.Limit,
		TotalItems:  page.TotalItems,
		TotalPages:  page.TotalPages,
		HasNextPage: page.HasNext,
		HasPrevPage: page.HasPrev,
	}
}

// postDraftRequest is the create/replace body: exactly the draft's three
// fields, nothing else (the decoder refuses unknown keys).
type postDraftRequest struct {
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

// decodePostDraft is the boundary discipline from the validation topic,
// applied to the posts bodies: bound the read, exactly one JSON value,
// refuse null, then strict-decode with unknown fields refused — every
// refusal is the 422 envelope with the decoder's own reason in
// details.body.
func decodePostDraft(w http.ResponseWriter, r *http.Request) (posts.Draft, error) {
	invalid := func(reason string) (posts.Draft, error) {
		return posts.Draft{}, core.ValidationFailed(
			"Invalid post body.", map[string]string{"body": reason},
		)
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPostBodyBytes))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return invalid(err.Error())
	}
	// Exactly one JSON value: a second Decode must hit EOF, otherwise the
	// body is not one post document.
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return invalid("body must contain exactly one JSON object")
	}
	// JSON null decodes into any struct silently; a post draft must be an
	// object.
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return invalid("body must be a JSON object, not null")
	}

	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	var request postDraftRequest
	if err := strict.Decode(&request); err != nil {
		return invalid(err.Error())
	}
	return posts.Draft{Title: request.Title, Body: request.Body, Tags: request.Tags}, nil
}

// Compile-time proofs that every posts handler satisfies the interface
// route registration requires.
var (
	_ http.Handler = (*PostsListHandler)(nil)
	_ http.Handler = (*PostsCreateHandler)(nil)
	_ http.Handler = (*PostDetailHandler)(nil)
	_ http.Handler = (*PostReplaceHandler)(nil)
	_ http.Handler = (*PostDeleteHandler)(nil)
)

// PostsListHandler serves GET /api/posts: query-string values go through
// the list use case's normalizer (defaults, bounds, the sort vocabulary —
// every violation one 422), unknown query KEYS are ignored on purpose
// (intermediaries append parameters; bodies are strict, URLs are not).
type PostsListHandler struct {
	list   *posts.ListPostsUseCase
	logger *slog.Logger
}

// NewPostsListHandler wires the list use case and the process logger.
func NewPostsListHandler(list *posts.ListPostsUseCase, logger *slog.Logger) *PostsListHandler {
	return &PostsListHandler{list: list, logger: logger}
}

// ServeHTTP implements http.Handler for the posts collection listing.
func (h *PostsListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	normalized, err := posts.NormalizeListQuery(
		query.Get("page"),
		query.Get("limit"),
		strings.TrimSpace(query.Get("tag")),
		query.Get("sort"),
	)
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	page, err := h.list.Execute(r.Context(), normalized)
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, newPostPageResponse(page))
}

// PostsCreateHandler serves POST /api/posts: strict decode → create use
// case → 201 with the stored post and a Location header naming it.
type PostsCreateHandler struct {
	create *posts.CreatePostUseCase
	logger *slog.Logger
}

// NewPostsCreateHandler wires the create use case and the process logger.
func NewPostsCreateHandler(create *posts.CreatePostUseCase, logger *slog.Logger) *PostsCreateHandler {
	return &PostsCreateHandler{create: create, logger: logger}
}

// ServeHTTP implements http.Handler for post creation.
func (h *PostsCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	draft, err := decodePostDraft(w, r)
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	post, err := h.create.Execute(r.Context(), draft)
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	w.Header().Set("Location", "/api/posts/"+post.ID)
	writeJSON(w, http.StatusCreated, newPostResponse(post))
}

// PostDetailHandler serves GET /api/posts/{id}: one use case call; a miss
// arrives as the typed taxonomy error and leaves as the 404 envelope.
type PostDetailHandler struct {
	get    *posts.GetPostUseCase
	logger *slog.Logger
}

// NewPostDetailHandler wires the get use case and the process logger.
func NewPostDetailHandler(get *posts.GetPostUseCase, logger *slog.Logger) *PostDetailHandler {
	return &PostDetailHandler{get: get, logger: logger}
}

// ServeHTTP implements http.Handler for reading one post.
func (h *PostDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	post, err := h.get.Execute(r.Context(), r.PathValue("id"))
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, newPostResponse(post))
}

// PostReplaceHandler serves PUT /api/posts/{id}: full replacement — the
// body is the whole next state, validated by the same rules create uses;
// a miss is 404, a rule violation 422, success 200 with the stored post.
type PostReplaceHandler struct {
	replace *posts.ReplacePostUseCase
	logger  *slog.Logger
}

// NewPostReplaceHandler wires the replace use case and the process logger.
func NewPostReplaceHandler(replace *posts.ReplacePostUseCase, logger *slog.Logger) *PostReplaceHandler {
	return &PostReplaceHandler{replace: replace, logger: logger}
}

// ServeHTTP implements http.Handler for full post replacement.
func (h *PostReplaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	draft, err := decodePostDraft(w, r)
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	post, err := h.replace.Execute(r.Context(), r.PathValue("id"), draft)
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, newPostResponse(post))
}

// PostDeleteHandler serves DELETE /api/posts/{id} under the strict stance:
// 204 (empty body) on removal, the 404 envelope on a miss — deleting a
// vanished post is the truth, not a silent success.
type PostDeleteHandler struct {
	delete *posts.DeletePostUseCase
	logger *slog.Logger
}

// NewPostDeleteHandler wires the delete use case and the process logger.
func NewPostDeleteHandler(delete *posts.DeletePostUseCase, logger *slog.Logger) *PostDeleteHandler {
	return &PostDeleteHandler{delete: delete, logger: logger}
}

// ServeHTTP implements http.Handler for post deletion.
func (h *PostDeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.delete.Execute(r.Context(), r.PathValue("id")); err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
