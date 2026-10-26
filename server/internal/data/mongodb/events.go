// The events.Store adapter: mongo-driver v2 over the events collection.
// The BSON mapping (the document struct below) is this file's private
// business — documents never leak past the repository implementation, the
// dependency rule the whole guide keeps. Every driver failure maps to the
// typed service_unavailable error; the unique index on evtKey is what
// makes Upsert idempotent; the TTL index is the topic's expiry story with
// a deliberately LONG window so nothing vanishes mid-demo.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"guide/internal/core"
	"guide/internal/domain/events"
)

// Compile-time proof the adapter satisfies the domain port.
var _ events.Store = (*EventStore)(nil)

// EventsCollection is the collection name inside whatever database the
// composition root chose for the demo stack.
const EventsCollection = "events"

// TTLSeconds is the events collection's expiry window: ten years. A real
// retention policy picks its own number; the guide picks one long enough
// that the TTL monitor never evicts a demo document mid-lesson — the SPEC
// is the teaching, not an eviction.
const TTLSeconds int32 = 10 * 365 * 24 * 60 * 60

// Index names, pinned so the demo payload and the tests can speak them.
const (
	keyIndexName   = "evtKey_1"
	queryIndexName = "type_1_occurredAt_-1"
	ttlIndexName   = "occurredAt_1"
)

// EventStore is the MongoDB adapter behind events.Store.
type EventStore struct {
	collection *mongo.Collection
}

// NewEventStore wires the adapter over an already-connected client and
// ensures the collection's indexes — idempotent CreateOne calls, safe to
// run at every boot. An unreachable MongoDB surfaces here as the typed
// service_unavailable error; the caller decides whether that is fatal
// (for this store's consumers it never is — Mongo is optional).
func NewEventStore(ctx context.Context, client *mongo.Client, database string) (*EventStore, error) {
	collection := client.Database(database).Collection(EventsCollection)
	if err := ensureEventIndexes(ctx, collection); err != nil {
		return nil, err
	}
	return &EventStore{collection: collection}, nil
}

// ensureEventIndexes creates the collection's three indexes, in place and
// idempotent: the unique business key (the upsert address), the compound
// query index (type equality + newest-first), and the TTL index with its
// long window. Each name is pinned so a re-run with different options
// fails loudly instead of silently leaving a twin.
func ensureEventIndexes(ctx context.Context, collection *mongo.Collection) error {
	indexes := []struct {
		name  string
		model mongo.IndexModel
	}{
		{keyIndexName, mongo.IndexModel{
			Keys:    bson.D{{Key: "evtKey", Value: 1}},
			Options: options.Index().SetUnique(true).SetName(keyIndexName),
		}},
		{queryIndexName, mongo.IndexModel{
			Keys:    bson.D{{Key: "type", Value: 1}, {Key: "occurredAt", Value: -1}},
			Options: options.Index().SetName(queryIndexName),
		}},
		{ttlIndexName, mongo.IndexModel{
			Keys:    bson.D{{Key: "occurredAt", Value: 1}},
			Options: options.Index().SetName(ttlIndexName).SetExpireAfterSeconds(TTLSeconds),
		}},
	}
	for _, index := range indexes {
		if _, err := collection.Indexes().CreateOne(ctx, index.model); err != nil {
			return unavailable(fmt.Errorf("ensure index %s: %w", index.name, err))
		}
	}
	return nil
}

// eventDocument is the BSON shape — the adapter's private mapping. Field
// names are the wire the collection speaks (camelCase, the Mongo
// convention); _id is minted from the event's own instant so the whole
// stored form is deterministic across re-upserts.
type eventDocument struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	Key        string        `bson:"evtKey"`
	Type       string        `bson:"type"`
	Severity   string        `bson:"severity"`
	Source     string        `bson:"source"`
	Message    string        `bson:"message"`
	Tags       []string      `bson:"tags,omitempty"`
	OccurredAt time.Time     `bson:"occurredAt"`
}

