package security

// The two topic 25 demo stories. The lab story mints a FRESH limiter
// and spends its whole window in front of the caller (burst, wall,
// one-token recovery, per-client isolation); the PII story walks one
// fixture subject from classification through masked serving, the
// access log, erasure, and a live audit that sweeps every downstream
// copy for the original values. Each runner reports raw rows — the
// payload builders in internal/domain/demos derive every sentence.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"guide/internal/domain/demos"
	domsecurity "guide/internal/domain/security"
)

// RunSecurityLabStory walks the header set and a live rate-limit
// window against a fresh limiter, so the story is deterministic
// request over request.
func (c *Console) RunSecurityLabStory(ctx context.Context) (demos.SecurityLabRun, error) {
	limiter, err := c.newStoryLimiter()
	if err != nil {
		return demos.SecurityLabRun{}, err
	}

	headers := make([]demos.SecurityHeaderRow, 0, len(c.Headers))
	for _, rule := range c.Headers {
		headers = append(headers, demos.SecurityHeaderRow{Header: rule.Header, Value: rule.Value, Why: rule.Why})
	}

	// The window: one client spends the burst (5 at 10/s), then hammers
	// a wall that answers with the honest retry hint.
	const storyClient = "ip:198.51.100.7"
	rows := make([]demos.SecurityRateRequestRow, 0, 8)
	allowedCount, deniedCount := 0, 0
	firstRetryAfter := ""
	for i := 1; i <= 8; i++ {
		decision := limiter.Allow(storyClient)
		if decision.Allowed {
			allowedCount++
		} else {
			deniedCount++
			if firstRetryAfter == "" {
				firstRetryAfter = decision.RetryAfterText()
			}
		}
		rows = append(rows, demos.SecurityRateRequestRow{N: i, Allowed: decision.Allowed, RetryAfter: decision.RetryAfterText()})
	}

	// Recovery: the cooperative Wait SERVES the over-budget client by
	// waiting for the next token (one interval), and the wait after
	// that costs one MORE interval — over budget, a patient client is
	// served at exactly the sustained rate.
	waitStart := time.Now()
	if err := limiter.Wait(ctx, storyClient); err != nil {
		return demos.SecurityLabRun{}, fmt.Errorf("lab story recovery wait: %w", err)
	}
	recoveryMs := fmt.Sprintf("%.1f ms", float64(time.Since(waitStart).Microseconds())/1000)
	steadyStart := time.Now()
	if err := limiter.Wait(ctx, storyClient); err != nil {
		return demos.SecurityLabRun{}, fmt.Errorf("lab story steady wait: %w", err)
	}
	steadyMs := fmt.Sprintf("%.1f ms", float64(time.Since(steadyStart).Microseconds())/1000)

	// Per-client isolation: a second client arrives while the first is
	// over budget — its own bucket, its own full burst.
	const otherClient = "ip:203.0.113.9"
	otherAllowed := limiter.Allow(otherClient).Allowed

	return demos.SecurityLabRun{
		Headers:            headers,
		Policy:             limiter.Policy().String(),
		Requests:           rows,
		AllowedCount:       allowedCount,
		DeniedCount:        deniedCount,
		FirstRetryAfter:    firstRetryAfter,
		RecoveryWaitMs:     recoveryMs,
		SteadyWaitMs:       steadyMs,
		WaitServed:         true,
		OtherClientAllowed: otherAllowed,
		Buckets:            limiter.Size(),
		RoutePolicy:        RouteRatePolicy.String(),
	}, nil
}

