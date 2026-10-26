// The PostgreSQL PostsRepository: pgx v5 over topic 16's schema, the live
// store behind the port the API has served since topic 11. Every statement
// is parameterized ($1, $2, …) — the DAO discipline the sql-safety demo
// runs live; ids are UUIDv7 minted app-side with the same RFC 9562 layout
// the uuid lab teaches (the lab's generator lives in domain/demos, which
// imports this package for its exhibits, so the mint lives here
// independently); multi-statement writes (a post and its tag edges) ride
// one transaction. All pgx failures map to the typed service_unavailable
// error — an unreachable live store is an honest 503, never a 500.
package postsdata

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// Compile-time proof the PostgreSQL adapter satisfies the port.
var _ posts.PostsRepository = (*PostgresRepository)(nil)

// SeedPostIDs anchors the six seed posts on fixed UUIDv7 literals carrying
// their creation instants in the leading 48 bits — the same values the
// in-memory seeds carry as p-1…p-6 on the same January 2026 dates. The
// literals never change, which is what makes Seed idempotent: ON CONFLICT
// (id) DO NOTHING turns every re-seed into a no-op.
// SeedPostIDs are the six fixed UUIDv7 literals of the seed posts, oldest
// first — exported so the data-layer demos can pin their exhibits to
// the seeded rows (and only those), keeping payloads deterministic
// against demo traffic.
var SeedPostIDs = [6]string{
	"019b8d70-3620-7101-82b6-a17c9d4e0f01",
	"019b98e6-7a00-7102-82b6-a17c9d4e0f02",
	"019ba6ef-ebe0-7103-82b6-a17c9d4e0f03",
	"019bb33d-55e0-7104-82b6-a17c9d4e0f04",
	"019bc17d-b640-7105-82b6-a17c9d4e0f05",
	"019bd1f1-2340-7106-82b6-a17c9d4e0f06",
}

// PostgresRepository is the PostgreSQL adapter: a share of the process-wide pool
// (never its own connections), a clock seam, and an id seam. Writes that
// touch more than one row (post + tag edges) run inside one transaction;
// reads that need two statements to answer one question (the list's totals
// and its window) run inside one transaction too, so a concurrent writer
// cannot make a page disagree with its own metadata.
type PostgresRepository struct {
	pool  *pgxpool.Pool
	now   func() time.Time
	newID func() (string, error)
}

// NewPostgresRepository wires the live adapter over the shared pool, on
// the production clock and the production UUIDv7 minter.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool:  pool,
		now:   func() time.Time { return time.Now().UTC() },
		newID: mintUUIDv7,
	}
}

// listOrderBy maps the closed sort vocabulary to fixed ORDER BY clauses —
// never interpolated from user input; the vocabulary is validated before
// the repository is consulted, and the tie-break is always the id so
// equal keys never leave the order to chance.
func listOrderBy(sort string) string {
	switch sort {
	case posts.SortOldest:
		return "p.created_at ASC, p.id ASC"
	case posts.SortTitle:
		return "lower(p.title) ASC, p.id ASC"
	default: // SortNewest — the API's default ordering.
		return "p.created_at DESC, p.id DESC"
	}
}

// List implements the port: one transaction holds the totals and the page
// on a single snapshot; the filter is exact tag match through the edge
// table (an EXISTS subquery, not a join, so totals count posts and not
// edges); tags aggregate sorted, which is the set order both stores keep.
func (r *PostgresRepository) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return posts.PostPage{}, unavailable(err)
	}
	defer tx.Rollback(ctx)

	var total int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM posts p
		WHERE ($1 = '' OR p.id IN (SELECT post_id FROM post_tags WHERE tag_name = $1))`,
		query.Tag).Scan(&total); err != nil {
		return posts.PostPage{}, unavailable(err)
	}

	order := listOrderBy(query.Sort)
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT p.id, p.title, p.body, p.created_at, p.updated_at,
		       COALESCE(array_agg(pt.tag_name ORDER BY pt.tag_name)
		                FILTER (WHERE pt.tag_name IS NOT NULL), '{}') AS tags
		FROM posts p
		LEFT JOIN post_tags pt ON pt.post_id = p.id
		WHERE ($1 = '' OR p.id IN (SELECT post_id FROM post_tags WHERE tag_name = $1))
		GROUP BY p.id, p.title, p.body, p.created_at, p.updated_at
		ORDER BY %s
		LIMIT $2 OFFSET $3`, order),
		query.Tag, query.Limit, (query.Page-1)*query.Limit)
	if err != nil {
		return posts.PostPage{}, unavailable(err)
	}
	items, err := collectPosts(ctx, rows)
	if err != nil {
		return posts.PostPage{}, unavailable(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return posts.PostPage{}, unavailable(err)
	}

	// The same page arithmetic the in-memory adapter runs — the shared
	// behavior table pins both stores to identical metadata, including the
	// zero totalPages of an empty match.
	totalPages := (total + query.Limit - 1) / query.Limit
	return posts.PostPage{
		Items:      items,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalItems: total,
		TotalPages: totalPages,
		HasNext:    query.Page < totalPages,
		HasPrev:    query.Page > 1,
	}, nil
}

