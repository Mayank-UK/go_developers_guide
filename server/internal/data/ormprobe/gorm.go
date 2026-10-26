// The GORM half of the orm-probe: models mapping the SAME ormprobe_*
// tables the sqlc schema declares (one schema, two mappings — that is the
// comparison being honest), the session-logger statement counter that is
// GORM's observability seam, the lazy and eager tag-loading shapes, and
// the two-repository unit of work the exhibit deliberately rolls back.
package ormprobe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"guide/internal/domain/demos"
)

// Post is the GORM model for ormprobe_posts. TableName() overrides the
// naming convention — GORM would otherwise look for a "posts" table,
// which is the guide's REAL one — and the Tags association declares the
// many-to-many edge explicitly, join columns included, so the mapping
// states the composite natural key rather than hoping the convention
// guesses it.
type Post struct {
	ID        int64 `gorm:"primaryKey"`
	Title     string
	Body      string
	CreatedAt time.Time
	Tags      []Tag `gorm:"many2many:ormprobe_post_tags;joinForeignKey:post_id;joinReferences:tag_name"`
}

// BeforeCreate is a GORM hook: called in-statement before every insert,
// it normalizes the model before the SQL runs — and because hooks ride
// the same transaction as the insert, their rejections unwind it whole.
func (p *Post) BeforeCreate(_ *gorm.DB) error {
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" {
		return errors.New("ormprobe: a post needs a title")
	}
	return nil
}

// Tag is the GORM model for the natural-keyed ormprobe_tags.
type Tag struct {
	Name string `gorm:"primaryKey"`
}

// PostTagLink is the join table as its own model, so edge writes can be
// explicit upserts rather than association magic.
type PostTagLink struct {
	PostID  int64  `gorm:"primaryKey"`
	TagName string `gorm:"primaryKey"`
}

// TableName overrides keep every model on the probe's prefixed tables.
func (Post) TableName() string        { return "ormprobe_posts" }
func (Tag) TableName() string         { return "ormprobe_tags" }
func (PostTagLink) TableName() string { return "ormprobe_post_tags" }

// sessionCounter is a gorm.Logger that stays silent but COUNTS: every
// statement a session issues passes Trace exactly once, which is the
// GORM-side counting seam the probe reads (and the tests cross-check
// against the pool tracer). Statement SQL is kept, whitespace-normalized,
// as the receipts.
type sessionCounter struct {
	logger.Interface
	mu         sync.Mutex
	statements []string
}

// newSessionCounter builds a counter over the silent default logger, so
// nothing the probe measures ever reaches GORM's log output.
func newSessionCounter() *sessionCounter {
	return &sessionCounter{Interface: logger.Default.LogMode(logger.Silent)}
}

// Trace implements gorm.Logger: record the statement, print nothing.
// (v1.31's shape: fc returns the SQL and the affected-row count, and the
// statement's error arrives as the last argument.)
func (c *sessionCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	statement := normalizeSQL(sql)
	c.mu.Lock()
	c.statements = append(c.statements, statement)
	c.mu.Unlock()
}

// count reports how many statements the session issued.
func (c *sessionCounter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.statements)
}

// samples copies the recorded statements — the receipts.
func (c *sessionCounter) samples() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.statements...)
}

// openGORM points GORM at the probe's pool through pgx's database/sql
// adapter — one pool, three APIs, no second connection source. The
// automatic ping is disabled: the pool was already proven by
// openCountedPool, and GORM's version probe is setup noise no measurement
// wants in its window.
func openGORM(sqlDB *sql.DB) (*gorm.DB, error) {
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger:               logger.Default.LogMode(logger.Silent),
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm over the probe's pool: %w", err)
	}
	return gormDB, nil
}

// runGormLazy is the N+1 exhibit: find the page, then resolve each post's
// tags PER ROW through the association API. GORM has no transparent lazy
// loading (struct fields are not proxies in Go) — this explicit loop is
// the shape lazy loading takes here, and it costs one statement per post
// on top of the page query: 1 + N.
func runGormLazy(ctx context.Context, db *gorm.DB) ([]probeRow, demos.ORMProbeTrialFacts, error) {
	counter := newSessionCounter()
	session := db.WithContext(ctx).Session(&gorm.Session{Logger: counter})
	started := time.Now()

	var posts []Post
	if err := session.Order("id").Find(&posts).Error; err != nil {
		return nil, demos.ORMProbeTrialFacts{}, fmt.Errorf("gorm: load the posts page: %w", err)
	}
	for i := range posts {
		if err := session.Model(&posts[i]).Association("Tags").Find(&posts[i].Tags); err != nil {
			return nil, demos.ORMProbeTrialFacts{}, fmt.Errorf("gorm: load tags for post %d: %w", posts[i].ID, err)
		}
	}

	return gormProbeRows(posts), demos.ORMProbeTrialFacts{
		Strategy:         demos.ORMProbeStrategyGormLazy,
		Statements:       counter.count(),
		Rows:             len(posts),
		DurationMicros:   time.Since(started).Microseconds(),
		SampleStatements: counter.samples(),
	}, nil
}

// runGormEager is the structural fix: Preload resolves the association
// for the whole page in one IN query after the page query — two
// statements total, regardless of how many posts the page carries.
func runGormEager(ctx context.Context, db *gorm.DB) ([]probeRow, demos.ORMProbeTrialFacts, error) {
	counter := newSessionCounter()
	session := db.WithContext(ctx).Session(&gorm.Session{Logger: counter})
	started := time.Now()

	var posts []Post
	if err := session.Preload("Tags").Order("id").Find(&posts).Error; err != nil {
		return nil, demos.ORMProbeTrialFacts{}, fmt.Errorf("gorm: preload the posts page: %w", err)
	}

	return gormProbeRows(posts), demos.ORMProbeTrialFacts{
		Strategy:         demos.ORMProbeStrategyGormEager,
		Statements:       counter.count(),
		Rows:             len(posts),
		DurationMicros:   time.Since(started).Microseconds(),
		SampleStatements: counter.samples(),
	}, nil
}

