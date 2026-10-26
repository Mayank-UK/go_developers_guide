-- The probe's one logical job as a sqlc query: the posts page WITH its
-- tags, in a single statement — the same LEFT JOIN + array_agg read the
-- raw-pgx baseline runs and topic 17's DAO teaches. The annotation below
-- is the whole contract sqlc needs: :many rows, typed by the schema above.
-- The ::text[] cast is deliberate: it pins the aggregate's type for BOTH
-- the planner and sqlc's inference (COALESCE around a FILTERed array_agg
-- alone infers as untyped).
-- name: ListPostsWithTags :many
SELECT p.id, p.title, p.body, p.created_at,
       (COALESCE(array_agg(pt.tag_name ORDER BY pt.tag_name)
                 FILTER (WHERE pt.tag_name IS NOT NULL), '{}'))::text[] AS tags
FROM ormprobe_posts p
LEFT JOIN ormprobe_post_tags pt ON pt.post_id = p.id
GROUP BY p.id, p.title, p.body, p.created_at
ORDER BY p.id;
