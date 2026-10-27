package demos

import (
	"strings"
	"testing"
)

// The i18n:negotiator payload builder: the seven beats in order, every
// sentence derived from the run's rows (no asserted facts the engines
// did not report), and the honest-edge beat flipping verdict only when
// a policy failed to hold.

func i18nRunFixture() I18nNegotiatorRun {
	return I18nNegotiatorRun{
		Header:       "fr-CH, fr;q=0.9, en;q=0.8",
		HeaderSource: "request",
		Candidates: []I18nCandidateRow{
			{Range: "fr-ch", Quality: 1}, {Range: "fr", Quality: 0.9}, {Range: "en", Quality: 0.8},
		},
		Selected: "fr",
		Via:      "base",
		Chain: []I18nChainRow{
			{Range: "fr-ch", Quality: 1, Outcome: "base", Matched: "fr"},
		},
		Greetings: []I18nMessageRow{
			{Tag: "en", Text: "Welcome back, Ada!", Served: "en", Found: true},
			{Tag: "de", Text: "Willkommen zurück, Ada!", Served: "de", Found: true},
			{Tag: "fr", Text: "Bienvenue, Ada !", Served: "fr", Found: true},
			{Tag: "ja", Text: "おかえりなさい、Adaさん！", Served: "ja", Found: true},
		},
		Plurals: []I18nPluralRow{
			{Tag: "en", Count: 0, Category: "other", Text: "You have 0 new posts in your feed."},
			{Tag: "en", Count: 1, Category: "one", Text: "You have 1 new post in your feed."},
			{Tag: "en", Count: 2, Category: "other", Text: "You have 2 new posts in your feed."},
			{Tag: "fr", Count: 0, Category: "one", Text: "Tu as 0 nouvelle publication dans ton fil."},
			{Tag: "fr", Count: 1, Category: "one", Text: "Tu as 1 nouvelle publication dans ton fil."},
			{Tag: "fr", Count: 2, Category: "other", Text: "Tu as 2 nouvelles publications dans ton fil."},
			{Tag: "ja", Count: 0, Category: "other", Text: "フィードに新しい投稿が0件あります。"},
			{Tag: "ja", Count: 1, Category: "other", Text: "フィードに新しい投稿が1件あります。"},
			{Tag: "ja", Count: 2, Category: "other", Text: "フィードに新しい投稿が2件あります。"},
		},
		Formats: []I18nFormatRow{
			{Tag: "en", Number: "1,234,567.89", Currency: "$18.99", Date: "Oct 27, 2026", Clock: "12:30 PM"},
			{Tag: "de", Number: "1.234.567,89", Currency: "18,99 €", Date: "27.10.2026", Clock: "12:30"},
			{Tag: "fr", Number: "1\u202f234\u202f567,89", Currency: "18,99 €", Date: "27 octobre 2026", Clock: "12:30"},
			{Tag: "ja", Number: "1,234,567.89", Currency: "¥1,899", Date: "2026年10月27日", Clock: "12:30"},
		},
		Instant: "2026-10-27T12:30:00Z",
		Zones: []I18nZoneRow{
			{Zone: "Europe/Berlin", Abbreviation: "CET", Offset: "+01:00", ClockEN: "1:30 PM", ClockDE: "13:30"},
			{Zone: "America/New_York", Abbreviation: "EDT", Offset: "-04:00", ClockEN: "8:30 AM", ClockDE: "08:30"},
			{Zone: "Asia/Tokyo", Abbreviation: "JST", Offset: "+09:00", ClockEN: "9:30 PM", ClockDE: "21:30"},
		},
		ZoneHonesty: I18nZoneHonestyRow{Zone: "Mars/Olympus", Error: "unknown time zone Mars/Olympus"},
		Composed: []I18nMessageRow{
			{Tag: "en", Text: "Published Oct 27, 2026 at 1:30 PM (Europe/Berlin).", Served: "en", Found: true},
			{Tag: "de", Text: "Veröffentlicht am 27.10.2026 um 13:30 (Europe/Berlin).", Served: "de", Found: true},
		},
		Fallback: I18nFallbackRow{
			Header:   "ko-KR,ko;q=0.9",
			Chain:    []I18nChainRow{{Range: "ko-kr", Outcome: "none"}, {Range: "ko", Outcome: "none"}},
			Selected: "en",
			Via:      "default",
		},
		MissingKey: I18nMissingKeyRow{
			Key:     "demo.never_translated",
			Text:    "demo.never_translated",
			LogLine: `missing i18n catalog key "demo.never_translated" for "de" — serving the key itself`,
		},
	}
}

