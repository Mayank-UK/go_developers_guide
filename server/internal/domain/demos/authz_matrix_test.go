package demos

import (
	"context"
	"strings"
	"testing"
)

// A fixture run shaped exactly like the data console's haul — the
// builder is pure, so its claims are proven against hand-checked rows.
func authzMatrixFixtureRun() AuthzMatrixRun {
	return AuthzMatrixRun{
		Roles: []AuthzRoleGrantRow{
			{Role: "reader", Permissions: []string{"posts:read"}},
			{Role: "author", Permissions: []string{"posts:read", "posts:write"}},
			{Role: "editor", Permissions: []string{"posts:read", "posts:write"}},
			{Role: "admin", Permissions: []string{"posts:read", "posts:write", "users:manage"}},
		},
		Subjects: []AuthzSubjectRow{
			{Username: "anonymous", DisplayName: "Anonymous", Via: "none", Roles: []string{}, Scopes: []string{}, Note: "no identity at all"},
			{Username: "linus", DisplayName: "Linus", Via: "session", Roles: []string{"reader"}, Scopes: []string{"posts:read"}, Note: "the read-only user"},
			{Username: "ada", DisplayName: "Ada", Via: "session", Roles: []string{"author"}, Scopes: []string{"posts:read", "posts:write"}, Note: "the author"},
			{Username: "kim", DisplayName: "Kim", Via: "api-key", Roles: []string{"author"}, Scopes: []string{"posts:read"}, Note: "author with a narrowed credential"},
			{Username: "grace", DisplayName: "Grace", Via: "jwt", Roles: []string{"editor"}, Scopes: []string{"posts:read", "posts:write"}, Note: "the editor"},
			{Username: "root", DisplayName: "Root", Via: "jwt", Roles: []string{"admin"}, Scopes: []string{"*"}, Note: "the admin"},
			{Username: "guest", DisplayName: "Guest", Via: "session", Roles: []string{}, Scopes: []string{"*"}, Note: "deny-by-default control"},
		},
		Posts: []AuthzPostRow{
			{ID: "p-sessions", Title: "Sealed cookie sessions", Owner: "ada"},
			{ID: "p-draft", Title: "Editing someone else's draft", Owner: "grace"},
			{ID: "p-narrowed", Title: "A read-only credential", Owner: "kim"},
		},
		Cells: []AuthzCellRow{
			// reads: everyone capability-holding allows; anonymous and guest deny.
			{Subject: "linus", Action: "posts:read", PostID: "p-draft", Allowed: true, Reason: "collection-level"},
			{Subject: "anonymous", Action: "posts:read", PostID: "p-draft", Allowed: false, Reason: "not authenticated"},
			{Subject: "guest", Action: "posts:read", PostID: "p-sessions", Allowed: false, Reason: "no role"},
			// writes on ada's post: owner, editor, admin allow; reader, narrowed author, guest, anonymous deny.
			{Subject: "ada", Action: "posts:write", PostID: "p-sessions", Allowed: true, Reason: "owner"},
			{Subject: "grace", Action: "posts:write", PostID: "p-sessions", Allowed: true, Reason: "editor spans every post"},
			{Subject: "ada", Action: "posts:write", PostID: "p-draft", Allowed: false, Reason: "object-level refusal"},
			{Subject: "kim", Action: "posts:write", PostID: "p-sessions", Allowed: false, Reason: "narrowed"},
			{Subject: "kim", Action: "posts:write", PostID: "p-narrowed", Allowed: false, Reason: "narrowed"},
			{Subject: "root", Action: "posts:write", PostID: "p-sessions", Allowed: true, Reason: "admin override"},
			{Subject: "guest", Action: "posts:write", PostID: "p-sessions", Allowed: false, Reason: "no role"},
			{Subject: "anonymous", Action: "posts:write", PostID: "p-sessions", Allowed: false, Reason: "not authenticated"},
			// the namespace island.
			{Subject: "root", Action: "users:manage", PostID: "-", Allowed: true, Reason: "admin grant"},
			{Subject: "ada", Action: "users:manage", PostID: "-", Allowed: false, Reason: "no role grants"},
		},
	}
}

