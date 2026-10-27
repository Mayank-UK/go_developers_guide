package demos

import (
	"strings"
	"testing"
)

// The security:pii payload builder: the six beats in order, the audit
// verdict DERIVED from the run (ok when clean, a loud rejected if the
// engine ever leaked), and no sentence asserting what the run did not
// report.

func TestSecurityPIIPayloadDerivesFromTheRun(t *testing.T) {
	run := SecurityPIIRun{
		SubjectID: "sub_ada",
		Classification: []SecurityClassifiedRow{
			{Name: "display_name", Sensitivity: "public", Value: "Ada"},
			{Name: "email", Sensitivity: "pii", Value: "ada@example.com"},
			{Name: "signup_ip", Sensitivity: "pii", Value: "198.51.100.7"},
		},
		MaskedView: []SecurityClassifiedRow{
			{Name: "display_name", Sensitivity: "public", Value: "Ada"},
			{Name: "email", Sensitivity: "pii", Value: "a***********m"},
		},
		LogBefore:    []SecurityLogRow{{Line: "subject=sub_ada action=read record=live"}},
		ErasedFields: []string{"email", "signup_ip"},
		ErasedAt:     "2026-10-27T12:00:01Z",
		ViewAfterErase: []SecurityClassifiedRow{
			{Name: "email", Sensitivity: "pii", Value: "[erased per subject request]"},
		},
		LogAfter: []SecurityLogRow{
			{Line: "subject=sub_ada action=read record=live"},
			{Line: "subject=sub_ada action=erase record=erased"},
		},
		PIIAbsentEverywhere: true,
	}
	payload := BuildSecurityPIIPayload(run)

	if payload.Mechanism != "security-pii" {
		t.Errorf("mechanism = %q", payload.Mechanism)
	}
	if len(payload.Beats) != 6 {
		t.Fatalf("beats = %d, want 6", len(payload.Beats))
	}
	beatIDs := make([]string, 0, len(payload.Beats))
	for _, beat := range payload.Beats {
		beatIDs = append(beatIDs, beat.Beat)
	}
	want := "classification,masked serving,the access-log policy,erasure (right to be forgotten),the view after erasure,the audit"
	if strings.Join(beatIDs, ",") != want {
		t.Errorf("beats = %v, want [%s]", beatIDs, want)
	}
	if !strings.Contains(payload.Beats[0].Detail, "email, signup_ip") || !strings.Contains(payload.Beats[0].Detail, "IP ADDRESS") {
		t.Errorf("classification beat must derive the PII list and the IP lesson: %s", payload.Beats[0].Detail)
	}
	if !strings.Contains(payload.Beats[3].Detail, "email") || !strings.Contains(payload.Beats[3].Detail, "2026-10-27T12:00:01Z") {
		t.Errorf("erasure beat must carry the fields and stamp: %s", payload.Beats[3].Detail)
	}
	if payload.Beats[5].Verdict != SecurityVerdictOK || !strings.Contains(payload.Beats[5].Detail, "absent everywhere") {
		t.Errorf("clean audit must read ok with the absence stated: %+v", payload.Beats[5])
	}
	for _, beat := range payload.Beats {
		if strings.Contains(beat.Detail, "ada@example.com") {
			t.Errorf("beat %q echoed RAW PII — previews only: %s", beat.Beat, beat.Detail)
		}
	}
	if len(payload.Beats[0].Facts) != 3 {
		t.Errorf("classification facts = %d, want one per field", len(payload.Beats[0].Facts))
	}
}

func TestSecurityPIIPayloadShoutsWhenTheAuditFails(t *testing.T) {
	payload := BuildSecurityPIIPayload(SecurityPIIRun{
		Classification:      []SecurityClassifiedRow{{Name: "email", Sensitivity: "pii", Value: "x@y.zz"}},
		MaskedView:          []SecurityClassifiedRow{{Name: "email", Sensitivity: "pii", Value: "*"}},
		ErasedFields:        []string{"email"},
		ViewAfterErase:      []SecurityClassifiedRow{{Name: "email", Sensitivity: "pii", Value: "[erased per subject request]"}},
		PIIAbsentEverywhere: false,
	})
	if payload.Beats[5].Verdict != SecurityVerdictRejected {
		t.Errorf("a failed audit must read rejected, got %q", payload.Beats[5].Verdict)
	}
	if !strings.Contains(payload.Beats[5].Detail, "engine defect") {
		t.Errorf("the failed-audit detail must name the defect loudly: %s", payload.Beats[5].Detail)
	}
}

func TestSecurityPIIPayloadPreviewTruncatesLongValues(t *testing.T) {
	payload := BuildSecurityPIIPayload(SecurityPIIRun{
		Classification: []SecurityClassifiedRow{
			{Name: "email", Sensitivity: "pii", Value: "averylongaddress@example.com"},
		},
		MaskedView:          []SecurityClassifiedRow{{Name: "email", Sensitivity: "pii", Value: "a********************m"}},
		ErasedFields:        []string{"email"},
		ViewAfterErase:      []SecurityClassifiedRow{{Name: "email", Sensitivity: "pii", Value: "[erased per subject request]"}},
		PIIAbsentEverywhere: true,
	})
	fact := payload.Beats[0].Facts[0].Value
	if !strings.HasSuffix(fact, "averylongaddress…") || strings.Contains(fact, "@example.com") {
		t.Errorf("classification preview = %q, want the value cut at 16 chars plus an ellipsis, domain gone", fact)
	}
}
