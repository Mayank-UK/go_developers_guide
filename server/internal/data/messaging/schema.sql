-- The transactional outbox demo's tables (topic 22). Deliberately NOT a
-- goose migration: migrations are pinned at three files by the
-- sql-schema topic's tests, and these tables are demo-scoped the same
-- way ormprobe's are — created idempotently (IF NOT EXISTS) by the
-- outbox adapter on first use. In production this DDL is a migration;
-- the transactional SHAPE (event row + outbox row, one transaction) is
-- what the demo teaches and is identical either way.

-- The business table: the state change the application commits. In
-- production this is orders, posts, payments; here one named event per
-- row, with run_id scoping each demo run's rows so concurrent runs and
-- the closing cleanup touch only their own work.
CREATE TABLE IF NOT EXISTS msgdemo_events (
    id         text PRIMARY KEY,
    run_id     text NOT NULL,
    kind       text NOT NULL,
    payload    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- The outbox: WHAT to publish (payload) and WHERE (stream), plus the
-- published_at seam the relay stamps. id is the app-minted message id
-- consumers deduplicate on — identical across every redelivery.
CREATE TABLE IF NOT EXISTS msgdemo_outbox (
    id           text PRIMARY KEY,
    run_id       text NOT NULL,
    stream       text NOT NULL,
    payload      text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);

-- The relay's driving query: pending rows for a run, oldest first. A
-- PARTIAL index carries only the unpublished rows — the published
-- majority never enters the index (the index-lesson from sql-schema,
-- applied to a queue's shape).
CREATE INDEX IF NOT EXISTS idx_msgdemo_outbox_pending
    ON msgdemo_outbox (run_id, created_at)
    WHERE published_at IS NULL;
