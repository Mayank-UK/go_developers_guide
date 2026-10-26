// Topic 19 mongodb's live demo: the mongo console. One scripted run
// against the RUNNING stack's MongoDB carries the whole topic's teaching
// as data — the idempotent keyed seed, the application-layer refusal
// (validation rejects an event before any I/O), the query beats
// (filter/sort/limit, tag containment, projection), the explain pair
// (COLLSCAN vs IXSCAN), the TTL index's own spec, and the BSON type
// survey of one stored document. The driver work lives in
// internal/data/mongodb (the console runner behind the port below); this
// file owns the port's types and the pure payload builder, so the
// layering stays honest: data → domain, never the reverse.
package demos

import (
	"context"
	"sort"
)

// MongoConsoleRunner is the port the mongo-console demo reads through:
// one call runs the whole scripted console against the live store. The
// mongo-driver-backed implementation lives in internal/data/mongodb;
// tests and the handler wire fakes through this interface.
type MongoConsoleRunner interface {
	// RunConsole seeds (idempotently), runs the query and explain beats,
	// and reads the TTL index's spec back from the server.
	RunConsole(ctx context.Context) (MongoConsoleObservation, error)
}

// MongoConsoleObservation is the runner's raw haul: display-ready rows in
// the order the beats ran, carrying no driver types across the seam.
type MongoConsoleObservation struct {
	// Database and Collection name where the whole run happened.
	Database   string
	Collection string
	// Seed reports the idempotent keyed upserts that plant the world.
	Seed MongoSeedReport
	// Refusal shows the event the application layer refused — and why.
	Refusal MongoRefusalRow
	// Queries are the read beats: filter/sort/limit, containment, count,
	// and projection, each with the command and the documents it returned.
	Queries []MongoQueryBeat
	// Explains are the plan pair: one unindexed scan, one index walk.
	Explains []MongoExplainRow
	// TTL is the TTL index as the server itself states it.
	TTL MongoTTLShowcase
	// BSONTypes surveys one stored document's field types.
	BSONTypes []MongoBSONTypeRow
}

// MongoSeedReport is the seed step: what was planted, and the proof the
// planting is idempotent (re-runs match instead of insert).
type MongoSeedReport struct {
	SeedKeys       []string
	CountBefore    int64
	Upserted       int64
	TotalAfter     int64
	IndexesEnsured []string
}

// MongoRefusalRow is the validation beat: the draft that was attempted
// and every rule it broke — refused in the application layer, with zero
// I/O behind it.
type MongoRefusalRow struct {
	Attempted map[string]string
	Errors    []string
}

// MongoQueryBeat is one read beat: the operation, its mongosh-shaped
// command, the outcome sentence, and the documents it returned (each
// document a field→rendered-value row).
type MongoQueryBeat struct {
	Op          string
	Description string
	Command     string
	Outcome     string
	Documents   []map[string]string
}

// MongoExplainRow is one explain plan: the winning plan's scan stage,
// which index (if any) it walked, and the exam counts.
type MongoExplainRow struct {
	Label        string
	Query        string
	Stage        string
	IndexName    string
	KeysExamined int64
	DocsExamined int64
}

// MongoTTLShowcase is the TTL index read back from listIndexes: the key
// pattern, the window, and the sweeper's cadence.
type MongoTTLShowcase struct {
	IndexName          string
	Key                string
	ExpireAfterSeconds int64
}

// MongoBSONTypeRow is one field of the BSON type survey: the field's
// name, its BSON type, the value as it rode the wire, and the note that
// explains what Go type it round-trips through.
type MongoBSONTypeRow struct {
	Field    string
	BSONType string
	Value    string
	Note     string
}

// MongoConsolePayload is the wire shape of GET /api/demo/db/mongo-console
// (camelCase JSON). Deterministic for a seeded stack: beat order fixed,
// documents sorted newest-first, every slice non-nil so the wire renders
// [] — never null — and map rows marshal with sorted keys.
type MongoConsolePayload struct {
	Database   string                `json:"database"`
	Collection string                `json:"collection"`
	Seed       MongoConsoleSeed      `json:"seed"`
	Refusal    MongoConsoleRefusal   `json:"refusal"`
	Queries    []MongoConsoleQuery   `json:"queries"`
	Explains   []MongoConsoleExplain `json:"explains"`
	TTL        MongoConsoleTTL       `json:"ttl"`
	BSONTypes  []MongoConsoleBSONRow `json:"bsonTypes"`
}

// MongoConsoleSeed is the seed report on the wire.
type MongoConsoleSeed struct {
	SeedKeys       []string `json:"seedKeys"`
	CountBefore    int64    `json:"countBefore"`
	Upserted       int64    `json:"upserted"`
	TotalAfter     int64    `json:"totalAfter"`
	IndexesEnsured []string `json:"indexesEnsured"`
}

