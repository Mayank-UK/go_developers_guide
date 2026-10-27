// Topic 24 authorization's demo `authz:matrix`: one STORY payload in
// the shape the authentication demos established (beats, proofs,
// reasoning) — the honest way to serve a decision matrix, because every
// sentence below is DERIVED from cells the runner actually evaluated
// against the domain evaluator. The adapter reports raw rows; this
// builder proves claims from them — a drifted answer surfaces as a
// sentence saying so, never a silent contradiction.
package demos

import (
	"context"
	"fmt"
	"strings"
)

// AuthzMatrixRunner is the port the matrix story runs through; the
// data-layer console implements it over the seeded directory and posts.
type AuthzMatrixRunner interface {
	RunAuthzMatrixStory(ctx context.Context) (AuthzMatrixRun, error)
}

// AuthzRoleGrantRow is one role with the permissions the registry
// grants it — the roster the first beat renders.
type AuthzRoleGrantRow struct {
	Role        string
	Permissions []string
}

// AuthzSubjectRow is one principal the matrix judges: the identity a
// credential would resolve to, the roles the directory assigns, and the
// one-line teaching note that says why this subject exists.
type AuthzSubjectRow struct {
	Username    string
	DisplayName string
	Via         string
	Roles       []string
	Scopes      []string
	Note        string
}

// AuthzPostRow is one owned post the matrix judges against.
type AuthzPostRow struct {
	ID    string
	Title string
	Owner string
}

// AuthzCellRow is one decision the evaluator actually answered:
// subject × action × post → verdict + the evaluator's own reason.
type AuthzCellRow struct {
	Subject string
	Action  string
	PostID  string
	Allowed bool
	Reason  string
}

// AuthzMatrixRun is the story's raw haul: the role roster, the
// subjects, the owned posts, and every evaluated cell.
type AuthzMatrixRun struct {
	Roles    []AuthzRoleGrantRow
	Subjects []AuthzSubjectRow
	Posts    []AuthzPostRow
	Cells    []AuthzCellRow
}

// authzCell finds one cell of the matrix (or "" when the runner did not
// produce it — which the builder reports rather than invents).
func (run AuthzMatrixRun) authzCell(subject, action, postID string) (AuthzCellRow, bool) {
	for _, cell := range run.Cells {
		if cell.Subject == subject && cell.Action == action && cell.PostID == postID {
			return cell, true
		}
	}
	return AuthzCellRow{}, false
}

// authzCellOrMissing renders one matrix position for a beat fact —
// honestly, including the runner-did-not-evaluate case.
func (run AuthzMatrixRun) authzCellOrMissing(subject, action, postID string) string {
	cell, ok := run.authzCell(subject, action, postID)
	if !ok {
		return fmt.Sprintf("%s %s %s: NOT EVALUATED (runner drift)", subject, action, postID)
	}
	verdict := "deny"
	if cell.Allowed {
		verdict = "allow"
	}
	return fmt.Sprintf("%s %s %s: %s — %s", subject, action, postID, verdict, cell.Reason)
}

// authzAllowed answers the raw verdict of one cell (false when the
// runner never evaluated it — fail closed in the prose too).
func (run AuthzMatrixRun) authzAllowed(subject, action, postID string) bool {
	cell, ok := run.authzCell(subject, action, postID)
	return ok && cell.Allowed
}