// RunSecurityPIIStory walks one fixture subject from classification to
// the post-erasure audit, over a fresh store so erasure is always
// available to demonstrate. The story runs the REAL service use cases
// (View, Erase) and reads its own adapters only where the service
// deliberately has no side door (raw classification, the log list).
func (c *Console) RunSecurityPIIStory(ctx context.Context) (demos.SecurityPIIRun, error) {
	subjects := newMemorySubjectStore(seedSubjects())
	logs := &memoryAccessLogStore{}
	service := domsecurity.NewPIIService(subjects, logs)
	const subjectID = "sub_ada"
	now := time.Now().UTC()

	// Classification snapshot from the raw record, and the original PII
	// values captured BEFORE anything derives from them — the audit's
	// needles.
	record, found, err := subjects.FindSubject(ctx, subjectID)
	if err != nil {
		return demos.SecurityPIIRun{}, fmt.Errorf("pii story load subject: %w", err)
	}
	if !found {
		return demos.SecurityPIIRun{}, fmt.Errorf("pii story: fixture subject %s missing", subjectID)
	}
	classification := make([]demos.SecurityClassifiedRow, 0, len(record.Fields))
	originalPII := make([]string, 0, len(record.Fields))
	for _, field := range record.Fields {
		classification = append(classification, demos.SecurityClassifiedRow{
			Name: field.Name, Sensitivity: string(field.Sensitivity), Value: field.Value,
		})
		if field.Sensitivity == domsecurity.SensitivityPII {
			originalPII = append(originalPII, field.Value)
		}
	}

	// The masked view the API serves (the read lands in the log).
	view, found, err := service.View(ctx, subjectID, now)
	if err != nil || !found {
		return demos.SecurityPIIRun{}, fmt.Errorf("pii story view: found=%t err=%v", found, err)
	}
	masked := classifiedRows(view)
	logBefore, err := logRows(ctx, logs)
	if err != nil {
		return demos.SecurityPIIRun{}, fmt.Errorf("pii story log: %w", err)
	}

	// The right to be forgotten.
	report, err := service.Erase(ctx, subjectID, now.Add(time.Second))
	if err != nil {
		return demos.SecurityPIIRun{}, fmt.Errorf("pii story erase: %w", err)
	}

	// Serve again: tombstones now. (The read is logged again — with the
	// erased marker this time.)
	viewAfter, found, err := service.View(ctx, subjectID, now.Add(2*time.Second))
	if err != nil || !found {
		return demos.SecurityPIIRun{}, fmt.Errorf("pii story post-erase view: found=%t err=%v", found, err)
	}
	viewAfterErase := classifiedRows(viewAfter)
	logAfter, err := logRows(ctx, logs)
	if err != nil {
		return demos.SecurityPIIRun{}, fmt.Errorf("pii story log: %w", err)
	}

	// The audit: sweep every downstream copy for the original values —
	// the STORE's current (erased) state, every log line, and both
	// views. Masking is not erasure, so the pre-erase masked view is
	// swept too: it must contain only derived edges, never the values.
	stored, _, err := subjects.FindSubject(ctx, subjectID)
	if err != nil {
		return demos.SecurityPIIRun{}, fmt.Errorf("pii story audit load: %w", err)
	}
	var sweep strings.Builder
	for _, field := range stored.Fields {
		sweep.WriteString(field.Value)
		sweep.WriteByte(' ')
	}
	for _, row := range logAfter {
		sweep.WriteString(row.Line)
		sweep.WriteByte(' ')
	}
	for _, field := range masked {
		sweep.WriteString(field.Value)
		sweep.WriteByte(' ')
	}
	for _, field := range viewAfterErase {
		sweep.WriteString(field.Value)
		sweep.WriteByte(' ')
	}
	downstream := sweep.String()
	absent := true
	for _, needle := range originalPII {
		if strings.Contains(downstream, needle) {
			absent = false
		}
	}

	return demos.SecurityPIIRun{
		SubjectID:           subjectID,
		Classification:      classification,
		MaskedView:          masked,
		LogBefore:           logBefore,
		ErasedFields:        report.ErasedFields,
		ErasedAt:            report.ErasedAt.Format(time.RFC3339),
		ViewAfterErase:      viewAfterErase,
		LogAfter:            logAfter,
		PIIAbsentEverywhere: absent,
	}, nil
}

// classifiedRows converts a masked view to the demo's row shape.
func classifiedRows(view []domsecurity.MaskedField) []demos.SecurityClassifiedRow {
	rows := make([]demos.SecurityClassifiedRow, 0, len(view))
	for _, field := range view {
		rows = append(rows, demos.SecurityClassifiedRow{
			Name: field.Name, Sensitivity: string(field.Sensitivity), Value: field.Value,
		})
	}
	return rows
}

// logRows renders the access log's entries as demo rows.
func logRows(ctx context.Context, logs *memoryAccessLogStore) ([]demos.SecurityLogRow, error) {
	entries, err := logs.List(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]demos.SecurityLogRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, demos.SecurityLogRow{Line: domsecurity.RenderLogLine(entry)})
	}
	return rows, nil
}