// MongoConsoleRefusal is the refusal beat on the wire.
type MongoConsoleRefusal struct {
	Attempted map[string]string `json:"attempted"`
	Errors    []string          `json:"errors"`
	Note      string            `json:"note"`
}

// MongoConsoleQuery is one read beat on the wire.
type MongoConsoleQuery struct {
	Op          string              `json:"op"`
	Description string              `json:"description"`
	Command     string              `json:"command"`
	Outcome     string              `json:"outcome"`
	Documents   []map[string]string `json:"documents"`
}

// MongoConsoleExplain is one explain row on the wire.
type MongoConsoleExplain struct {
	Label        string `json:"label"`
	Query        string `json:"query"`
	Stage        string `json:"stage"`
	IndexName    string `json:"indexName"`
	KeysExamined int64  `json:"keysExamined"`
	DocsExamined int64  `json:"docsExamined"`
}

// MongoConsoleTTL is the TTL showcase on the wire.
type MongoConsoleTTL struct {
	IndexName          string `json:"indexName"`
	Key                string `json:"key"`
	ExpireAfterSeconds int64  `json:"expireAfterSeconds"`
	SweeperNote        string `json:"sweeperNote"`
}

// MongoConsoleBSONRow is one BSON type-survey row on the wire.
type MongoConsoleBSONRow struct {
	Field    string `json:"field"`
	BSONType string `json:"bsonType"`
	Value    string `json:"value"`
	Note     string `json:"note"`
}

// BuildMongoConsolePayload shapes an observation into the demo payload:
// rows passed through in beat order, keys sorted, nil collections
// materialized as empty slices, and the two fixed teaching notes (the
// refusal's zero-IO note and the TTL sweeper's cadence) stamped once so
// every consumer reads the same words.
func BuildMongoConsolePayload(observation MongoConsoleObservation) MongoConsolePayload {
	queries := make([]MongoConsoleQuery, 0, len(observation.Queries))
	for _, beat := range observation.Queries {
		documents := make([]map[string]string, 0, len(beat.Documents))
		documents = append(documents, beat.Documents...)
		queries = append(queries, MongoConsoleQuery{
			Op:          beat.Op,
			Description: beat.Description,
			Command:     beat.Command,
			Outcome:     beat.Outcome,
			Documents:   documents,
		})
	}

	explains := make([]MongoConsoleExplain, 0, len(observation.Explains))
	for _, explain := range observation.Explains {
		explains = append(explains, MongoConsoleExplain{
			Label:        explain.Label,
			Query:        explain.Query,
			Stage:        explain.Stage,
			IndexName:    explain.IndexName,
			KeysExamined: explain.KeysExamined,
			DocsExamined: explain.DocsExamined,
		})
	}

	bsonTypes := make([]MongoConsoleBSONRow, 0, len(observation.BSONTypes))
	for _, row := range observation.BSONTypes {
		bsonTypes = append(bsonTypes, MongoConsoleBSONRow{
			Field:    row.Field,
			BSONType: row.BSONType,
			Value:    row.Value,
			Note:     row.Note,
		})
	}

	return MongoConsolePayload{
		Database:   observation.Database,
		Collection: observation.Collection,
		Seed: MongoConsoleSeed{
			SeedKeys:       sortedCopy(observation.Seed.SeedKeys),
			CountBefore:    observation.Seed.CountBefore,
			Upserted:       observation.Seed.Upserted,
			TotalAfter:     observation.Seed.TotalAfter,
			IndexesEnsured: sortedCopy(observation.Seed.IndexesEnsured),
		},
		Refusal: MongoConsoleRefusal{
			Attempted: nonNilMap(observation.Refusal.Attempted),
			Errors:    nonNilStrings(observation.Refusal.Errors),
			Note:      "Refused in the application layer — MongoDB never saw this event.",
		},
		Queries:  queries,
		Explains: explains,
		TTL: MongoConsoleTTL{
			IndexName:          observation.TTL.IndexName,
			Key:                observation.TTL.Key,
			ExpireAfterSeconds: observation.TTL.ExpireAfterSeconds,
			SweeperNote: "MongoDB's TTL monitor sweeps roughly every 60 seconds; this window is ten " +
				"years, so nothing vanishes mid-demo — the spec is the teaching, not an eviction.",
		},
		BSONTypes: bsonTypes,
	}
}

// sortedCopy returns the values sorted (defensive: the runner sends them
// sorted, the payload guarantees it) and never nil — reusing the package's
// nonNilStrings for the never-null half.
func sortedCopy(values []string) []string {
	copied := nonNilStrings(values)
	sort.Strings(copied)
	return copied
}

// nonNilMap materializes nil as an empty object for the same rule.
func nonNilMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}