// The builder must derive every teaching claim from the cells — pinned
// by asserting the load-bearing verdicts and proofs against the fixture.
func TestAuthzMatrixBuilderDerivesTeachingBeatsFromCells(t *testing.T) {
	payload := BuildAuthzMatrixPayload(authzMatrixFixtureRun())
	if payload.Mechanism != "rbac-matrix" {
		t.Errorf("mechanism = %q", payload.Mechanism)
	}
	if len(payload.Beats) != 8 {
		t.Fatalf("beats = %d, want 8", len(payload.Beats))
	}
	byBeat := map[string]AuthStoryBeat{}
	for _, beat := range payload.Beats {
		byBeat[beat.Beat] = beat
	}
	ownerBeat := byBeat["owner, editor, stranger"]
	if ownerBeat.Verdict != AuthVerdictOK {
		t.Errorf("owner/editor/stranger beat verdict = %q, want ok", ownerBeat.Verdict)
	}
	for _, want := range []string{
		"ada posts:write p-sessions: allow",
		"grace posts:write p-sessions: allow",
		"ada posts:write p-draft: deny",
	} {
		if !strings.Contains(ownerBeat.Detail, want) {
			t.Errorf("owner beat detail lacks %q: %s", want, ownerBeat.Detail)
		}
	}
	scopes := byBeat["scopes narrow roles"]
	if !strings.Contains(scopes.Detail, "kim posts:write p-narrowed: deny") {
		t.Errorf("scopes beat must show kim denied on kim's own post: %s", scopes.Detail)
	}
	deny := byBeat["deny by default"]
	if !strings.Contains(deny.Detail, "guest posts:write p-sessions: deny") {
		t.Errorf("deny beat must show the role-less control row: %s", deny.Detail)
	}
	namespaces := byBeat["namespaces isolate"]
	if !strings.Contains(namespaces.Detail, "root users:manage -: allow") ||
		!strings.Contains(namespaces.Detail, "ada users:manage -: deny") {
		t.Errorf("namespace beat must show both island verdicts: %s", namespaces.Detail)
	}
	reads := byBeat["read is collection-level"]
	if !strings.Contains(reads.Detail, "linus posts:read p-draft: allow") {
		t.Errorf("read beat must show the reader's allow: %s", reads.Detail)
	}
	if reads.Verdict != AuthVerdictOK {
		t.Errorf("read beat verdict = %q, want ok", reads.Verdict)
	}
}

func TestAuthzMatrixBuilderProofsAndReasoningCarryTheNumbers(t *testing.T) {
	payload := BuildAuthzMatrixPayload(authzMatrixFixtureRun())
	joined := ""
	for _, proof := range payload.Proofs {
		joined += proof.Label + "=" + proof.Value + "\n"
	}
	// The fixture has 8 write cells, 3 allowed.
	for _, want := range []string{"matrix size=13", "write column=3 of 8", "deny by default"} {
		if !strings.Contains(joined, want) {
			t.Errorf("proofs lack %q:\n%s", want, joined)
		}
	}
	if !strings.Contains(payload.Reasoning, "scope") || !strings.Contains(payload.Reasoning, "ownership") {
		t.Error("reasoning must name the gates")
	}
}

// A contradicted run must SURFACE as a red beat, not a green lie: flip
// the owner cell and the owner/editor/stranger beat must go rejected.
func TestAuthzMatrixBuilderTurnsRedWhenCellsContradictTheClaim(t *testing.T) {
	run := authzMatrixFixtureRun()
	for i := range run.Cells {
		if run.Cells[i].Subject == "ada" && run.Cells[i].Action == "posts:write" && run.Cells[i].PostID == "p-draft" {
			run.Cells[i].Allowed = true // drifted: the stranger was let in
		}
	}
	payload := BuildAuthzMatrixPayload(run)
	for _, beat := range payload.Beats {
		if beat.Beat == "owner, editor, stranger" && beat.Verdict != AuthVerdictRejected {
			t.Fatalf("contradicted claim must render rejected, got %q", beat.Verdict)
		}
	}
}

// A missing cell renders as NOT EVALUATED rather than a fabricated verdict.
func TestAuthzMatrixBuilderFlagsMissingCellsHonestly(t *testing.T) {
	run := authzMatrixFixtureRun()
	run.Cells = nil
	payload := BuildAuthzMatrixPayload(run)
	if payload.Proofs == nil {
		t.Fatal("proofs must never be nil")
	}
	found := false
	for _, beat := range payload.Beats {
		if strings.Contains(beat.Detail, "NOT EVALUATED") {
			found = true
		}
	}
	if !found {
		t.Error("empty run must surface NOT EVALUATED in its beats")
	}
	for _, beat := range payload.Beats {
		if beat.Facts == nil {
			t.Errorf("beat %q facts must never be nil", beat.Beat)
		}
	}
}

// The runner port contract: the data console implements it — pinned
// here by shape, exercised for real in internal/data/authorization.
func TestAuthzMatrixRunnerIsALocalPort(t *testing.T) {
	var _ AuthzMatrixRunner = authzFixedMatrixRunner{}
}

type authzFixedMatrixRunner struct{}

func (authzFixedMatrixRunner) RunAuthzMatrixStory(context.Context) (AuthzMatrixRun, error) {
	return authzMatrixFixtureRun(), nil
}
