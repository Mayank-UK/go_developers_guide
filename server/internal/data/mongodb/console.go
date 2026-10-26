// The mongo-console demo runner: ONE type over the events adapter that
// plays the whole topic 19 script against the live MongoDB — the
// idempotent seed through the domain's own use case, the application-
// layer refusal (zero I/O), the query beats (filter/sort/limit, tag
// containment, count, projection), the explain pair (COLLSCAN vs a
// hinted IXSCAN), the TTL index's spec read back from listIndexes, and
// the BSON type survey. Every payload value is deterministic for a
// seeded stack: the server assigns each document's _id once at first
// insert and re-upserts never touch it, so re-runs produce the same
// documents, counts, ids, and plans.
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"guide/internal/core"
	"guide/internal/domain/demos"
	"guide/internal/domain/events"
)

// Compile-time proof the runner satisfies the demo port.
var _ demos.MongoConsoleRunner = (*ConsoleRunner)(nil)

// ConsoleRunner runs the mongo-console demo over the events adapter.
type ConsoleRunner struct {
	store  *EventStore
	record *events.RecordEventUseCase
}

// NewConsoleRunner wires the runner over an already-ensured EventStore.
func NewConsoleRunner(store *EventStore) *ConsoleRunner {
	return &ConsoleRunner{store: store, record: events.NewRecordEventUseCase(store)}
}

// seedEvents is the demo's fixed world: eight events across the guide's
// January–March 2026 timeline — three deploys, two alerts, one scaling
// move, two backups — with keys, tags, and instants chosen so every query
// beat below has a stable, story-shaped answer.
var seedEvents = []events.Draft{
	{Key: "deploy-api-0126", Type: events.TypeDeploy, Severity: events.SeverityInfo,
		Source: "ci", Message: "api image 1.4.0 rolled out", Tags: []string{"deploy", "prod"},
		OccurredAt: time.Date(2026, 1, 26, 10, 4, 0, 0, time.UTC)},
	{Key: "alert-latency-0127", Type: events.TypeAlert, Severity: events.SeverityWarning,
		Source: "grafana", Message: "p95 latency above 400ms for five minutes", Tags: []string{"alert", "latency", "prod"},
		OccurredAt: time.Date(2026, 1, 27, 9, 15, 0, 0, time.UTC)},
	{Key: "deploy-web-0210", Type: events.TypeDeploy, Severity: events.SeverityInfo,
		Source: "ci", Message: "web image 2.1.0 rolled out", Tags: []string{"deploy", "staging"},
		OccurredAt: time.Date(2026, 2, 10, 16, 42, 0, 0, time.UTC)},
	{Key: "backup-nightly-0215", Type: events.TypeBackup, Severity: events.SeverityInfo,
		Source: "cron", Message: "nightly snapshot uploaded", Tags: []string{"backup"},
		OccurredAt: time.Date(2026, 2, 15, 2, 0, 0, 0, time.UTC)},
	{Key: "alert-disk-0221", Type: events.TypeAlert, Severity: events.SeverityCritical,
		Source: "node-exporter", Message: "disk usage 92% on db-2", Tags: []string{"alert", "disk", "prod"},
		OccurredAt: time.Date(2026, 2, 21, 4, 30, 0, 0, time.UTC)},
	{Key: "scaling-web-0305", Type: events.TypeScaling, Severity: events.SeverityInfo,
		Source: "autoscaler", Message: "web replicas 3 to 6", Tags: []string{"scaling", "prod"},
		OccurredAt: time.Date(2026, 3, 5, 11, 20, 0, 0, time.UTC)},
	{Key: "deploy-api-0309", Type: events.TypeDeploy, Severity: events.SeverityInfo,
		Source: "ci", Message: "api image 1.5.1 rolled out", Tags: []string{"deploy", "prod"},
		OccurredAt: time.Date(2026, 3, 9, 8, 55, 0, 0, time.UTC)},
	{Key: "backup-nightly-0312", Type: events.TypeBackup, Severity: events.SeverityInfo,
		Source: "cron", Message: "nightly snapshot uploaded", Tags: []string{"backup"},
		OccurredAt: time.Date(2026, 3, 12, 2, 0, 0, 0, time.UTC)},
}

