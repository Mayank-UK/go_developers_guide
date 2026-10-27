package authorization

import (
	"fmt"
	"time"

	domauth "guide/internal/domain/auth"
	"guide/internal/domain/posts"
)

// OwnedPost is the posts substrate with the one column topic 24 adds:
// WHO made it. It embeds the real posts.Post entity unchanged — the
// resource the earlier topics built — and adds the owning username the
// object-level rule judges against. Production would carry the same
// fact as an owner_id column from topic 16's migration thinking; the
// embedding makes the extension honest instead of retconning the
// entity.
type OwnedPost struct {
	posts.Post
	// Owner is the username of the account that created this post.
	Owner string
}

// NewOwnedPost stamps an owned post for seeds and tests.
func NewOwnedPost(id, title, body, owner string, tags []string, at time.Time) OwnedPost {
	return OwnedPost{
		Post:  posts.Post{ID: id, Title: title, Body: body, Tags: tags, CreatedAt: at, UpdatedAt: at},
		Owner: owner,
	}
}

// DecidePost is the object-level rule: the full decision pipeline for
// one identity acting on ONE post. It runs Authorize first — scopes
// and roles must already allow the ACTION — then polices the OBJECT:
//
//   - posts:read is collection-level: any caller whose capability
//     passed may read any post. Reads are not ownership-policed here;
//     private resources would add a visibility field, not a new rule.
//   - admins override ownership (break-glass for incident response).
//   - editors write ANY post — the role's object-level meaning; this is
//     the row that makes editor differ from author despite identical
//     permission lists.
//   - everyone else may write a post only when they OWN it.
//   - anything else denies — including a narrowed credential on the
//     owner's OWN post: the capability gate runs first, so "author with
//     a read-only credential" cannot write even their own work.
func DecidePost(identity domauth.Identity, roles []Role, action Permission, post OwnedPost) Decision {
	capability := Authorize(identity, roles, action)
	if !capability.Allowed {
		return capability
	}
	switch {
	case action == PermPostsRead:
		return allow("reading is collection-level — any caller holding posts:read may read any post")
	case hasRole(roles, RoleAdmin):
		return allow("admin overrides object ownership — the break-glass row")
	case hasRole(roles, RoleEditor):
		return allow("editor writes span every post — the role's object-level meaning")
	case post.Owner == identity.Username:
		return allow(fmt.Sprintf("object-level pass: %s owns %s", post.Owner, post.ID))
	default:
		return deny(fmt.Sprintf("object-level refusal: %s owns %s and %s is neither its owner, an editor, nor an admin",
			post.Owner, post.ID, identity.Username))
	}
}

// hasRole reports membership without imports beyond the vocabulary.
func hasRole(roles []Role, want Role) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}
