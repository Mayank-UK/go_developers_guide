// Topic 11 api-design's live demo: the posts workbench companion payload —
// the API surface as DATA. The workbench view drives the real /api/posts
// CRUD routes; this payload is the static contract card that sits beside
// it: every route with its success status and its error cases, the
// pagination rules, the filtering semantics, the versioning stance, the
// delete semantics decision, and the error-envelope anatomy (with RFC 9457
// held up for comparison). Deterministic by construction — no clock, no
// randomness, the same bytes on every call.
package demos

import "guide/internal/domain/posts"

// WorkbenchResource names the resource the whole surface serves.
type WorkbenchResource struct {
	Name           string `json:"name"`
	CollectionPath string `json:"collectionPath"`
	ItemPath       string `json:"itemPath"`
	WireShape      string `json:"wireShape"`
}

// WorkbenchOutcome is a route's success contract: the status plus what
// rides with it.
type WorkbenchOutcome struct {
	Status int    `json:"status"`
	Note   string `json:"note"`
}

// WorkbenchErrorContract is one error case a route can answer: the status,
// the envelope's code bucket, and when it fires.
type WorkbenchErrorContract struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	When   string `json:"when"`
}

// WorkbenchRoute is one row of the route table: method, path, purpose, the
// success outcome, and every error envelope the route can answer.
type WorkbenchRoute struct {
	ID      string                   `json:"id"`
	Method  string                   `json:"method"`
	Path    string                   `json:"path"`
	Purpose string                   `json:"purpose"`
	Success WorkbenchOutcome         `json:"success"`
	Errors  []WorkbenchErrorContract `json:"errors"`
}

// WorkbenchPagination is the list route's paging contract: defaults,
// bounds, the sort vocabulary, the envelope's metadata fields, and the
// empty-page rule.
type WorkbenchPagination struct {
	DefaultLimit   int      `json:"defaultLimit"`
	MaxLimit       int      `json:"maxLimit"`
	DefaultPage    int      `json:"defaultPage"`
	DefaultSort    string   `json:"defaultSort"`
	SortOptions    []string `json:"sortOptions"`
	EnvelopeFields []string `json:"envelopeFields"`
	EmptyPageRule  string   `json:"emptyPageRule"`
}

// WorkbenchFiltering states the filter semantics: exact match against
// write-normalized tags.
type WorkbenchFiltering struct {
	Param     string `json:"param"`
	Semantics string `json:"semantics"`
	Example   string `json:"example"`
}

// WorkbenchVersioning is this API's versioning stance and the moment it
// changes.
type WorkbenchVersioning struct {
	Prefix         string `json:"prefix"`
	Strategy       string `json:"strategy"`
	ContentExample string `json:"contentExample"`
	WhenToVersion  string `json:"whenToVersion"`
}

// WorkbenchDeleteSemantics records the chosen DELETE stance — the decision
// the topic teaches rather than hides.
type WorkbenchDeleteSemantics struct {
	OnMiss      int    `json:"onMiss"`
	Chosen      string `json:"chosen"`
	Rationale   string `json:"rationale"`
	Alternative string `json:"alternative"`
}

// WorkbenchEnvelopeMember is one member of the error envelope with its
// meaning.
type WorkbenchEnvelopeMember struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
}

// WorkbenchEnvelope is the RECORDED envelope's anatomy plus the RFC 9457
// comparison the topic argues.
type WorkbenchEnvelope struct {
	WireShape      string                    `json:"wireShape"`
	Members        []WorkbenchEnvelopeMember `json:"members"`
	Rfc9457Verdict string                    `json:"rfc9457Verdict"`
}

// PostsWorkbenchPayload is the whole GET /api/demo/api/posts-workbench
// body. Every collection is non-nil so the wire renders [] — never null.
type PostsWorkbenchPayload struct {
	Resource        WorkbenchResource        `json:"resource"`
	Routes          []WorkbenchRoute         `json:"routes"`
	Pagination      WorkbenchPagination      `json:"pagination"`
	Filtering       WorkbenchFiltering       `json:"filtering"`
	Versioning      WorkbenchVersioning      `json:"versioning"`
	DeleteSemantics WorkbenchDeleteSemantics `json:"deleteSemantics"`
	Envelope        WorkbenchEnvelope        `json:"envelope"`
}