// RunConsole executes the scripted beats in teaching order and returns
// the observation the payload builder shapes.
func (r *ConsoleRunner) RunConsole(ctx context.Context) (demos.MongoConsoleObservation, error) {
	seed, err := r.seed(ctx)
	if err != nil {
		return demos.MongoConsoleObservation{}, err
	}
	refusal, err := r.refusal(ctx)
	if err != nil {
		return demos.MongoConsoleObservation{}, err
	}
	queries, err := r.queryBeats(ctx)
	if err != nil {
		return demos.MongoConsoleObservation{}, err
	}
	explains, err := r.explainBeats(ctx)
	if err != nil {
		return demos.MongoConsoleObservation{}, err
	}
	ttl, err := r.ttlShowcase(ctx)
	if err != nil {
		return demos.MongoConsoleObservation{}, err
	}
	bsonTypes, err := r.bsonSurvey(ctx)
	if err != nil {
		return demos.MongoConsoleObservation{}, err
	}

	return demos.MongoConsoleObservation{
		Database:   r.store.collection.Database().Name(),
		Collection: r.store.collection.Name(),
		Seed:       seed,
		Refusal:    refusal,
		Queries:    queries,
		Explains:   explains,
		TTL:        ttl,
		BSONTypes:  bsonTypes,
	}, nil
}

// seed replants the fixed world through the domain's own use case — every
// draft passes the same validation a client write would — and derives the
// idempotency proof from counts: upserted is what this run added, matched
// is what was already there and got replaced in place.
func (r *ConsoleRunner) seed(ctx context.Context) (demos.MongoSeedReport, error) {
	before, err := r.store.Count(ctx, events.Query{Limit: events.MaxQueryLimit})
	if err != nil {
		return demos.MongoSeedReport{}, err
	}

	keys := make([]string, 0, len(seedEvents))
	for _, draft := range seedEvents {
		if _, err := r.record.Execute(ctx, draft); err != nil {
			return demos.MongoSeedReport{}, fmt.Errorf("seed %s: %w", draft.Key, err)
		}
		keys = append(keys, draft.Key)
	}

	after, err := r.store.Count(ctx, events.Query{Limit: events.MaxQueryLimit})
	if err != nil {
		return demos.MongoSeedReport{}, err
	}
	upserted := after - before
	if upserted < 0 {
		upserted = 0
	}
	return demos.MongoSeedReport{
		SeedKeys:       keys,
		CountBefore:    before,
		Upserted:       upserted,
		TotalAfter:     after,
		IndexesEnsured: []string{keyIndexName, queryIndexName, ttlIndexName},
	}, nil
}

// refusal attempts one deliberately invalid draft through the same use
// case and captures the judgment — the store is never consulted, which is
// the point the payload's note states and the events package's tests pin.
func (r *ConsoleRunner) refusal(ctx context.Context) (demos.MongoRefusalRow, error) {
	draft := events.Draft{
		Key:        "NOT A KEYPATTERN!",
		Type:       "restart",
		Severity:   "sev-1",
		Source:     "",
		Message:    "",
		Tags:       []string{"prod"},
		OccurredAt: time.Date(2026, 3, 14, 7, 30, 0, 0, time.UTC),
	}
	_, err := r.record.Execute(ctx, draft)
	if err == nil {
		return demos.MongoRefusalRow{}, errors.New("the invalid seed draft must be refused")
	}

	var appErr *core.Error
	if !errors.As(err, &appErr) || appErr.Kind != core.KindValidationFailed {
		return demos.MongoRefusalRow{}, fmt.Errorf("refusal must be the typed 422, got %w", err)
	}
	issues, _ := appErr.Details.([]events.Issue)
	errorsList := make([]string, 0, len(issues))
	for _, issue := range issues {
		errorsList = append(errorsList, fmt.Sprintf("%s — %s (got %s)", issue.Field, issue.Rule, issue.Got))
	}
	if len(errorsList) == 0 {
		errorsList = append(errorsList, appErr.Message)
	}

	return demos.MongoRefusalRow{
		Attempted: map[string]string{
			"key": draft.Key, "type": draft.Type, "severity": draft.Severity,
			"source": "(empty)", "message": "(empty)",
		},
		Errors: errorsList,
	}, nil
}

