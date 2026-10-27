package authorization

import (
	"context"
	"strings"
	"testing"
	"time"

	dataauth "guide/internal/data/auth"
	domauth "guide/internal/domain/auth"
	domauthz "guide/internal/domain/authorization"
	"guide/internal/domain/demos"
)

// The ports first: assignments answer, unassigned answers EMPTY (never
// an error), owners answer or not-found.
func TestAuthorizationConsolePortsAnswerTheSeeds(t *testing.T) {
	console := NewConsole()
	ctx := context.Background()

	for username, want := range map[string][]domauthz.Role{
		"ada":   {domauthz.RoleAuthor},
		"linus": {domauthz.RoleReader},
		"grace": {domauthz.RoleEditor},
		"root":  {domauthz.RoleAdmin},
		"kim":   {domauthz.RoleAuthor},
	} {
		got, err := console.RolesFor(ctx, username)
		if err != nil {
			t.Fatalf("RolesFor(%s): %v", username, err)
		}
		if len(got) != len(want) || got[0] != want[0] {
			t.Errorf("RolesFor(%s) = %v, want %v", username, got, want)
		}
	}
	// guest is the deny-by-default control: present in the story's
	// subject list but ABSENT from the directory.
	guest, err := console.RolesFor(ctx, "guest")
	if err != nil {
		t.Fatalf("RolesFor(guest): %v", err)
	}
	if len(guest) != 0 {
		t.Errorf("unassigned user must answer empty, got %v", guest)
	}

	owner, found, err := console.PostOwner(ctx, "p-draft")
	if err != nil || !found || owner != "grace" {
		t.Fatalf("PostOwner(p-draft) = %q %t %v, want grace true", owner, found, err)
	}
	if _, found, _ := console.PostOwner(ctx, "p-missing"); found {
		t.Error("PostOwner must not find an unknown id")
	}
}

// The story's shape: every subject × action × post evaluated, plus one
// namespace-island cell per subject — and the load-bearing verdicts
// match the evaluator's own table.
func TestAuthorizationConsoleMatrixStoryVerdicts(t *testing.T) {
	console := NewConsole()
	run, err := console.RunAuthzMatrixStory(context.Background())
	if err != nil {
		t.Fatalf("RunAuthzMatrixStory: %v", err)
	}
	wantCells := len(seededSubjects) * (2*len(seededPosts) + 1)
	if len(run.Cells) != wantCells {
		t.Fatalf("cells = %d, want %d", len(run.Cells), wantCells)
	}
	if len(run.Subjects) != 7 || len(run.Posts) != 3 || len(run.Roles) != 4 {
		t.Fatalf("roster shapes: %d subjects %d posts %d roles", len(run.Subjects), len(run.Posts), len(run.Roles))
	}

	want := []struct {
		subject string
		action  string
		postID  string
		allowed bool
	}{
		{"ada", "posts:write", "p-sessions", true},    // owner
		{"grace", "posts:write", "p-sessions", true},  // editor spans posts
		{"ada", "posts:write", "p-draft", false},      // stranger
		{"kim", "posts:write", "p-sessions", false},   // narrowed credential
		{"kim", "posts:write", "p-narrowed", false},   // narrowed even on own post
		{"linus", "posts:read", "p-draft", true},      // collection-level read
		{"linus", "posts:write", "p-sessions", false}, // reader capability
		{"root", "posts:write", "p-draft", true},      // admin override
		{"root", "users:manage", "-", true},           // island granted
		{"ada", "users:manage", "-", false},           // island isolated
		{"guest", "posts:read", "p-sessions", false},  // no role
		{"anonymous", "posts:read", "p-sessions", false},
	}
	cells := map[string]demos.AuthzCellRow{}
	for _, cell := range run.Cells {
		cells[cell.Subject+" "+cell.Action+" "+cell.PostID] = cell
	}
	for _, tc := range want {
		cell, ok := cells[tc.subject+" "+tc.action+" "+tc.postID]
		if !ok {
			t.Fatalf("cell %s %s %s missing", tc.subject, tc.action, tc.postID)
		}
		if cell.Allowed != tc.allowed {
			t.Errorf("%s %s %s = %t (%s), want %t", tc.subject, tc.action, tc.postID, cell.Allowed, cell.Reason, tc.allowed)
		}
		if strings.TrimSpace(cell.Reason) == "" {
			t.Errorf("cell %s %s %s carries no reason", tc.subject, tc.action, tc.postID)
		}
	}
	// The deny-by-default controls deny EVERY cell they appear in.
	for _, control := range []string{"guest", "anonymous"} {
		for _, cell := range run.Cells {
			if cell.Subject == control && cell.Allowed {
				t.Fatalf("%s must be denied everywhere, allowed %s %s", control, cell.Action, cell.PostID)
			}
		}
	}
}