// GetByID implements the port: the row and its tags in one statement, or
// the typed miss. An id that is not a canonical UUID is a miss, not a
// driver error — the fake answers 404 for any unknown shape and so does
// the live store.
func (r *PostgresRepository) GetByID(ctx context.Context, id string) (posts.Post, error) {
	key, ok := parseUUID(id)
	if !ok {
		return posts.Post{}, core.Newf(core.KindNotFound, "Post %q not found.", id)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.title, p.body, p.created_at, p.updated_at,
		       COALESCE((SELECT array_agg(pt.tag_name ORDER BY pt.tag_name)
		                 FROM post_tags pt WHERE pt.post_id = p.id), '{}') AS tags
		FROM posts p
		WHERE p.id = $1`, key)
	if err != nil {
		return posts.Post{}, unavailable(err)
	}
	items, err := collectPosts(ctx, rows)
	if err != nil {
		return posts.Post{}, unavailable(err)
	}
	if len(items) == 0 {
		return posts.Post{}, core.Newf(core.KindNotFound, "Post %q not found.", id)
	}
	return items[0], nil
}

// Create implements the port: mint the UUIDv7, insert the post, grow the
// tag vocabulary, attach the edges — one transaction, all or nothing.
func (r *PostgresRepository) Create(ctx context.Context, draft posts.Draft) (posts.Post, error) {
	id, err := r.newID()
	if err != nil {
		return posts.Post{}, unavailable(err)
	}
	key, ok := parseUUID(id)
	if !ok {
		return posts.Post{}, core.Wrapf(err, core.KindInternal, "The id minter produced a malformed id.")
	}
	now := r.now()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return posts.Post{}, unavailable(err)
	}
	defer tx.Rollback(ctx)

	tags := uniqueTags(draft.Tags)
	// RETURNING both stamps: the adapter reports exactly what the database
	// states — timestamptz keeps microseconds, and the caller's post must
	// not carry a nanosecond tail the store never accepted.
	var createdAt, updatedAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO posts (id, title, body, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING created_at, updated_at`,
		key, draft.Title, draft.Body, now).Scan(&createdAt, &updatedAt); err != nil {
		return posts.Post{}, unavailable(err)
	}
	if err := attachTags(ctx, tx, key, tags); err != nil {
		return posts.Post{}, unavailable(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return posts.Post{}, unavailable(err)
	}
	return posts.Post{
		ID:        id,
		Title:     draft.Title,
		Body:      draft.Body,
		Tags:      append([]string(nil), tags...),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// Replace implements the port: full replacement — the draft IS the next
// state, so the old tag edges die and the new ones attach, atomically with
// the row update. updated_at is stamped by the topic 16 trigger; RETURNING
// hands the stamped value back so the adapter reports what the database
// did, not what it assumes.
func (r *PostgresRepository) Replace(ctx context.Context, id string, draft posts.Draft) (posts.Post, error) {
	key, ok := parseUUID(id)
	if !ok {
		return posts.Post{}, core.Newf(core.KindNotFound, "Post %q not found.", id)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return posts.Post{}, unavailable(err)
	}
	defer tx.Rollback(ctx)

	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE posts SET title = $2, body = $3 WHERE id = $1
		RETURNING created_at, updated_at`,
		key, draft.Title, draft.Body).Scan(&createdAt, &updatedAt)
	if err != nil {
		if errorsIsNoRows(err) {
			return posts.Post{}, core.Newf(core.KindNotFound, "Post %q not found.", id)
		}
		return posts.Post{}, unavailable(err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM post_tags WHERE post_id = $1`, key); err != nil {
		return posts.Post{}, unavailable(err)
	}
	tags := uniqueTags(draft.Tags)
	if err := attachTags(ctx, tx, key, tags); err != nil {
		return posts.Post{}, unavailable(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return posts.Post{}, unavailable(err)
	}
	return posts.Post{
		ID:        id,
		Title:     draft.Title,
		Body:      draft.Body,
		Tags:      append([]string(nil), tags...),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// Delete implements the port under the strict stance: remove and answer
// nil, or report the typed miss — and the tag edges go with the post by
// the schema's ON DELETE CASCADE, the database enforcing what the fake
// removes by hand.
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	key, ok := parseUUID(id)
	if !ok {
		return core.Newf(core.KindNotFound, "Post %q not found.", id)
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, key)
	if err != nil {
		return unavailable(err)
	}
	if tag.RowsAffected() == 0 {
		return core.Newf(core.KindNotFound, "Post %q not found.", id)
	}
	return nil
}

// attachTags grows the tag vocabulary (names that already exist are a
// no-op) and attaches the edges (a repeated pair is a no-op), inside the
// caller's transaction. An empty tag set attaches nothing.
func attachTags(ctx context.Context, tx pgx.Tx, key [16]byte, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tags (name)
		SELECT * FROM unnest($1::text[])
		ON CONFLICT (name) DO NOTHING`, tags); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO post_tags (post_id, tag_name)
		SELECT $1, unnest($2::text[])
		ON CONFLICT DO NOTHING`, key, tags); err != nil {
		return err
	}
	return nil
}

// collectPosts drains a post query into domain posts, materializing the
// tag slice (never nil, per the wire's []-not-null rule) and closing the
// rows. UUIDs cross the boundary in their canonical wire form.
func collectPosts(ctx context.Context, rows pgx.Rows) ([]posts.Post, error) {
	defer rows.Close()
	items := make([]posts.Post, 0, 8)
	for rows.Next() {
		var post posts.Post
		var key [16]byte
		var tags []string
		if err := rows.Scan(&key, &post.Title, &post.Body, &post.CreatedAt, &post.UpdatedAt, &tags); err != nil {
			return nil, err
		}
		post.ID = formatUUID(key)
		post.Tags = append([]string{}, tags...)
		post.CreatedAt = post.CreatedAt.UTC()
		post.UpdatedAt = post.UpdatedAt.UTC()
		items = append(items, post)
	}
	return items, rows.Err()
}

// Seed inserts the six deterministic guide posts (the same collection the
// in-memory fake serves) if they are not present, and returns how many
// posts THIS call inserted — a fresh database gets 6, every later boot 0.
// Data is deliberately NOT a migration: migrations carry structure and are
// append-only forever; the seed carries content and must be re-runnable,
// which ON CONFLICT DO NOTHING makes it. The whole seed is one
// transaction: a half-seeded database is never observable.
func Seed(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, unavailable(err)
	}
	defer tx.Rollback(ctx)

	inserted := 0
	for i, draft := range seedPosts {
		key, ok := parseUUID(SeedPostIDs[i])
		if !ok {
			return 0, fmt.Errorf("seed id %q is not a canonical uuid", SeedPostIDs[i])
		}
		created := seedCreatedAt[i]
		tag, err := tx.Exec(ctx, `
			INSERT INTO posts (id, title, body, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)
			ON CONFLICT (id) DO NOTHING`,
			key, draft.Title, draft.Body, created)
		if err != nil {
			return 0, unavailable(err)
		}
		inserted += int(tag.RowsAffected())
		if err := attachTags(ctx, tx, key, uniqueTags(draft.Tags)); err != nil {
			return 0, unavailable(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, unavailable(err)
	}
	return inserted, nil
}

// unavailable maps every infrastructure failure to the typed
// service_unavailable error — the one honest bucket when the live store
// cannot answer. The cause rides along (%w) for the logs; the kind is
// what the error writer sees.
func unavailable(err error) error {
	return core.Wrapf(err, core.KindServiceUnavailable,
		"The live posts store (PostgreSQL) is not reachable right now.")
}

// errorsIsNoRows reports the driver's no-rows sentinel without leaking the
// pgx type into call sites.
func errorsIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// ── UUIDv7 minting and parsing ────────────────────────────────────────────
//
// The same RFC 9562 layout the uuid lab runs (48-bit Unix milliseconds, a
// 12-bit per-millisecond counter, the variant bits, 62 random bits). The
// lab's generator lives in internal/domain/demos, whose grpc exhibit
// imports this package — so the mint lives here independently rather than
// creating an import cycle; the layout constants are the standard's, not
// a fork of the lab's opinions.

const (
	uuidV7Version = 7
	uuidSeqMax    = 0x0FFF
)

// uuidMinter is the monotonic core: one mutex-guarded (millisecond,
// counter) pair, exactly the lab generator's reserve() contract.
type uuidMinter struct {
	mu     sync.Mutex
	random io.Reader
	now    func() time.Time
	lastMS int64
	seq    uint16
}

// defaultMinter is the process-wide minter: crypto/rand and the UTC wall
// clock. mintUUIDv7 is its single-call face.
var defaultMinter = &uuidMinter{
	random: rand.Reader,
	now:    func() time.Time { return time.Now().UTC() },
	lastMS: -1,
}

// mintUUIDv7 mints one canonical UUIDv7 string — the production id source
// for every post Create stores.
func mintUUIDv7() (string, error) {
	return defaultMinter.next()
}

// next mints one id: reserve the (millisecond, counter) pair, draw the 62
// random bits, lay the fields into RFC 9562's positions.
func (m *uuidMinter) next() (string, error) {
	unixMs, seq, err := m.reserve()
	if err != nil {
		return "", err
	}
	var randB [8]byte
	if _, err := io.ReadFull(m.random, randB[:]); err != nil {
		return "", fmt.Errorf("uuidv7 randomness: %w", err)
	}
	var u [16]byte
	putUint48(&u, unixMs)
	u[6] = byte(uuidV7Version<<4) | byte(seq>>8)
	u[7] = byte(seq)
	u[8] = 0b10<<6 | randB[0]&0x3F
	copy(u[9:], randB[1:])
	return formatUUID(u), nil
}

// reserve picks the (millisecond, counter) pair under the lock. A new
// millisecond seeds the counter randomly in [1, max] so same-millisecond
// ids stay unguessable; the same millisecond increments it; an exhausted
// counter waits for the next millisecond (an error on a stuck clock —
// loud beats wrong).
func (m *uuidMinter) reserve() (int64, uint16, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	unixMs := m.now().UnixMilli()
	switch {
	case unixMs > m.lastMS:
		var seed [1]byte
		if _, err := io.ReadFull(m.random, seed[:]); err != nil {
			return 0, 0, fmt.Errorf("uuidv7 counter seed: %w", err)
		}
		m.lastMS = unixMs
		m.seq = 1 + uint16(seed[0])%uuidSeqMax
		return unixMs, m.seq, nil
	case m.seq < uuidSeqMax:
		m.seq++
		return unixMs, m.seq, nil
	default:
		return 0, 0, fmt.Errorf("uuidv7 counter exhausted in millisecond %d with a non-advancing clock", unixMs)
	}
}

// putUint48 writes a 48-bit timestamp into bytes 0–5, big-endian.
func putUint48(u *[16]byte, ms int64) {
	for i := 5; i >= 0; i-- {
		u[i] = byte(ms)
		ms >>= 8
	}
}

// hexDigits is the lower-case hex alphabet of the canonical UUID form.
const hexDigits = "0123456789abcdef"

// formatUUID renders the canonical 8-4-4-4-12 lower-case hex form.
func formatUUID(u [16]byte) string {
	var out [36]byte
	pos := 0
	for _, octet := range u {
		if pos == 8 || pos == 13 || pos == 18 || pos == 23 {
			out[pos] = '-'
			pos++
		}
		out[pos] = hexDigits[octet>>4]
		out[pos+1] = hexDigits[octet&0x0F]
		pos += 2
	}
	return string(out[:])
}

// parseUUID parses the canonical 36-char form into its 16 bytes; anything
// else is not a post id this store has ever minted.
func parseUUID(s string) ([16]byte, bool) {
	var key [16]byte
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return key, false
	}
	pos := 0
	for i := 0; i < len(s); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		hi, ok := hexNibble(s[i])
		if !ok {
			return [16]byte{}, false
		}
		lo, ok := hexNibble(s[i+1])
		if !ok {
			return [16]byte{}, false
		}
		key[pos] = hi<<4 | lo
		pos++
		i++
	}
	return key, true
}

// hexNibble decodes one lower- or upper-case hex digit.
func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}