// queryBeats runs the read path three ways through the domain port, then
// the projection beat at the adapter level (projection is a storage-level
// read shape, so it lives here, not on the domain port).
func (r *ConsoleRunner) queryBeats(ctx context.Context) ([]demos.MongoQueryBeat, error) {
	beats := make([]demos.MongoQueryBeat, 0, 4)

	newestDeploys, err := r.store.Recent(ctx, events.Query{Type: events.TypeDeploy, Limit: 2})
	if err != nil {
		return nil, err
	}
	beats = append(beats, demos.MongoQueryBeat{
		Op:          "find",
		Description: "Filter, sort, limit: the compound index's exact shape — type equality first, occurredAt descending after.",
		Command:     `db.events.find({type: "deploy"}).sort({occurredAt: -1}).limit(2)`,
		Outcome:     fmt.Sprintf("%d of the deploy events, newest first", len(newestDeploys)),
		Documents:   eventRows(newestDeploys),
	})

	prodEvents, err := r.store.Recent(ctx, events.Query{Tag: "prod", Limit: 10})
	if err != nil {
		return nil, err
	}
	beats = append(beats, demos.MongoQueryBeat{
		Op:          "find",
		Description: "Containment: matching one value inside the tags array — no join table, no second query.",
		Command:     `db.events.find({tags: "prod"}).sort({occurredAt: -1}).limit(10)`,
		Outcome:     fmt.Sprintf("%d events carry the prod tag, newest first", len(prodEvents)),
		Documents:   eventRows(prodEvents),
	})

	deployCount, err := r.store.Count(ctx, events.Query{Type: events.TypeDeploy})
	if err != nil {
		return nil, err
	}
	beats = append(beats, demos.MongoQueryBeat{
		Op:          "count",
		Description: "Count answers from the same filter without materializing documents.",
		Command:     `db.events.countDocuments({type: "deploy"})`,
		Outcome:     fmt.Sprintf("%d deploy events", deployCount),
		Documents:   []map[string]string{},
	})

	projected, err := r.projectionBeat(ctx)
	if err != nil {
		return nil, err
	}
	beats = append(beats, projected)

	return beats, nil
}

// projectionDocument is the partial read: three fields, no _id. What the
// projection leaves out never crosses the wire — that is the whole point
// of the beat.
type projectionDocument struct {
	Key        string    `bson:"evtKey"`
	Severity   string    `bson:"severity"`
	OccurredAt time.Time `bson:"occurredAt"`
}

// projectionBeat runs the adapter-level partial read: same filter as the
// first beat, three fields projected, _id explicitly excluded (the
// projection whitelist would otherwise keep it).
func (r *ConsoleRunner) projectionBeat(ctx context.Context) (demos.MongoQueryBeat, error) {
	cursor, err := r.store.collection.Find(
		ctx,
		bson.D{{Key: "type", Value: events.TypeDeploy}},
		options.Find().
			SetProjection(bson.D{
				{Key: "evtKey", Value: 1}, {Key: "severity", Value: 1}, {Key: "occurredAt", Value: 1},
				{Key: "_id", Value: 0},
			}).
			SetSort(bson.D{{Key: "occurredAt", Value: -1}}),
	)
	if err != nil {
		return demos.MongoQueryBeat{}, unavailable(err)
	}
	defer cursor.Close(ctx)

	documents := []map[string]string{}
	for cursor.Next(ctx) {
		var document projectionDocument
		if err := cursor.Decode(&document); err != nil {
			return demos.MongoQueryBeat{}, unavailable(err)
		}
		documents = append(documents, map[string]string{
			"evtKey":     document.Key,
			"severity":   document.Severity,
			"occurredAt": document.OccurredAt.UTC().Format(time.RFC3339),
		})
	}
	if err := cursor.Err(); err != nil {
		return demos.MongoQueryBeat{}, unavailable(err)
	}

	return demos.MongoQueryBeat{
		Op:          "find+project",
		Description: "Projection: three fields, _id excluded — the documents that come back are partial, and what is left out never crosses the wire.",
		Command:     `db.events.find({type: "deploy"}, {evtKey: 1, severity: 1, occurredAt: 1, _id: 0})`,
		Outcome:     fmt.Sprintf("%d partial documents — no _id, no tags, no message", len(documents)),
		Documents:   documents,
	}, nil
}

