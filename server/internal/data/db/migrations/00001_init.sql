-- +goose Up
-- Topic 16 sql-schema, migration 1: the guide's own core schema — posts,
-- tags, and the post_tags relationship. Every decision below is taught in
-- the topic's prose and verified live by the db:schema-explorer demo.

-- set_updated_at keeps updated_at truthful NO MATTER WHO writes the row —
-- the app, a DBA's hotfix, or a migration backfill. The alternative
-- (app-side stamping) is one forgotten UPDATE away from a lying timestamp;
-- the trigger costs a function call per write and buys a single source of
-- truth. topic 17's repository still sets created_at itself (the app knows
-- the draft's arrival instant), but updated_at belongs to the database.
-- StatementBegin/End wrap the dollar-quoted body: goose splits statements
-- on semicolons, and the semicolons INSIDE $$ … $$ must not split.
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE posts (
    -- uuid, not bigserial: post URLs must not be enumerable, and ids are
    -- minted app-side as UUIDv7 (time-ordered, index-friendly — the ids
    -- topic walks the whole trade-off). DEFAULT gen_random_uuid() is the
    -- safety net for rows inserted by hand.
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title       text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    body        text NOT NULL,
    -- timestamptz, never timestamp: every instant is a moment on the
    -- timeline; naive timestamps invite the UTC-vs-local bug class.
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- The tag representation is the topic's deliberate trade-off: a normalized
-- pair table (not posts.tags text[]). Tags are a RELATIONSHIP here —
-- queryable both ways, joinable, constraint-checkable — at the cost of one
-- join per tagged read. The denormalized array + GIN index is the honest
-- alternative for read-heavy tag filters, and the topic teaches when to
-- switch. Natural key (name): a tag IS its name; a surrogate id would buy
-- renames we do not support and a join we do not need.
CREATE TABLE tags (
    name        text PRIMARY KEY CHECK (name <> '')
);

-- The many-to-many edge. Composite PRIMARY KEY (post_id, tag_name) gives
-- three things at once: identity, "a post cannot carry the same tag twice"
-- (a unique constraint for free), and the posts→tags forward index.
CREATE TABLE post_tags (
    post_id     uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_name    text NOT NULL REFERENCES tags(name) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_name)
);

-- Indexes are per-query decisions, not decoration. Rationale:
--   posts_created_at_idx  — the API's default listing is newest-first
--                           (ORDER BY created_at DESC); a descending btree
--                           serves it without a sort node.
--   post_tags_tag_name_idx — the composite PK covers post_id lookups only
--                           (it is the LEADING column); the reverse
--                           direction — "which posts carry tag=go" — needs
--                           its own btree on tag_name. PostgreSQL does NOT
--                           auto-index foreign key columns.
CREATE INDEX posts_created_at_idx   ON posts (created_at DESC);
CREATE INDEX post_tags_tag_name_idx ON post_tags (tag_name);

-- The updated_at trigger, attached.
CREATE TRIGGER posts_set_updated_at
    BEFORE UPDATE ON posts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
-- Down is the rewind for local experiments, written to be safe on the way
-- out too: child tables first, then the function they all reference.
DROP TABLE IF EXISTS post_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS posts;
DROP FUNCTION IF EXISTS set_updated_at();