// BuildAuthzMatrixPayload shapes the matrix story: the roster, the
// principals, and the teaching beats — owner vs editor vs stranger,
// the narrowed credential, the deny-by-default rows, the namespace
// island — every line derived from the run's own cells.
func BuildAuthzMatrixPayload(run AuthzMatrixRun) AuthStoryPayload {
	// The anchors: ada owns the first post, grace owns the second. The
	// builder takes them from the run's post rows, never hard-codes.
	adaPostID, gracePostID := run.authzPostIDOfOwner("ada"), run.authzPostIDOfOwner("grace")

	rosterFacts := make([]AuthFactRow, 0, len(run.Roles))
	for _, role := range run.Roles {
		rosterFacts = append(rosterFacts, AuthFactRow{
			Label: role.Role,
			Value: strings.Join(role.Permissions, " "),
		})
	}
	subjectFacts := make([]AuthFactRow, 0, len(run.Subjects))
	for _, subject := range run.Subjects {
		subjectFacts = append(subjectFacts, AuthFactRow{
			Label: fmt.Sprintf("%s (%s)", subject.Username, strings.Join(subject.Roles, "+")),
			Value: fmt.Sprintf("scopes [%s] via %s — %s", strings.Join(subject.Scopes, " "), subject.Via, subject.Note),
		})
	}
	postFacts := make([]AuthFactRow, 0, len(run.Posts))
	for _, post := range run.Posts {
		postFacts = append(postFacts, AuthFactRow{
			Label: post.ID,
			Value: fmt.Sprintf("%q owned by %s", post.Title, post.Owner),
		})
	}

	beats := []AuthStoryBeat{
		{
			Beat: "the roster", Verdict: AuthVerdictInfo,
			Detail: "RBAC starts as a table: role → permission set, defined in ONE registry. Every grant in the story comes from these rows; a role absent from the table grants nothing.",
			Facts:  rosterFacts,
		},
		{
			Beat: "the principals", Verdict: AuthVerdictInfo,
			Detail: "Six callers, one for every row of the matrix: two are the authentication topic's own seeded users, one is an editor, one an admin, one an author holding a DELIBERATELY narrowed credential, one the deny-by-default control (a role-less account).",
			Facts:  subjectFacts,
		},
		{
			Beat: "the objects", Verdict: AuthVerdictInfo,
			Detail: "Ownership lives with the resource: each post carries its owner, the one column this topic adds to the posts substrate.",
			Facts:  postFacts,
		},
		{
			Beat: "read is collection-level", Verdict: authzVerdictOf(run.authzAllowed("linus", "posts:read", gracePostID)),
			Detail: fmt.Sprintf("linus (reader) reads grace's post → the evaluator allows: reading is gated at the CAPABILITY level only, so any caller holding posts:read reads any post — %s. The anonymous row denies the same action one gate earlier: %s.",
				run.authzCellOrMissing("linus", "posts:read", gracePostID),
				run.authzCellOrMissing("anonymous", "posts:read", gracePostID)),
		},
		{
			Beat: "owner, editor, stranger", Verdict: authzVerdictOf(
				run.authzAllowed("ada", "posts:write", adaPostID) &&
					run.authzAllowed("grace", "posts:write", adaPostID) &&
					!run.authzAllowed("ada", "posts:write", gracePostID)),
			Detail: fmt.Sprintf("Three write verdicts on the objects, one per rule: ada writes her OWN post (object-level pass), grace the EDITOR writes ada's post (the role spans every post), and ada the stranger on grace's post is refused AT the object gate despite holding the capability: %s · %s · %s.",
				run.authzCellOrMissing("ada", "posts:write", adaPostID),
				run.authzCellOrMissing("grace", "posts:write", adaPostID),
				run.authzCellOrMissing("ada", "posts:write", gracePostID)),
		},
		{
			Beat: "scopes narrow roles", Verdict: authzVerdictOf(!run.authzAllowed("kim", "posts:write", adaPostID)),
			Detail: fmt.Sprintf("kim is an AUTHOR like ada — same role, same capability on paper — but the credential carries scopes [posts:read] only, and the scope gate runs FIRST: %s. A role can never widen what the credential narrowed, not even on kim's own post: %s.",
				run.authzCellOrMissing("kim", "posts:write", adaPostID),
				run.authzCellOrMissing("kim", "posts:write", run.authzPostIDOfOwner("kim"))),
		},
		{
			Beat: "deny by default", Verdict: AuthVerdictRejected,
			Detail: fmt.Sprintf("The control rows: guest is AUTHENTICATED with full scopes but holds NO role — every action denies at the role gate: %s. And anonymous denies before any rule runs: %s. Absent rules refuse; nothing is implicitly public.",
				run.authzCellOrMissing("guest", "posts:write", adaPostID),
				run.authzCellOrMissing("anonymous", "posts:write", adaPostID)),
		},
		{
			Beat: "namespaces isolate", Verdict: authzVerdictOf(run.authzAllowed("root", "users:manage", "-") && !run.authzAllowed("ada", "users:manage", "-")),
			Detail: fmt.Sprintf("users:manage is the admin-only island: root (admin, scopes [*]) holds it, ada (author) does not — a posts grant never crosses namespaces: %s · %s.",
				run.authzCellOrMissing("root", "users:manage", "-"),
				run.authzCellOrMissing("ada", "users:manage", "-")),
		},
	}

	allowedWrites, totalWrites := run.authzWriteCounts()
	proofs := []AuthFactRow{
		{Label: "matrix size", Value: fmt.Sprintf("%d evaluated cells (%d subjects × actions × objects)", len(run.Cells), len(run.Subjects))},
		{Label: "write column", Value: fmt.Sprintf("%d of %d write cells allow — the rest deny at scope, role, or object gates", allowedWrites, totalWrites)},
		{Label: "narrowed credential", Value: fmt.Sprintf("kim (author, scopes [posts:read]) denied what ada's identical role allows: %t",
			!run.authzAllowed("kim", "posts:write", adaPostID) && run.authzAllowed("ada", "posts:write", adaPostID))},
		{Label: "deny by default", Value: fmt.Sprintf("guest (no role) denied every action: %t; anonymous denied every action: %t",
			run.authzSubjectAllDenied("guest"), run.authzSubjectAllDenied("anonymous"))},
	}

	reasoning := "Every verdict above is the evaluator's own answer, reason sentence included — no cell was hand-written. Read the matrix as one sentence per gate: " +
		"authentication decides WHO (anonymous denies first), scopes decide what the CREDENTIAL may ask (the narrowest gate, so roles can never widen it), " +
		"roles decide what the USER may do (deny by default when nothing is granted), and ownership decides at the OBJECT (owner, editor, admin — then refusal). " +
		"Stacked in that order, each gate only ever NARROWS the one before it, which is what makes the whole stack auditable: any 403 can name the gate that said no."

	return newAuthStoryPayload("rbac-matrix", beats, proofs, reasoning)
}