// explainStage is one node of the winning plan tree; the walk below finds
// the first scan stage in it.
type explainStage struct {
	Stage       string         `bson:"stage"`
	IndexName   string         `bson:"indexName"`
	InputStage  *explainStage  `bson:"inputStage"`
	InputStages []explainStage `bson:"inputStages"`
}

// explainOutcome is the shape of an explain command's interesting half.
type explainOutcome struct {
	QueryPlanner struct {
		WinningPlan explainStage `bson:"winningPlan"`
	} `bson:"queryPlanner"`
	ExecutionStats struct {
		TotalKeysExamined int64 `bson:"totalKeysExamined"`
		TotalDocsExamined int64 `bson:"totalDocsExamined"`
	} `bson:"executionStats"`
}

// explainBeats explains one unindexed query (the planner has no choice:
// COLLSCAN) and one indexed query with the index hinted — on eight
// documents the planner may honestly prefer a scan, so the hint pins the
// plan the lesson is about. Execution verbosity supplies the exam counts.
func (r *ConsoleRunner) explainBeats(ctx context.Context) ([]demos.MongoExplainRow, error) {
	rows := []demos.MongoExplainRow{
		{
			Label: "no index on severity — the planner scans",
			Query: `find({severity: "critical"})`,
		},
		{
			Label: "compound index, hinted — the planner walks the index",
			Query: `find({type: "deploy"}).sort({occurredAt: -1}).hint({type: 1, occurredAt: -1})`,
		},
	}
	commands := []bson.D{
		{ // explain #1 — the unindexed severity filter
			{Key: "explain", Value: bson.D{
				{Key: "find", Value: EventsCollection},
				{Key: "filter", Value: bson.D{{Key: "severity", Value: events.SeverityCritical}}},
			}},
			{Key: "verbosity", Value: "executionStats"},
		},
		{ // explain #2 — the indexed type filter, sort, and hint
			{Key: "explain", Value: bson.D{
				{Key: "find", Value: EventsCollection},
				{Key: "filter", Value: bson.D{{Key: "type", Value: events.TypeDeploy}}},
				{Key: "sort", Value: bson.D{{Key: "occurredAt", Value: -1}}},
				{Key: "hint", Value: bson.D{{Key: "type", Value: 1}, {Key: "occurredAt", Value: -1}}},
			}},
			{Key: "verbosity", Value: "executionStats"},
		},
	}

	for i, command := range commands {
		var outcome explainOutcome
		if err := r.store.collection.Database().RunCommand(ctx, command).Decode(&outcome); err != nil {
			return nil, unavailable(err)
		}
		stage := findScanStage(outcome.QueryPlanner.WinningPlan)
		rows[i].Stage = stage.Stage
		rows[i].IndexName = stage.IndexName
		rows[i].KeysExamined = outcome.ExecutionStats.TotalKeysExamined
		rows[i].DocsExamined = outcome.ExecutionStats.TotalDocsExamined
	}
	return rows, nil
}

// findScanStage walks the winning plan depth-first for the first COLLSCAN
// or IXSCAN node — the access decision the exhibit teaches.
func findScanStage(stage explainStage) explainStage {
	switch stage.Stage {
	case "COLLSCAN", "IXSCAN":
		return stage
	}
	if stage.InputStage != nil {
		if found := findScanStage(*stage.InputStage); found.Stage != "" {
			return found
		}
	}
	for _, nested := range stage.InputStages {
		if found := findScanStage(nested); found.Stage != "" {
			return found
		}
	}
	return explainStage{}
}