// The builder over the REAL run: all beats green (the run agrees with
// the teaching claims), proofs derived, nothing nil.
func TestAuthorizationConsoleMatrixStoryBuildsGreen(t *testing.T) {
	console := NewConsole()
	run, err := console.RunAuthzMatrixStory(context.Background())
	if err != nil {
		t.Fatalf("RunAuthzMatrixStory: %v", err)
	}
	payload := demos.BuildAuthzMatrixPayload(run)
	if payload.Mechanism != "rbac-matrix" {
		t.Errorf("mechanism = %q", payload.Mechanism)
	}
	// Claim-derived beats must be GREEN (the real run agrees with every
	// teaching claim); the deny-by-default beat is deliberately red —
	// it teaches the refusals — and must cite real deny verdicts.
	claimBeats := map[string]bool{
		"read is collection-level": true, "owner, editor, stranger": true,
		"scopes narrow roles": true, "namespaces isolate": true,
	}
	seen := map[string]string{}
	for _, beat := range payload.Beats {
		seen[beat.Beat] = beat.Verdict
		if beat.Facts == nil {
			t.Errorf("beat %q facts nil", beat.Beat)
		}
	}
	for beat := range claimBeats {
		if verdict, ok := seen[beat]; !ok || verdict != demos.AuthVerdictOK {
			t.Errorf("claim beat %q verdict = %q, want ok", beat, seen[beat])
		}
	}
	if verdict := seen["deny by default"]; verdict != demos.AuthVerdictRejected {
		t.Errorf("deny-by-default beat verdict = %q, want rejected", verdict)
	}
	if payload.Proofs == nil {
		t.Error("proofs must never be nil")
	}
}

// The end-to-end seam with topic 23: a REAL login of the auth
// console's ada resolves through the real resolver, and the identity
// the middleware would place in the context meets the SAME verdicts
// the matrix claims — ada writes her own post, linus does not write,
// and the scopes the resolver answers are the console's own seeds.
func TestAuthorizationConsoleRealAuthIdentityMeetsTheMatrix(t *testing.T) {
	authConsole, err := dataauth.NewConsole()
	if err != nil {
		t.Fatalf("auth console: %v", err)
	}
	console := NewConsole()
	ctx := context.Background()
	now := time.Now().UTC()
	hasher := domauth.NewHasher()

	minted, _, err := authConsole.Tokens.Login(ctx, authConsole.Directory, hasher, "ada", "correct-horse", now)
	if err != nil {
		t.Fatalf("ada login: %v", err)
	}
	identity, err := authConsole.Resolver.ResolveBearerToken(ctx, minted.Token, now)
	if err != nil {
		t.Fatalf("resolve ada's token: %v", err)
	}
	roles, err := console.RolesFor(ctx, identity.Username)
	if err != nil {
		t.Fatalf("roles for %s: %v", identity.Username, err)
	}
	adaOwned, _, _ := console.PostOwner(ctx, "p-sessions")
	decision := domauthz.DecidePost(identity, roles, domauthz.PermPostsWrite,
		domauthz.NewOwnedPost("p-sessions", "t", "b", adaOwned, nil, now))
	if !decision.Allowed {
		t.Fatalf("the real ada must write her own post: %s", decision)
	}
	graceOwned, _, _ := console.PostOwner(ctx, "p-draft")
	decision = domauthz.DecidePost(identity, roles, domauthz.PermPostsWrite,
		domauthz.NewOwnedPost("p-draft", "t", "b", graceOwned, nil, now))
	if decision.Allowed || !strings.Contains(decision.Reason, "object-level refusal") {
		t.Fatalf("the real ada must be refused as a stranger on grace's post: %s", decision)
	}

	linusMinted, _, err := authConsole.Tokens.Login(ctx, authConsole.Directory, hasher, "linus", "tr0ub4dor", now)
	if err != nil {
		t.Fatalf("linus login: %v", err)
	}
	linus, err := authConsole.Resolver.ResolveBearerToken(ctx, linusMinted.Token, now)
	if err != nil {
		t.Fatalf("resolve linus's token: %v", err)
	}
	linusRoles, _ := console.RolesFor(ctx, linus.Username)
	decision = domauthz.DecidePost(linus, linusRoles, domauthz.PermPostsWrite,
		domauthz.NewOwnedPost("p-sessions", "t", "b", adaOwned, nil, now))
	if decision.Allowed {
		t.Fatal("the real linus (reader, posts:read) must not write")
	}
}
