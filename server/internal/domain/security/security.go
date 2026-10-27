// Package security is the guide's security domain area (topic 25): the
// defense-in-depth pieces that live at the HTTP edge — the security
// header policy every response can carry, the per-client token-bucket
// rate limiter (golang.org/x/time/rate, pinned in go.mod), and the
// PII/GDPR substrate: records whose every field carries a sensitivity
// classification, masked views for serving, an erasure routine for the
// right to be forgotten, and an access-log entry type that has NO ROOM
// for field values — the log cannot leak PII because the type will not
// compile it in. Engines here depend on store ports and a clock the
// caller supplies, never on a storage technology; the in-memory
// adapters and demo stories live in internal/data/security.
package security

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ── classification ───────────────────────────────────────────────────────

// Sensitivity is the classification every field of a data subject's
// record carries. Classification is a DECISION made when the field is
// defined, not an afterthought at render time: what a field is
// classified as determines who may see it, how it is masked, and
// whether erasure must tombstone it.
type Sensitivity string

// The three classifications the substrate uses. PII is the GDPR sense:
// any information relating to an identified or identifiable person —
// email and phone obviously, but also signup IP addresses, which is
// the classification beginners get wrong.
const (
	// SensitivityPublic is safe for any caller: a display name.
	SensitivityPublic Sensitivity = "public"
	// SensitivityInternal is for the service's own use: a plan tier, a
	// feature flag. Not secret, not personal — but not the client's
	// business either.
	SensitivityInternal Sensitivity = "internal"
	// SensitivityPII is personal data: masked in every view, tombstoned
	// by erasure, and never allowed into an access-log entry.
	SensitivityPII Sensitivity = "pii"
)

// ClassifiedField is one field of a subject record with its
// classification stamped beside the value — the single source of truth
// both rendering and erasure consult.
type ClassifiedField struct {
	Name        string
	Sensitivity Sensitivity
	Value       string
}

// ErasedValue is the tombstone erasure writes into PII fields. It is a
// sentence, not an empty string, so an erased field is visibly erased
// rather than looking like missing data — and the fact of erasure
// survives while the value does not.
const ErasedValue = "[erased per subject request]"

// SubjectRecord is one data subject's row: a stable subject id, the
// classified fields, and — after erasure — the marker recording that
// the PII was tombstoned and when.
type SubjectRecord struct {
	SubjectID string
	Fields    []ClassifiedField
	Erased    bool
	ErasedAt  time.Time
}

// Field answers the named field.
func (r SubjectRecord) Field(name string) (ClassifiedField, bool) {
	for _, field := range r.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return ClassifiedField{}, false
}

// PIIFields lists the fields classified PII — what erasure must reach.
func (r SubjectRecord) PIIFields() []ClassifiedField {
	fields := make([]ClassifiedField, 0, len(r.Fields))
	for _, field := range r.Fields {
		if field.Sensitivity == SensitivityPII {
			fields = append(fields, field)
		}
	}
	return fields
}

// ── masking ──────────────────────────────────────────────────────────────

// MaskedField is one field of a masked view: the name and
// classification travel; PII values arrive masked, erased PII arrives
// as the tombstone, public and internal values arrive verbatim.
type MaskedField struct {
	Name        string
	Sensitivity Sensitivity
	Value       string
}

// MaskValue keeps the first and last character of a value and replaces
// everything between with asterisks ("ada@example.com" → "a***********m").
// Masking is a DISPLAY transform, not a protection: the output still
// derives from the PII, which is why erased fields render their
// tombstone instead of a mask — there is nothing left to derive from.
func MaskValue(value string) string {
	runes := []rune(value)
	switch {
	case len(runes) <= 2:
		return strings.Repeat("*", len(runes))
	default:
		return string(runes[0]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1])
	}
}

// MaskedView renders the record for serving: public and internal fields
// verbatim, PII fields masked — and once the record is erased, PII
// fields carry the tombstone, so no masked output can resurrect a
// value that erasure removed.
func (r SubjectRecord) MaskedView() []MaskedField {
	view := make([]MaskedField, 0, len(r.Fields))
	for _, field := range r.Fields {
		value := field.Value
		if field.Sensitivity == SensitivityPII {
			if r.Erased {
				value = ErasedValue
			} else {
				value = MaskValue(field.Value)
			}
		}
		view = append(view, MaskedField{Name: field.Name, Sensitivity: field.Sensitivity, Value: value})
	}
	return view
}

// ── erasure ──────────────────────────────────────────────────────────────

// ErasureReport is what the right-to-be-forgotten routine answers: the
// fields tombstoned, when, and whether the record had already been
// erased (the routine is idempotent — erasing twice is not an error).
type ErasureReport struct {
	SubjectID     string
	ErasedFields  []string
	ErasedAt      time.Time
	AlreadyErased bool
}

// ErrUnknownSubject is returned by the store lookup when the subject id
// does not resolve — the caller decides whether that is a 404.
var ErrUnknownSubject = errors.New("unknown subject")

