-- +goose Up
-- Topic 16 sql-schema, migration 2: the audit-logging pattern's table.
-- One append-only row per state change of a post, written INSIDE the same
-- transaction as the change it describes — so the history and the change
-- commit or roll back together (topic 17's db:audit demo runs that dance).

CREATE TABLE audit_log (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Nullable ON PURPOSE, with ON DELETE SET NULL: the audit trail must
    -- OUTLIVE the thing it describes. CASCADE would rewrite history on
    -- every delete; RESTRICT would make posts undeletable; SET NULL keeps
    -- the row (action + actor + timestamp survive) while admitting the
    -- subject is gone. The pre-delete snapshot rides in the summary.
    post_id     uuid REFERENCES posts(id) ON DELETE SET NULL,
    action      text NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    actor       text NOT NULL,
    summary     text NOT NULL,
    changed_at  timestamptz NOT NULL DEFAULT now()
);

-- "Show me this post's history, newest first" is the query the pattern
-- exists for: a btree on (post_id, changed_at DESC) serves it directly.
CREATE INDEX audit_log_post_changed_idx ON audit_log (post_id, changed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