func TestI18nNegotiatorPayloadDerivesFromTheRun(t *testing.T) {
	payload := BuildI18nNegotiatorPayload(i18nRunFixture())

	if payload.Mechanism != "i18n-negotiator" {
		t.Errorf("mechanism = %q", payload.Mechanism)
	}
	if len(payload.Beats) != 7 {
		t.Fatalf("beats = %d, want 7", len(payload.Beats))
	}
	wantBeats := "the header, parsed,the match,one message, every catalog,plurals across locales,locale formats,timezone is not locale,the honest edges"
	got := make([]string, 0, len(payload.Beats))
	for _, beat := range payload.Beats {
		got = append(got, beat.Beat)
	}
	if strings.Join(got, ",") != wantBeats {
		t.Errorf("beats = %v", got)
	}
	// Beat 1 derives the header and its source.
	if !strings.Contains(payload.Beats[0].Detail, "fr-CH, fr;q=0.9") || !strings.Contains(payload.Beats[0].Detail, "own header") {
		t.Errorf("header beat must quote the request header and name its source: %s", payload.Beats[0].Detail)
	}
	if len(payload.Beats[0].Facts) != 3 || payload.Beats[0].Facts[0].Label != "fr-ch" {
		t.Errorf("candidate facts = %+v", payload.Beats[0].Facts)
	}
	// Beat 2 derives selection and via.
	if !strings.Contains(payload.Beats[1].Detail, `"fr" catalog via base`) {
		t.Errorf("match beat must state the selected catalog and via: %s", payload.Beats[1].Detail)
	}
	// Beat 4 derives each locale's singular counts from the rows.
	if !strings.Contains(payload.Beats[3].Detail, "fr: 0 and 1") || !strings.Contains(payload.Beats[3].Detail, "en: 1") {
		t.Errorf("plural beat must derive the singular counts: %s", payload.Beats[3].Detail)
	}
	if !strings.Contains(payload.Beats[3].Detail, "ja: no singular") {
		t.Errorf("ja's rule has no singular — must be derived, not asserted: %s", payload.Beats[3].Detail)
	}
	// Beat 6 renders the honesty error verbatim.
	if !strings.Contains(payload.Beats[5].Detail, "unknown time zone Mars/Olympus") {
		t.Errorf("zone beat must quote the unknown-zone error: %s", payload.Beats[5].Detail)
	}
	// Beat 7 states both edges.
	if !strings.Contains(payload.Beats[6].Detail, "ko-KR,ko;q=0.9") || !strings.Contains(payload.Beats[6].Detail, "demo.never_translated") {
		t.Errorf("edges beat must carry the fallback header and missing key: %s", payload.Beats[6].Detail)
	}
	if payload.Beats[6].Verdict != I18nVerdictOK {
		t.Errorf("a policy that held must read ok: %q", payload.Beats[6].Verdict)
	}
	for i, beat := range payload.Beats {
		if beat.Facts == nil {
			t.Errorf("beat %d facts must render [] not null", i)
		}
	}
	if len(payload.Proofs) == 0 || payload.Reasoning == "" {
		t.Errorf("payload lacks proofs/reasoning: %+v", payload)
	}
}

func TestI18nNegotiatorPayloadNamesTheFixtureWhenTheRequestHadNoHeader(t *testing.T) {
	run := i18nRunFixture()
	run.Header = I18nFixtureHeader
	run.HeaderSource = "fixture"
	payload := BuildI18nNegotiatorPayload(run)
	if !strings.Contains(payload.Beats[0].Detail, "fixture") || strings.Contains(payload.Beats[0].Detail, "own header") {
		t.Errorf("fixture runs must say so: %s", payload.Beats[0].Detail)
	}
}

func TestI18nNegotiatorPayloadShoutsWhenTheMissingKeyPolicyDidNotRun(t *testing.T) {
	run := i18nRunFixture()
	run.MissingKey = I18nMissingKeyRow{Key: "demo.never_translated"}
	payload := BuildI18nNegotiatorPayload(run)
	if payload.Beats[6].Verdict != I18nVerdictRejected {
		t.Errorf("an unreported missing-key policy must read rejected: %q", payload.Beats[6].Verdict)
	}
}
