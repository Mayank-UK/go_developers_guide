// Package authorization is the topic 24 data layer: the in-memory
// adapters behind the domain's two ports (role assignments and post
// ownership) plus the console that seeds them and runs the matrix
// story. The shape mirrors topic 23's auth console deliberately — one
// constructible substrate wired at the composition root, engines in
// the domain, seeds here, and the same honest note: in-memory state is
// teaching substrate; production backs these ports with a roles table
// (or identity-provider group claims) and the posts table's owner
// column, without touching a line of policy.
package authorization

import (
	"context"
	"time"

	domauth "guide/internal/domain/auth"
	domauthz "guide/internal/domain/authorization"
	"guide/internal/domain/demos"
)

// Console is topic 24's whole substrate: the seeded role directory, the
// seeded owned posts, and the matrix story over both. It implements
// the domain's RoleDirectory and PostOwnership ports (the middleware
// consumes those) and the demos runner port (the handler consumes
// that) — one value, three hats, wired once at the composition root.
type Console struct {
	directory *Directory
	posts     []domauthz.OwnedPost
}

// Compile-time proofs the console satisfies every port it is wired to.
var (
	_ domauthz.RoleDirectory  = (*Console)(nil)
	_ domauthz.PostOwnership  = (*Console)(nil)
	_ demos.AuthzMatrixRunner = (*Console)(nil)
)

// Directory is the in-memory RoleDirectory: username → roles. An
// absent username answers the empty slice — unassigned, which the
// evaluator turns into a universal deny — never an error.
type Directory struct {
	assignments map[string][]domauthz.Role
}

// NewDirectory builds a directory over the seed assignments.
func NewDirectory(assignments map[string][]domauthz.Role) *Directory {
	return &Directory{assignments: assignments}
}

// RolesFor answers the username's assigned roles; empty means
// unassigned (deny by default).
func (d *Directory) RolesFor(_ context.Context, username string) ([]domauthz.Role, error) {
	return d.assignments[username], nil
}

// seededAssignments is the roster the demos and the topic's prose share:
// the authentication topic's own two users (ada the author, linus the
// reader), grace the editor, root the admin, kim an author holding a
// deliberately narrowed credential — and NO entry for guest, the
// authenticated-but-unassigned deny-by-default control.
var seededAssignments = map[string][]domauthz.Role{
	"ada":   {domauthz.RoleAuthor},
	"linus": {domauthz.RoleReader},
	"grace": {domauthz.RoleEditor},
	"root":  {domauthz.RoleAdmin},
	"kim":   {domauthz.RoleAuthor},
}

// seededAt is the fixed stamp on the seeded posts — deterministic
// payloads, honest timestamps.
var seededAt = time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)

// seededPosts is the owned substrate: real posts.Post entities with the
// one column this topic adds. ada owns the sessions post, grace owns
// the draft, kim owns the post her narrowed credential cannot write.
var seededPosts = []domauthz.OwnedPost{
	domauthz.NewOwnedPost("p-sessions", "Sealed cookie sessions",
		"The login flow from topic 23, as a post owned by ada.", "ada", []string{"auth"}, seededAt),
	domauthz.NewOwnedPost("p-draft", "Editing someone else's draft",
		"The object every stranger is refused on: grace's post.", "grace", []string{"process"}, seededAt),
	domauthz.NewOwnedPost("p-narrowed", "A read-only credential",
		"Kim's own post — unwritable through the narrowed key she holds.", "kim", []string{"authz"}, seededAt),
}

// NewConsole wires the topic 24 substrate from its seeds.
func NewConsole() *Console {
	return &Console{directory: NewDirectory(seededAssignments), posts: seededPosts}
}

// RolesFor answers the username's assigned roles through the console's
// directory — the RoleDirectory port the permission middleware consults.
func (c *Console) RolesFor(ctx context.Context, username string) ([]domauthz.Role, error) {
	return c.directory.RolesFor(ctx, username)
}

// PostOwner answers the owning username of the post at id — the
// PostOwnership port the object-level middleware consults.
func (c *Console) PostOwner(_ context.Context, id string) (string, bool, error) {
	for _, post := range c.posts {
		if post.ID == id {
			return post.Owner, true, nil
		}
	}
	return "", false, nil
}

// authzSubject is one matrix principal: the identity a credential
// resolves to plus the teaching note the payload carries. The scopes
// match what each carrier would really hand over — ada's and linus's
// are the authentication console's own seed values, kim's are the
// read-only mint topic 23's key service would produce for a posts:read
// request, and root's [*] is the admin token that delegates everything.
type authzSubject struct {
	identity domauth.Identity
	note     string
}

