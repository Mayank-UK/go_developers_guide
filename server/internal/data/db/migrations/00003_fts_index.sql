-- +goose Up
-- Topic 17 databases-sql, migration 3: the full-text search expression
-- index. The tsvector values are DERIVED (to_tsvector over title and body
-- at write time would denormalize a fact the text already owns), so the
-- index is an EXPRESSION index — PostgreSQL indexes the computed tsvector
-- and the query matches when it computes the SAME expression. The 'english'
-- dictionary is pinned in both index and query (an immutable expression
-- requires a fixed configuration); the db:search demo runs the query this
-- index serves and the schema explorer shows the index in the catalog.

CREATE INDEX posts_search_idx ON posts
    USING gin (to_tsvector('english', title || ' ' || body));

-- +goose Down
DROP INDEX IF EXISTS posts_search_idx;