// authzPostIDOfOwner answers the first seeded post owned by username
// ("" when the runner did not seed one — which the citing beat shows).
func (run AuthzMatrixRun) authzPostIDOfOwner(owner string) string {
	for _, post := range run.Posts {
		if post.Owner == owner {
			return post.ID
		}
	}
	return "(no seeded post)"
}

// authzWriteCounts derives the write-column tally from the cells.
func (run AuthzMatrixRun) authzWriteCounts() (allowed, total int) {
	for _, cell := range run.Cells {
		if cell.Action == "posts:write" {
			total++
			if cell.Allowed {
				allowed++
			}
		}
	}
	return allowed, total
}

// authzSubjectAllDenied reports whether EVERY evaluated cell for one
// subject denied — the deny-by-default proof.
func (run AuthzMatrixRun) authzSubjectAllDenied(subject string) bool {
	seen := false
	for _, cell := range run.Cells {
		if cell.Subject != subject {
			continue
		}
		seen = true
		if cell.Allowed {
			return false
		}
	}
	return seen
}

// verdictOf maps a derived boolean to the payload's verdict labels —
// green when the teaching claim held, red when the run contradicts it.
func authzVerdictOf(claim bool) string {
	if claim {
		return AuthVerdictOK
	}
	return AuthVerdictRejected
}