// GormPostsRepository is the posts resource's repository over a *gorm.DB
// handle — the exact port-and-adapter shape the guide uses for pgx, one
// resource per repository.
type GormPostsRepository struct {
	db *gorm.DB
}

// Create inserts a post; the BeforeCreate hook fires inside the statement.
func (r *GormPostsRepository) Create(ctx context.Context, post *Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

// GetByID fetches one post by primary key.
func (r *GormPostsRepository) GetByID(ctx context.Context, id int64) (Post, error) {
	var post Post
	if err := r.db.WithContext(ctx).First(&post, id).Error; err != nil {
		return Post{}, err
	}
	return post, nil
}

// GormTagsRepository owns the tag vocabulary and its edges.
type GormTagsRepository struct {
	db *gorm.DB
}

// Attach upserts the named tags and their edges to one post, ignoring
// pairs that already exist — the same ON CONFLICT DO NOTHING discipline
// the raw seed uses, spelled as a GORM clause.
func (r *GormTagsRepository) Attach(ctx context.Context, postID int64, names []string) error {
	tags := make([]Tag, len(names))
	for i, name := range names {
		tags[i] = Tag{Name: name}
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).Create(&tags).Error; err != nil {
		return err
	}
	links := make([]PostTagLink, len(names))
	for i, name := range names {
		links[i] = PostTagLink{PostID: postID, TagName: name}
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).Create(&links).Error
}

// runUnitOfWork is the transactions-across-repositories exhibit: two
// repositories constructed over ONE transaction — post create (the hook
// fires in-statement), tag attach, read-back inside the unit — then a
// DELIBERATE failure, and the post counts before and after prove every
// write unwound together. The scratch post's identity value may drift
// across runs (sequences do not roll back); nothing else about the
// exhibit does.
func runUnitOfWork(ctx context.Context, db *gorm.DB) (demos.ORMProbeUnitOfWorkFacts, error) {
	postsBefore, err := countGormPosts(ctx, db)
	if err != nil {
		return demos.ORMProbeUnitOfWorkFacts{}, err
	}

	facts := demos.ORMProbeUnitOfWorkFacts{
		Repositories: []string{"GormPostsRepository", "GormTagsRepository"},
		PostsBefore:  postsBefore,
		Steps: []demos.ORMProbeStepFacts{
			{Action: "begin", Detail: "db.Transaction opens one unit of work over the probe's pool"},
		},
	}
	counter := newSessionCounter()
	txErr := db.WithContext(ctx).Session(&gorm.Session{Logger: counter}).Transaction(func(tx *gorm.DB) error {
		posts := &GormPostsRepository{db: tx}
		tags := &GormTagsRepository{db: tx}

		scratch := &Post{
			Title: "  unit-of-work scratch  ",
			Body:  "two repositories, one transaction, one deliberate failure",
		}
		if err := posts.Create(ctx, scratch); err != nil {
			return fmt.Errorf("create the scratch post: %w", err)
		}
		if err := tags.Attach(ctx, scratch.ID, []string{"scratch"}); err != nil {
			return fmt.Errorf("attach the scratch tag: %w", err)
		}
		observed, err := posts.GetByID(ctx, scratch.ID)
		if err != nil {
			return fmt.Errorf("read the scratch post back inside the unit: %w", err)
		}
		facts.HookApplied = observed.Title == "unit-of-work scratch"
		facts.Steps = append(facts.Steps, demos.ORMProbeStepFacts{
			Action: "read back inside the unit",
			Detail: fmt.Sprintf("title stored as %q — the BeforeCreate hook trimmed the padded draft", observed.Title),
		})
		return errors.New("deliberate failure: prove the whole unit unwinds")
	})
	if txErr == nil {
		return demos.ORMProbeUnitOfWorkFacts{}, errors.New(
			"ormprobe: the unit of work was expected to roll back, but the transaction reported success")
	}
	facts.RolledBack = true
	postsAfter, err := countGormPosts(ctx, db)
	if err != nil {
		return demos.ORMProbeUnitOfWorkFacts{}, err
	}
	facts.PostsAfter = postsAfter
	facts.Steps = append(facts.Steps,
		demos.ORMProbeStepFacts{Action: "rollback", Detail: txErr.Error()},
		demos.ORMProbeStepFacts{
			Action: "count after",
			Detail: fmt.Sprintf("%d posts before, %d after — nothing leaked", facts.PostsBefore, postsAfter),
		},
	)
	return facts, nil
}

// countGormPosts counts the dataset through GORM itself.
func countGormPosts(ctx context.Context, db *gorm.DB) (int, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&Post{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("gorm: count the probe posts: %w", err)
	}
	return int(count), nil
}

// gormProbeRows normalizes GORM's models into the strategy-neutral shape
// the apples-to-apples comparison reads.
func gormProbeRows(posts []Post) []probeRow {
	rows := make([]probeRow, 0, len(posts))
	for _, post := range posts {
		names := make([]string, 0, len(post.Tags))
		for _, tag := range post.Tags {
			names = append(names, tag.Name)
		}
		rows = append(rows, probeRow{ID: post.ID, Title: post.Title, Tags: names})
	}
	return rows
}