// BuildPostsWorkbenchPayload assembles the contract card from the real
// domain vocabulary — sort options and paging bounds come from the posts
// package itself, so this payload cannot drift from the enforced rules.
func BuildPostsWorkbenchPayload() PostsWorkbenchPayload {
	return PostsWorkbenchPayload{
		Resource: WorkbenchResource{
			Name:           "posts",
			CollectionPath: "/api/posts",
			ItemPath:       "/api/posts/{id}",
			WireShape:      `{"id","title","body","tags","createdAt","updatedAt"}`,
		},
		Routes: []WorkbenchRoute{
			{
				ID: "list", Method: "GET", Path: "/api/posts",
				Purpose: "one page of the collection — filter, sort, paginate",
				Success: WorkbenchOutcome{Status: 200, Note: "page envelope: items + page/limit/totalItems/totalPages/hasNextPage/hasPrevPage"},
				Errors: []WorkbenchErrorContract{
					{Status: 422, Code: "validation_failed", When: "page, limit, or sort outside the rules (details name every broken parameter)"},
				},
			},
			{
				ID: "create", Method: "POST", Path: "/api/posts",
				Purpose: "create a post from the full draft body",
				Success: WorkbenchOutcome{Status: 201, Note: "the stored post + Location: /api/posts/{id}"},
				Errors: []WorkbenchErrorContract{
					{Status: 422, Code: "validation_failed", When: "body is not one strict JSON object, or a field breaks a rule (details carry the issue list)"},
				},
			},
			{
				ID: "read", Method: "GET", Path: "/api/posts/{id}",
				Purpose: "read one post by id",
				Success: WorkbenchOutcome{Status: 200, Note: "the post"},
				Errors: []WorkbenchErrorContract{
					{Status: 404, Code: "not_found", When: "no post with that id exists"},
				},
			},
			{
				ID: "replace", Method: "PUT", Path: "/api/posts/{id}",
				Purpose: "full replacement — the body is the whole next state",
				Success: WorkbenchOutcome{Status: 200, Note: "the stored post (createdAt preserved, updatedAt moved)"},
				Errors: []WorkbenchErrorContract{
					{Status: 404, Code: "not_found", When: "no post with that id exists"},
					{Status: 422, Code: "validation_failed", When: "the replacement draft breaks a field rule"},
				},
			},
			{
				ID: "delete", Method: "DELETE", Path: "/api/posts/{id}",
				Purpose: "remove a post under the strict stance",
				Success: WorkbenchOutcome{Status: 204, Note: "no body — nothing to say"},
				Errors: []WorkbenchErrorContract{
					{Status: 404, Code: "not_found", When: "already gone: the miss is the truth, not a silent 204"},
				},
			},
		},
		Pagination: WorkbenchPagination{
			DefaultLimit:   posts.DefaultPageLimit,
			MaxLimit:       posts.MaxPageLimit,
			DefaultPage:    1,
			DefaultSort:    posts.SortNewest,
			SortOptions:    posts.SortOptions(),
			EnvelopeFields: []string{"items", "page", "limit", "totalItems", "totalPages", "hasNextPage", "hasPrevPage"},
			EmptyPageRule:  "a page past the end is a valid empty page (items: []) with honest metadata — totals say where the data ended",
		},
		Filtering: WorkbenchFiltering{
			Param:     "tag",
			Semantics: "exact match against the post's write-normalized tags (trimmed, lowercased on the way in)",
			Example:   "/api/posts?tag=go&limit=3",
		},
		Versioning: WorkbenchVersioning{
			Prefix:         "/api",
			Strategy:       "unversioned prefix; version the contract the day a breaking change ships (path major: /api/v2/posts)",
			ContentExample: "the content endpoints carry a version field (\"version\": 1) so clients detect a wire-shape bump on any response",
			WhenToVersion:  "the moment you cannot change a response without breaking a client you do not control — before that moment, versioning is speculative branching",
		},
		DeleteSemantics: WorkbenchDeleteSemantics{
			OnMiss:      404,
			Chosen:      "strict — DELETE of a vanished post answers 404",
			Rationale:   "the response reports the world: the resource is not there now; silent 204s swallow client bugs and stale UIs",
			Alternative: "idempotent 204-on-any-miss trades honesty for retry-friendliness — defensible, but then a mistyped id deletes nothing and says nothing",
		},
		Envelope: WorkbenchEnvelope{
			WireShape: `{"error":{"code","message","requestId","details?"}}`,
			Members: []WorkbenchEnvelopeMember{
				{Name: "code", Meaning: "the taxonomy bucket — a closed set (not_found, validation_failed, …) clients branch on"},
				{Name: "message", Meaning: "one human sentence, safe to show any user; internals ride the logs, never the body"},
				{Name: "requestId", Meaning: "the join key: matches the x-request-id header and the server's warn-level log line"},
				{Name: "details", Meaning: "structured context (which fields, what was expected) — omitted when absent, never null"},
			},
			Rfc9457Verdict: "RFC 9457 problem+json shares the shape's bones (a machine-readable type, a human summary, the status) and the recorded envelope matches it in spirit: code plays type, message plays title, details carries extensions — the deliberate differences are the requestId join key and one envelope for every error, not a per-problem document",
		},
	}
}