// Erase tombstones every PII field of the record in place and stamps
// the erasure marker. Public and internal fields survive (a plan tier
// is not personal data); the FACT of erasure survives (GDPR allows —
// indeed expects — keeping the audit trail); the VALUES do not.
// Derived views need no separate pass because every view is rendered
// from the record, and the record no longer holds the values.
func (r *SubjectRecord) Erase(now time.Time) ErasureReport {
	if r.Erased {
		return ErasureReport{SubjectID: r.SubjectID, ErasedAt: r.ErasedAt, AlreadyErased: true}
	}
	erased := make([]string, 0, len(r.Fields))
	for i := range r.Fields {
		if r.Fields[i].Sensitivity == SensitivityPII {
			r.Fields[i].Value = ErasedValue
			erased = append(erased, r.Fields[i].Name)
		}
	}
	r.Erased = true
	r.ErasedAt = now
	return ErasureReport{SubjectID: r.SubjectID, ErasedFields: erased, ErasedAt: now}
}

// ── the access-log policy ────────────────────────────────────────────────

// AccessLogEntry is what an access log may hold about a subject
// interaction. The type is the policy: it carries the subject id, the
// action, the erased marker, and a timestamp — and has NO field a
// value could hide in. A logging call that wanted to write an email
// address here would not compile, which is a stronger guarantee than
// any "remember not to log PII" convention.
type AccessLogEntry struct {
	SubjectID string
	Action    string // "read" or "erase"
	Erased    bool
	At        time.Time
}

// RenderLogLine renders one entry as the log line the demo shows —
// built ONLY from the entry's own fields, so the rendering cannot leak
// what the type cannot hold.
func RenderLogLine(entry AccessLogEntry) string {
	erased := "live"
	if entry.Erased {
		erased = "erased"
	}
	return fmt.Sprintf("subject=%s action=%s record=%s at=%s", entry.SubjectID, entry.Action, erased, entry.At.Format(time.RFC3339))
}

// ── ports + the service ──────────────────────────────────────────────────

// SubjectStore is the PII store port: resolve by subject id, persist
// the mutated record. The demo console backs it with an in-memory map
// of fixture records; production would back it with the subjects table
// behind pgx — same port, same erasure routine.
type SubjectStore interface {
	FindSubject(ctx context.Context, subjectID string) (SubjectRecord, bool, error)
	SaveSubject(ctx context.Context, record SubjectRecord) error
}

// AccessLogStore is the access-log port: append on every interaction,
// list for the audit view.
type AccessLogStore interface {
	Append(ctx context.Context, entry AccessLogEntry) error
	List(ctx context.Context) ([]AccessLogEntry, error)
}

// PIIService carries the two GDPR-shaped use cases over the ports:
// View (masked serving plus a logged read) and Erase (the right to be
// forgotten plus a logged erasure). Both log through AccessLogEntry,
// which is why neither can leak field values into the log.
type PIIService struct {
	subjects SubjectStore
	logs     AccessLogStore
}

// NewPIIService wires the service over its two ports.
func NewPIIService(subjects SubjectStore, logs AccessLogStore) *PIIService {
	return &PIIService{subjects: subjects, logs: logs}
}

// View answers the masked view for a subject and records the read.
// The second return is false (with a nil error) when the subject id is
// unknown — the caller maps that to its own not-found answer.
func (s *PIIService) View(ctx context.Context, subjectID string, now time.Time) ([]MaskedField, bool, error) {
	record, found, err := s.subjects.FindSubject(ctx, subjectID)
	if err != nil {
		return nil, false, fmt.Errorf("find subject: %w", err)
	}
	if !found {
		return nil, false, nil
	}
	if err := s.logs.Append(ctx, AccessLogEntry{SubjectID: record.SubjectID, Action: "read", Erased: record.Erased, At: now}); err != nil {
		return nil, false, fmt.Errorf("append read log: %w", err)
	}
	return record.MaskedView(), true, nil
}

// Erase executes the right to be forgotten: load the record, tombstone
// its PII, persist, and log the erasure. Idempotent — erasing an
// already-erased subject answers the earlier report and changes
// nothing.
func (s *PIIService) Erase(ctx context.Context, subjectID string, now time.Time) (ErasureReport, error) {
	record, found, err := s.subjects.FindSubject(ctx, subjectID)
	if err != nil {
		return ErasureReport{}, fmt.Errorf("find subject: %w", err)
	}
	if !found {
		return ErasureReport{}, ErrUnknownSubject
	}
	report := record.Erase(now)
	if err := s.subjects.SaveSubject(ctx, record); err != nil {
		return ErasureReport{}, fmt.Errorf("save erased subject: %w", err)
	}
	if err := s.logs.Append(ctx, AccessLogEntry{SubjectID: record.SubjectID, Action: "erase", Erased: true, At: now}); err != nil {
		return ErasureReport{}, fmt.Errorf("append erase log: %w", err)
	}
	return report, nil
}
