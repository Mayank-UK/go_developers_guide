package messaging

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"guide/internal/domain/messaging"
)

// Compile-time proofs the outbox satisfies the domain ports.
var (
	_ messaging.OutboxRecorder = (*Outbox)(nil)
	_ messaging.OutboxRelay    = (*Outbox)(nil)
)

// schemaDDL is the demo tables' DDL, embedded (the ormprobe pattern):
// CREATE TABLE IF NOT EXISTS, idempotent, so EnsureSchema is safe to call
// on every story run.
//
//go:embed schema.sql
var schemaDDL string

// Outbox is the pgx-backed transactional outbox: the write side commits a
// business event row and its outbox row in ONE transaction; the read side
// drains committed pending rows, publishes each onto its Redis stream
// through the stream store, and stamps it published. It composes BOTH
// stores — which is the pattern's point: PostgreSQL owns the
// all-or-nothing commit, Redis owns the fan-out.
type Outbox struct {
	pool    *pgxpool.Pool
	streams *StreamStore
}

// NewOutbox wires the outbox over the shared PostgreSQL pool (never its
// own connections) and the stream store it publishes through.
func NewOutbox(pool *pgxpool.Pool, streams *StreamStore) *Outbox {
	return &Outbox{pool: pool, streams: streams}
}

// EnsureSchema applies the embedded idempotent DDL. Production outboxes
// migrate once at deploy time; the demo's tables are created on demand so
// this feature carries no boot dependency of its own.
func (o *Outbox) EnsureSchema(ctx context.Context) error {
	if _, err := o.pool.Exec(ctx, schemaDDL); err != nil {
		return fmt.Errorf("ensure outbox schema: %w", err)
	}
	return nil
}

// RecordEventWithOutbox is the pattern's write side: BEGIN, INSERT the
// business event, INSERT the outbox row, COMMIT — one transaction, one
// all-or-nothing unit. If the process dies before COMMIT, neither row
// exists; there is no committed state change whose event nobody can
// publish, and no event describing a change that never happened. That
// double impossibility is the entire reason the outbox exists — the
// naive alternative (INSERT, commit, then publish) leaves a crash window
// between the two where the event is lost forever.
func (o *Outbox) RecordEventWithOutbox(ctx context.Context, event messaging.EventRecord, message messaging.OutboxMessage) error {
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin the outbox transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO msgdemo_events (id, run_id, kind, payload)
		VALUES ($1, $2, $3, $4)`,
		event.ID, event.RunID, event.Kind, event.Payload); err != nil {
		return fmt.Errorf("insert the event row: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO msgdemo_outbox (id, run_id, stream, payload)
		VALUES ($1, $2, $3, $4)`,
		message.ID, message.RunID, message.Stream, message.Payload); err != nil {
		return fmt.Errorf("insert the outbox row: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit the outbox transaction: %w", err)
	}
	return nil
}

// RecordEventWithOutboxThenRollback is the demo's honesty beat: the SAME
// two writes as RecordEventWithOutbox, followed by an explicit ROLLBACK —
// proving the unit is really all-or-nothing, because afterwards NEITHER
// row exists. It exists for teaching; production code never rolls back
// on purpose, but it crashes mid-transaction all the time, and the
// rollback is exactly what a crash does.
func (o *Outbox) RecordEventWithOutboxThenRollback(ctx context.Context, event messaging.EventRecord, message messaging.OutboxMessage) error {
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin the rolled-back transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO msgdemo_events (id, run_id, kind, payload)
		VALUES ($1, $2, $3, $4)`,
		event.ID, event.RunID, event.Kind, event.Payload); err != nil {
		return fmt.Errorf("insert the rolled-back event row: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO msgdemo_outbox (id, run_id, stream, payload)
		VALUES ($1, $2, $3, $4)`,
		message.ID, message.RunID, message.Stream, message.Payload); err != nil {
		return fmt.Errorf("insert the rolled-back outbox row: %w", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		return fmt.Errorf("roll the transaction back: %w", err)
	}
	return nil
}

