package demos

import (
	"strings"
	"testing"
)

// The schema-explorer payload builder: pure shaping over a snapshot — the
// tests feed hand-built (deliberately unsorted, gap-carrying) snapshots
// and pin the mapping: columns grouped under tables, curated purposes and
// rationale notes attached ONLY to subjects the snapshot contains, every
// collection deterministically ordered and never nil.

func TestDBSchemaExplorerBuilderGroupsColumnsUnderTablesInOrder(t *testing.T) {
	t.Parallel()

	payload := BuildSchemaExplorerPayload(SchemaSnapshot{
		Tables: []string{"posts", "audit_log"},
		Columns: []ColumnRow{
			{Table: "audit_log", Name: "id", Type: "uuid", Nullable: false, HasDefault: true, Default: "gen_random_uuid()"},
			{Table: "posts", Name: "id", Type: "uuid", Nullable: false, HasDefault: true, Default: "gen_random_uuid()"},
			{Table: "posts", Name: "created_at", Type: "timestamp with time zone", Nullable: false, HasDefault: true, Default: "now()"},
			{Table: "posts", Name: "title", Type: "text", Nullable: false},
		},
	})

	// Tables sort by name even though the snapshot arrived unsorted, and
	// columns keep their per-table declaration order (id before title).
	if len(payload.Tables) != 2 || payload.Tables[0].Name != "audit_log" || payload.Tables[1].Name != "posts" {
		t.Fatalf("tables = %+v, want audit_log then posts", payload.Tables)
	}
	posts := payload.Tables[1]
	if posts.Columns[0].Name != "id" || posts.Columns[1].Name != "created_at" || posts.Columns[2].Name != "title" {
		t.Errorf("posts columns drifted from declaration order: %+v", posts.Columns)
	}
	if posts.Columns[0].Default != "gen_random_uuid()" {
		t.Errorf("posts.id default = %q, want gen_random_uuid()", posts.Columns[0].Default)
	}
	if posts.Columns[2].Default != "" {
		t.Errorf("title default = %q, want empty (no default, no lie)", posts.Columns[2].Default)
	}
	if posts.Columns[2].Nullable {
		t.Error("title must be NOT NULL as the snapshot said")
	}
}

func TestDBSchemaExplorerBuilderSortsConstraintsAndIndexes(t *testing.T) {
	t.Parallel()

	payload := BuildSchemaExplorerPayload(SchemaSnapshot{
		Tables: []string{"posts"},
		Constraints: []ConstraintRow{
			{Table: "posts", Name: "posts_title_key", Kind: "UNIQUE", Definition: "UNIQUE (title)"},
			{Table: "posts", Name: "posts_pkey", Kind: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"},
			{Table: "audit_log", Name: "audit_log_pkey", Kind: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"},
		},
		Indexes: []IndexRow{
			{Table: "posts", Name: "posts_created_at_idx", Kind: "btree", Columns: []string{"created_at"}},
			{Table: "posts", Name: "posts_pkey", Kind: "btree", Primary: true, Columns: []string{"id"}},
		},
	})

	if payload.Constraints[0].Name != "audit_log_pkey" ||
		payload.Constraints[1].Name != "posts_pkey" ||
		payload.Constraints[2].Name != "posts_title_key" {
		t.Errorf("constraints not sorted by (table, name): %+v", payload.Constraints)
	}
	if payload.Indexes[0].Name != "posts_created_at_idx" || payload.Indexes[1].Name != "posts_pkey" {
		t.Errorf("indexes not sorted by (table, name): %+v", payload.Indexes)
	}
	if payload.Indexes[1].Primary != true {
		t.Error("posts_pkey must keep its primary flag through the mapping")
	}
}

func TestDBSchemaExplorerBuilderAttachesOnlyLiveNotesAndPurposes(t *testing.T) {
	t.Parallel()

	payload := BuildSchemaExplorerPayload(SchemaSnapshot{
		Tables: []string{"posts"},
		Columns: []ColumnRow{
			{Table: "posts", Name: "id", Type: "uuid"},
		},
		Indexes: []IndexRow{
			{Table: "posts", Name: "posts_created_at_idx", Kind: "btree"},
		},
	})

	// posts carries a purpose; a table with no curated purpose still
	// renders (empty string is data the view styles around).
	if payload.Tables[0].Purpose == "" {
		t.Error("posts must carry its curated purpose")
	}
	for _, note := range payload.Notes {
		if !strings.HasPrefix(note.Subject, "posts") {
			t.Errorf("note subject %q survives without a live subject — notes must filter to the snapshot", note.Subject)
		}
	}
	// The composite-PK rationale's subject (post_tags_pkey) is absent
	// here, so it must be gone.
	for _, note := range payload.Notes {
		if note.Subject == "post_tags_pkey" {
			t.Error("note for absent constraint post_tags_pkey must be dropped")
		}
	}
}

func TestDBSchemaExplorerBuilderNeverServesNilCollections(t *testing.T) {
	t.Parallel()

	payload := BuildSchemaExplorerPayload(SchemaSnapshot{
		Tables:  []string{"posts", "mystery"},
		Columns: []ColumnRow{{Table: "posts", Name: "id", Type: "uuid"}},
		Indexes: []IndexRow{{Table: "posts", Name: "posts_pkey", Kind: "btree"}},
	})

	// An index whose columns list did not arrive maps to [] on the wire,
	// never null.
	if payload.Indexes[0].Columns == nil {
		t.Fatal("index columns must be a non-nil empty slice when the introspector sent none")
	}
	if len(payload.Indexes[0].Columns) != 0 {
		t.Errorf("index columns = %v, want empty", payload.Indexes[0].Columns)
	}
	// A table with no curated purpose still renders — empty string is
	// data (the view styles around it), not a dropped table. Sorted by
	// name, "mystery" precedes "posts".
	if payload.Tables[0].Name != "mystery" || payload.Tables[0].Purpose != "" {
		t.Errorf("uncurated table = %+v, want present with an empty purpose", payload.Tables[0])
	}
	if payload.Tables[0].Columns == nil {
		t.Error("a table without introspected columns must carry [], not null")
	}
}