// seededSubjects is the matrix's rows, judgment order: the anonymous
// control, the reader, the author, the narrowed author, the editor,
// the admin, and the role-less control. ada and linus carry the SAME
// username + scopes the auth console seeds, so a real login of either
// user meets the SAME verdicts through the middleware.
var seededSubjects = []authzSubject{
	{domauth.Identity{}, "no identity at all — every gate denies"},
	{domauth.Identity{Username: "linus", DisplayName: "Linus", Scopes: []string{"posts:read"}, Via: "session"},
		"the authentication topic's read-only user, assigned reader"},
	{domauth.Identity{Username: "ada", DisplayName: "Ada", Scopes: []string{"posts:read", "posts:write"}, Via: "session"},
		"the authentication topic's author — owner of the sessions post"},
	{domauth.Identity{Username: "kim", DisplayName: "Kim", Scopes: []string{"posts:read"}, Via: "api-key"},
		"an author whose credential was minted read-only — scopes narrow roles"},
	{domauth.Identity{Username: "grace", DisplayName: "Grace", Scopes: []string{"posts:read", "posts:write"}, Via: "jwt"},
		"the editor — writes span every post, not just her own"},
	{domauth.Identity{Username: "root", DisplayName: "Root", Scopes: []string{"*"}, Via: "jwt"},
		"the admin — every permission, ownership overridden"},
	{domauth.Identity{Username: "guest", DisplayName: "Guest", Scopes: []string{"*"}, Via: "session"},
		"authenticated, full scopes, NO role — the deny-by-default control"},
}

// RunAuthzMatrixStory evaluates the full matrix honestly: every
// subject × every protected action × every seeded post through the REAL
// domain evaluator, plus the namespace-island pair (users:manage is
// not object-level, so it rides a synthetic "-"). Raw rows out; the
// demos builder shapes the payload and derives every claim.
func (c *Console) RunAuthzMatrixStory(ctx context.Context) (demos.AuthzMatrixRun, error) {
	roles := make([]demos.AuthzRoleGrantRow, 0, len(domauthz.Roles()))
	for _, role := range domauthz.Roles() {
		permissions, ok := domauthz.PermissionsOf(role)
		if !ok {
			continue // the registry only answers for roles it defines
		}
		names := make([]string, len(permissions))
		for i, permission := range permissions {
			names[i] = string(permission)
		}
		roles = append(roles, demos.AuthzRoleGrantRow{Role: string(role), Permissions: names})
	}

	subjects := make([]demos.AuthzSubjectRow, 0, len(seededSubjects))
	for _, subject := range seededSubjects {
		assigned, err := c.directory.RolesFor(ctx, subject.identity.Username)
		if err != nil {
			return demos.AuthzMatrixRun{}, err
		}
		subjects = append(subjects, demos.AuthzSubjectRow{
			Username:    authzSubjectLabel(subject),
			DisplayName: subject.identity.DisplayName,
			Via:         subject.identity.Via,
			Roles:       roleNames(assigned),
			Scopes:      subject.identity.Scopes,
			Note:        subject.note,
		})
	}

	posts := make([]demos.AuthzPostRow, 0, len(c.posts))
	for _, post := range c.posts {
		posts = append(posts, demos.AuthzPostRow{ID: post.ID, Title: post.Title, Owner: post.Owner})
	}

	actions := []domauthz.Permission{domauthz.PermPostsRead, domauthz.PermPostsWrite}
	cells := make([]demos.AuthzCellRow, 0, len(seededSubjects)*(len(actions)*len(c.posts)+1))
	for _, subject := range seededSubjects {
		assigned, err := c.directory.RolesFor(ctx, subject.identity.Username)
		if err != nil {
			return demos.AuthzMatrixRun{}, err
		}
		for _, action := range actions {
			for _, post := range c.posts {
				decision := domauthz.DecidePost(subject.identity, assigned, action, post)
				cells = append(cells, demos.AuthzCellRow{
					Subject: authzSubjectLabel(subject),
					Action:  string(action),
					PostID:  post.ID,
					Allowed: decision.Allowed,
					Reason:  decision.Reason,
				})
			}
		}
		// The namespace island: users:manage has no object — one cell
		// per subject with the synthetic post id "-".
		decision := domauthz.Authorize(subject.identity, assigned, domauthz.PermUsersManage)
		cells = append(cells, demos.AuthzCellRow{
			Subject: authzSubjectLabel(subject),
			Action:  string(domauthz.PermUsersManage),
			PostID:  "-",
			Allowed: decision.Allowed,
			Reason:  decision.Reason,
		})
	}

	return demos.AuthzMatrixRun{Roles: roles, Subjects: subjects, Posts: posts, Cells: cells}, nil
}

// authzSubjectLabel renders the row's display name — the anonymous
// control EVALUATES as the zero identity (honest) but LABELS itself,
// so the payload's rows are readable and the builder can cite them.
func authzSubjectLabel(subject authzSubject) string {
	if subject.identity.Username == "" {
		return "anonymous"
	}
	return subject.identity.Username
}

// roleNames renders roles for the payload rows.
func roleNames(roles []domauthz.Role) []string {
	names := make([]string, len(roles))
	for i, role := range roles {
		names[i] = string(role)
	}
	return names
}