// RelayPending is the pattern's read side: drain the run's pending
// outbox rows oldest-first under FOR UPDATE SKIP LOCKED (concurrent
// relays divide the work instead of fighting over rows), XADD each onto
// its stream, stamp it published, COMMIT. The stamp rides the SAME
// transaction as the row locks — if the relay dies between publish and
// commit, the row is still pending on restart and publishes AGAIN:
// at-least-once by construction, which is why the consumer side dedups
// on the outbox id rather than trusting this pipeline to be exactly-once.
// The run scoping is the DEMO's isolation seam (concurrent story runs
// never relay each other's rows); a production relay drains globally.
func (o *Outbox) RelayPending(ctx context.Context, runID string, limit int) (messaging.RelayReport, error) {
	if limit <= 0 {
		return messaging.RelayReport{}, fmt.Errorf("relay limit must be positive, got %d", limit)
	}
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return messaging.RelayReport{}, fmt.Errorf("begin the relay transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, stream, payload FROM msgdemo_outbox
		WHERE published_at IS NULL AND run_id = $2
		ORDER BY created_at, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED`,
		limit, runID)
	if err != nil {
		return messaging.RelayReport{}, fmt.Errorf("select pending outbox rows: %w", err)
	}
	type pendingRow struct {
		id      string
		stream  string
		payload string
	}
	var pending []pendingRow
	for rows.Next() {
		var row pendingRow
		if err := rows.Scan(&row.id, &row.stream, &row.payload); err != nil {
			rows.Close()
			return messaging.RelayReport{}, fmt.Errorf("scan a pending outbox row: %w", err)
		}
		pending = append(pending, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return messaging.RelayReport{}, fmt.Errorf("read pending outbox rows: %w", err)
	}

	report := messaging.RelayReport{Published: make([]messaging.RelayOutcome, 0, len(pending))}
	for _, row := range pending {
		// The outbox id rides WITH the message as a field: consumers
		// dedup on it, so redelivery is detectable downstream.
		entryID, err := o.streams.Append(ctx, row.stream, map[string]string{
			"messageId": row.id,
			"payload":   row.payload,
		})
		if err != nil {
			return messaging.RelayReport{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE msgdemo_outbox SET published_at = now() WHERE id = $1`,
			row.id); err != nil {
			return messaging.RelayReport{}, fmt.Errorf("mark outbox row %s published: %w", row.id, err)
		}
		report.Published = append(report.Published, messaging.RelayOutcome{
			OutboxID:    row.id,
			Stream:      row.stream,
			StreamEntry: entryID,
		})
	}
	if err := tx.Commit(ctx); err != nil {
		return messaging.RelayReport{}, fmt.Errorf("commit the relay transaction: %w", err)
	}
	return report, nil
}

// CountEvent counts the demo's committed business rows for one event of a
// run — the rollback beat's proof reads from here: the rolled-back event
// is absent by exactly one.
func (o *Outbox) CountEvent(ctx context.Context, runID string, eventID string) (int, error) {
	var count int
	if err := o.pool.QueryRow(ctx,
		`SELECT count(*) FROM msgdemo_events WHERE run_id = $1 AND id = $2`,
		runID, eventID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count event %s: %w", eventID, err)
	}
	return count, nil
}

// Cleanup deletes every row the run created — the events and outbox rows
// of this run only (run_id scoping), so a demo leaves the shared
// PostgreSQL exactly as it found it.
func (o *Outbox) Cleanup(ctx context.Context, runID string) (events, outbox int64, err error) {
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin the outbox cleanup: %w", err)
	}
	defer tx.Rollback(ctx)

	eventsTag, err := tx.Exec(ctx, `DELETE FROM msgdemo_events WHERE run_id = $1`, runID)
	if err != nil {
		return 0, 0, fmt.Errorf("delete run %s events: %w", runID, err)
	}
	outboxTag, err := tx.Exec(ctx, `DELETE FROM msgdemo_outbox WHERE run_id = $1`, runID)
	if err != nil {
		return 0, 0, fmt.Errorf("delete run %s outbox rows: %w", runID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("commit the outbox cleanup: %w", err)
	}
	return eventsTag.RowsAffected(), outboxTag.RowsAffected(), nil
}