// Upsert replaces the document at evtKey (inserting when absent) and
// returns the stored form read back from the database — the same
// "return what the database states, not what the app assumed" discipline
// the PostgreSQL adapter's RETURNING clauses keep. The replacement
// deliberately carries NO _id: _id is immutable, so a replacement that
// set one could never update an existing document. On the insert path
// the server assigns it once — the driver-side ObjectId — and every
// later replacement leaves it alone, which is exactly the stability
// re-runs want.
func (s *EventStore) Upsert(ctx context.Context, event events.Event) (events.Event, error) {
	if _, err := s.collection.ReplaceOne(
		ctx,
		bson.D{{Key: "evtKey", Value: event.Key}},
		eventDocument{
			Key:        event.Key,
			Type:       event.Type,
			Severity:   event.Severity,
			Source:     event.Source,
			Message:    event.Message,
			Tags:       event.Tags,
			OccurredAt: event.OccurredAt,
		},
		options.Replace().SetUpsert(true),
	); err != nil {
		return events.Event{}, unavailable(err)
	}
	return s.FindByKey(ctx, event.Key)
}

// FindByKey returns the stored event at a business key, or the typed 404.
func (s *EventStore) FindByKey(ctx context.Context, key string) (events.Event, error) {
	var document eventDocument
	err := s.collection.FindOne(ctx, bson.D{{Key: "evtKey", Value: key}}).Decode(&document)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return events.Event{}, core.Newf(core.KindNotFound, "Event %q not found.", key)
		}
		return events.Event{}, unavailable(err)
	}
	return storedEvent(document), nil
}

// filterFor builds the find filter for a normalized query: type equality
// when set, tag membership when set (matching against the array field is
// containment — one document, many tags, no join table).
func filterFor(query events.Query) bson.D {
	filter := bson.D{}
	if query.Type != "" {
		filter = append(filter, bson.E{Key: "type", Value: query.Type})
	}
	if query.Tag != "" {
		filter = append(filter, bson.E{Key: "tags", Value: query.Tag})
	}
	return filter
}

// Recent lists matching events newest first (evtKey breaking ties, so
// equal instants never leave the order to chance), capped at the query's
// limit.
func (s *EventStore) Recent(ctx context.Context, query events.Query) ([]events.Event, error) {
	cursor, err := s.collection.Find(
		ctx,
		filterFor(query),
		options.Find().
			SetSort(bson.D{{Key: "occurredAt", Value: -1}, {Key: "evtKey", Value: 1}}).
			SetLimit(int64(query.Limit)),
	)
	if err != nil {
		return nil, unavailable(err)
	}
	defer cursor.Close(ctx)
	results := []events.Event{}
	for cursor.Next(ctx) {
		var document eventDocument
		if err := cursor.Decode(&document); err != nil {
			return nil, unavailable(err)
		}
		results = append(results, storedEvent(document))
	}
	if err := cursor.Err(); err != nil {
		return nil, unavailable(err)
	}
	return results, nil
}

// Count returns how many events match the query's filters.
func (s *EventStore) Count(ctx context.Context, query events.Query) (int64, error) {
	total, err := s.collection.CountDocuments(ctx, filterFor(query))
	if err != nil {
		return 0, unavailable(err)
	}
	return total, nil
}

// storedEvent maps the BSON document to the domain's stored form; a nil
// tag slice renders as the empty set.
func storedEvent(document eventDocument) events.Event {
	tags := document.Tags
	if tags == nil {
		tags = []string{}
	}
	return events.Event{
		ID:         document.ID.Hex(),
		Key:        document.Key,
		Type:       document.Type,
		Severity:   document.Severity,
		Source:     document.Source,
		Message:    document.Message,
		Tags:       tags,
		OccurredAt: document.OccurredAt.UTC(),
	}
}