// ttlShowcase reads the TTL index's own spec back from listIndexes — the
// server's statement of the key pattern and the window, not the
// constant the adapter sent.
func (r *ConsoleRunner) ttlShowcase(ctx context.Context) (demos.MongoTTLShowcase, error) {
	cursor, err := r.store.collection.Indexes().List(ctx)
	if err != nil {
		return demos.MongoTTLShowcase{}, unavailable(err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var spec struct {
			Name               string `bson:"name"`
			Key                bson.D `bson:"key"`
			ExpireAfterSeconds *int64 `bson:"expireAfterSeconds"`
		}
		if err := cursor.Decode(&spec); err != nil {
			return demos.MongoTTLShowcase{}, unavailable(err)
		}
		if spec.ExpireAfterSeconds == nil {
			continue
		}
		return demos.MongoTTLShowcase{
			IndexName:          spec.Name,
			Key:                renderKeyPattern(spec.Key),
			ExpireAfterSeconds: *spec.ExpireAfterSeconds,
		}, nil
	}
	if err := cursor.Err(); err != nil {
		return demos.MongoTTLShowcase{}, unavailable(err)
	}
	return demos.MongoTTLShowcase{}, errors.New("the events collection has no TTL index")
}

// renderKeyPattern renders a key document the way mongosh shows it:
// "{field: direction, …}".
func renderKeyPattern(key bson.D) string {
	parts := make([]string, 0, len(key))
	for _, entry := range key {
		parts = append(parts, fmt.Sprintf("%s: %v", entry.Key, entry.Value))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// bsonSurvey reads one stored document back and walks its fields' BSON
// types — the marshaling story (ObjectIds, dates, arrays) as the wire
// actually carried it. The newest deploy is the exhibit.
func (r *ConsoleRunner) bsonSurvey(ctx context.Context) ([]demos.MongoBSONTypeRow, error) {
	event, err := r.store.FindByKey(ctx, "deploy-api-0309")
	if err != nil {
		return nil, err
	}
	created, err := bson.ObjectIDFromHex(event.ID)
	if err != nil {
		return nil, unavailable(fmt.Errorf("decode the stored object id: %w", err))
	}

	return []demos.MongoBSONTypeRow{
		{Field: "_id", BSONType: "objectId", Value: event.ID,
			Note: fmt.Sprintf("assigned by the server at the first insert — the first four bytes ARE that instant (%s), and re-upserts keep it stable",
				created.Timestamp().UTC().Format(time.RFC3339))},
		{Field: "evtKey", BSONType: "string", Value: event.Key,
			Note: "the business key the unique index guards — upserts address it, not the _id"},
		{Field: "type", BSONType: "string", Value: event.Type,
			Note: "closed vocabulary judged in the application layer — the store enforces no enum"},
		{Field: "occurredAt", BSONType: "date", Value: event.OccurredAt.Format(time.RFC3339),
			Note: "BSON dates are int64 milliseconds since the epoch; Go time.Time round-trips through them to the millisecond"},
		{Field: "tags", BSONType: "array", Value: "[" + strings.Join(event.Tags, " ") + "]",
			Note: "one field holds the whole set — the containment query rides it with no join table"},
	}, nil
}

// eventRows renders events as display rows: field → string, every value
// present so the panel never guesses at absence.
func eventRows(list []events.Event) []map[string]string {
	rows := make([]map[string]string, 0, len(list))
	for _, event := range list {
		rows = append(rows, map[string]string{
			"_id":        event.ID,
			"evtKey":     event.Key,
			"type":       event.Type,
			"severity":   event.Severity,
			"source":     event.Source,
			"message":    event.Message,
			"tags":       "[" + strings.Join(event.Tags, " ") + "]",
			"occurredAt": event.OccurredAt.Format(time.RFC3339),
		})
	}
	return rows
}

// UnavailableConsoleRunner is the optional-service fallback: when
// MongoDB is down at boot, the composition root wires the demo endpoint
// over this runner instead, and every call degrades to the typed
// service_unavailable error carrying the boot-time cause. The endpoint
// serves the honest 503 envelope; the process never crashes; every other
// topic keeps working.
type UnavailableConsoleRunner struct {
	cause error
}

// NewUnavailableConsoleRunner keeps the boot-time failure so callers see
// exactly why the store is unavailable.
func NewUnavailableConsoleRunner(cause error) *UnavailableConsoleRunner {
	return &UnavailableConsoleRunner{cause: cause}
}

// RunConsole implements the demo port by refusing, honestly and typed.
func (r *UnavailableConsoleRunner) RunConsole(context.Context) (demos.MongoConsoleObservation, error) {
	return demos.MongoConsoleObservation{}, unavailable(r.cause)
}

// Compile-time proof the fallback satisfies the same port.
var _ demos.MongoConsoleRunner = (*UnavailableConsoleRunner)(nil)
