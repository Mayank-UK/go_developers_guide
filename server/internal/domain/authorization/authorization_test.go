package authorization

import "testing"

// The registry facts the evaluator leans on: every role's exact grant
// set, unknown roles granting nothing, and copies leaving the registry
// untouched.
func TestAuthorizationRoleRegistryGrantsExactSets(t *testing.T) {
	cases := []struct {
		role  Role
		want  []Permission
		grant []Permission
		deny  []Permission
	}{
		{RoleReader, []Permission{PermPostsRead},
			[]Permission{PermPostsRead}, []Permission{PermPostsWrite, PermUsersManage}},
		{RoleAuthor, []Permission{PermPostsRead, PermPostsWrite},
			[]Permission{PermPostsRead, PermPostsWrite}, []Permission{PermUsersManage}},
		{RoleEditor, []Permission{PermPostsRead, PermPostsWrite},
			[]Permission{PermPostsRead, PermPostsWrite}, []Permission{PermUsersManage}},
		{RoleAdmin, []Permission{PermPostsRead, PermPostsWrite, PermUsersManage},
			[]Permission{PermPostsRead, PermPostsWrite, PermUsersManage}, nil},
	}
	for _, tc := range cases {
		granted, ok := PermissionsOf(tc.role)
		if !ok {
			t.Fatalf("PermissionsOf(%s) must find the role", tc.role)
		}
		if len(granted) != len(tc.want) {
			t.Fatalf("PermissionsOf(%s) = %v, want %v", tc.role, granted, tc.want)
		}
		for i := range granted {
			if granted[i] != tc.want[i] {
				t.Fatalf("PermissionsOf(%s) = %v, want %v", tc.role, granted, tc.want)
			}
		}
		for _, perm := range tc.grant {
			if !RoleGrants(tc.role, perm) {
				t.Errorf("RoleGrants(%s, %s) = false, want true", tc.role, perm)
			}
		}
		for _, perm := range tc.deny {
			if RoleGrants(tc.role, perm) {
				t.Errorf("RoleGrants(%s, %s) = true, want false", tc.role, perm)
			}
		}
	}
}

func TestAuthorizationUnknownRoleGrantsNothing(t *testing.T) {
	if _, ok := PermissionsOf(Role("intern")); ok {
		t.Error("PermissionsOf(unknown role) must answer false")
	}
	for _, perm := range []Permission{PermPostsRead, PermPostsWrite, PermUsersManage} {
		if RoleGrants(Role("superuser"), perm) {
			t.Errorf("unknown role granted %s — deny by default violated", perm)
		}
	}
	if RolesGrant([]Role{Role("intern"), RoleReader}, PermPostsWrite) {
		t.Error("a known role in the list must not be widened by an unknown one")
	}
	if !RolesGrant([]Role{Role("intern"), RoleReader}, PermPostsRead) {
		t.Error("union semantics: the reader half of the list must still grant posts:read")
	}
	if RolesGrant(nil, PermPostsRead) {
		t.Error("empty role list must grant nothing")
	}
}

func TestAuthorizationRolesVocabularyIsOrdered(t *testing.T) {
	want := []Role{RoleReader, RoleAuthor, RoleEditor, RoleAdmin}
	got := Roles()
	if len(got) != len(want) {
		t.Fatalf("Roles() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Roles() = %v, want %v", got, want)
		}
	}
}

// A returned permission set is a COPY: mutating it must not leak into
// the registry and silently re-grant the world.
func TestAuthorizationPermissionsOfReturnsACopy(t *testing.T) {
	granted, _ := PermissionsOf(RoleReader)
	granted[0] = PermUsersManage
	if RoleGrants(RoleReader, PermUsersManage) {
		t.Fatal("caller mutation leaked into the role registry")
	}
	again, _ := PermissionsOf(RoleReader)
	if again[0] != PermPostsRead {
		t.Fatalf("registry mutated: %v", again)
	}
}
