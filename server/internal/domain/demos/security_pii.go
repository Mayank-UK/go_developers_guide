// security:pii — the second topic 25 demo: classification, masked
// serving, the access-log policy, and the right to be forgotten, over
// fixture records, with the erasure audit derived live.
package demos

import (
	"context"
	"fmt"
	"strings"
)

// SecurityPIIRunner is the port the PII story runs through.
type SecurityPIIRunner interface {
	RunSecurityPIIStory(ctx context.Context) (SecurityPIIRun, error)
}

// SecurityClassifiedRow is one field with its classification and the
// value as THIS beat observed it (raw for classification, masked for
// views, tombstone after erasure).
type SecurityClassifiedRow struct {
	Name        string
	Sensitivity string
	Value       string
}

// SecurityLogRow is one access-log line as the run rendered it.
type SecurityLogRow struct {
	Line string
}

// SecurityPIIRun is the PII story's raw haul: the fixture record's
// classification, the masked view the API serves, the log lines, the
// erasure report, the post-erasure view, and the audit verdict that
// none of the original PII survives anywhere downstream.
type SecurityPIIRun struct {
	SubjectID           string
	Classification      []SecurityClassifiedRow
	MaskedView          []SecurityClassifiedRow
	LogBefore           []SecurityLogRow
	ErasedFields        []string
	ErasedAt            string
	ViewAfterErase      []SecurityClassifiedRow
	LogAfter            []SecurityLogRow
	PIIAbsentEverywhere bool
}

// BuildSecurityPIIPayload shapes the PII story.
func BuildSecurityPIIPayload(run SecurityPIIRun) SecurityStoryPayload {
	classificationFacts := make([]SecurityFactRow, 0, len(run.Classification))
	for _, field := range run.Classification {
		classificationFacts = append(classificationFacts, SecurityFactRow{
			Label: field.Name,
			Value: fmt.Sprintf("%s — %s", field.Sensitivity, previewValue(field.Value)),
		})
	}
	maskedFacts := make([]SecurityFactRow, 0, len(run.MaskedView))
	for _, field := range run.MaskedView {
		maskedFacts = append(maskedFacts, SecurityFactRow{Label: field.Name, Value: field.Value})
	}
	auditVerdict := SecurityVerdictOK
	auditDetail := fmt.Sprintf("the audit swept the store, BOTH logged reads, and the post-erasure view for the original PII values: absent everywhere (%t). The tombstone survives, the fact of erasure survives in the log — the values do not.",
		run.PIIAbsentEverywhere)
	if !run.PIIAbsentEverywhere {
		auditVerdict = SecurityVerdictRejected
		auditDetail = "the audit FOUND original PII somewhere downstream — reported as it happened; this would be an engine defect"
	}
	logFacts := make([]SecurityFactRow, 0, len(run.LogAfter))
	for _, row := range run.LogAfter {
		logFacts = append(logFacts, SecurityFactRow{Label: "log", Value: row.Line})
	}
	piiNames := make([]string, 0, len(run.Classification))
	ipNote := ""
	for _, field := range run.Classification {
		if field.Sensitivity != "pii" {
			continue
		}
		piiNames = append(piiNames, field.Name)
		if strings.Contains(field.Name, "ip") {
			ipNote = " — including an IP ADDRESS, which is personal data under GDPR and the classification most teams get wrong"
		}
	}
	beats := []SecurityStoryBeat{
		{
			Beat: "classification", Verdict: SecurityVerdictInfo,
			Detail: fmt.Sprintf("one fixture record for %s, every field stamped at definition time. The PII set this record declares is [%s]%s.",
				run.SubjectID, strings.Join(piiNames, ", "), ipNote),
			Facts: classificationFacts,
		},
		{
			Beat: "masked serving", Verdict: SecurityVerdictOK,
			Detail: "the API serves the MASKED view: public and internal fields verbatim, every PII field reduced to its edges. Masking is a display transform — the output still derives from the PII, so it is for serving, never for storage.",
			Facts:  maskedFacts,
		},
		{
			Beat: "the access-log policy", Verdict: SecurityVerdictInfo,
			Detail: fmt.Sprintf("the read was logged (%d line(s) before erasure) — and the log entry TYPE carries only subject id, action, and the erased marker: it has NO field a value could hide in, so a logging call that wanted to write the email would not compile. Type-level policy beats a convention.",
				len(run.LogBefore)),
			Facts: securityLogFacts(run.LogBefore),
		},
		{
			Beat: "erasure (right to be forgotten)", Verdict: SecurityVerdictOK,
			Detail: fmt.Sprintf("the erasure routine tombstoned exactly the %d PII field(s) [%s] at %s and PERSISTED the record. Public and internal fields survive (a plan tier is not personal data); the fact of erasure survives (the audit trail is expected to); the VALUES do not.",
				len(run.ErasedFields), strings.Join(run.ErasedFields, ", "), run.ErasedAt),
		},
		{
			Beat: "the view after erasure", Verdict: SecurityVerdictOK,
			Detail: "serving the same subject now answers tombstones for the PII fields — no masked output can resurrect a value the record no longer holds. Erasure completeness for derived views is free BECAUSE every view renders from the one record.",
			Facts:  securityClassifiedFacts(run.ViewAfterErase),
		},
		{
			Beat: "the audit", Verdict: auditVerdict,
			Detail: auditDetail,
			Facts:  logFacts,
		},
	}
	return newSecurityStoryPayload("security-pii",
		beats,
		[]SecurityFactRow{
			{Label: "classification", Value: "public / internal / pii stamped at field definition — the decision that drives masking, logging, and erasure"},
			{Label: "minimization", Value: "masked serving: PII reduced to its edges, derived per request, never stored masked"},
			{Label: "the log cannot leak", Value: "AccessLogEntry holds subject id + action + markers — nowhere for a value; assert it by type, not by discipline"},
			{Label: "erasure", Value: fmt.Sprintf("tombstoned %d PII field(s), persisted, logged; re-erasure is an idempotent no-op", len(run.ErasedFields))},
		},
		"GDPR's shape for an API is these four moves: classify at definition (the field list IS the inventory an auditor asks for), minimize in serving (masked views derived per request), log access without values (an entry type with no room for them), and erase completely (tombstone the PII, keep the fact, persist, verify). The audit beat is the discipline that makes erasure real: sweep every downstream copy for the original values — store, logs, and views — and prove the absence, because 'we deleted it' is a claim while an absent-value sweep is a check.")
}

// securityClassifiedFacts renders classified rows as facts.
func securityClassifiedFacts(rows []SecurityClassifiedRow) []SecurityFactRow {
	facts := make([]SecurityFactRow, 0, len(rows))
	for _, field := range rows {
		facts = append(facts, SecurityFactRow{Label: field.Name, Value: field.Value})
	}
	return facts
}

// securityLogFacts renders log rows as facts.
func securityLogFacts(rows []SecurityLogRow) []SecurityFactRow {
	facts := make([]SecurityFactRow, 0, len(rows))
	for _, row := range rows {
		facts = append(facts, SecurityFactRow{Label: "log", Value: row.Line})
	}
	return facts
}

// previewValue shortens raw classification values for display.
func previewValue(value string) string {
	if len(value) > 16 {
		return value[:16] + "…"
	}
	return value
}
