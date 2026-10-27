// i18n:negotiator — topic 27's demo: one request's Accept-Language
// header walked from raw text to a served message. The run rows are
// what the engines answered; this builder derives every sentence from
// them (the story discipline the family's demos share: beats in
// execution order, derived proofs, a closing reasoning paragraph).
package demos

import (
	"context"
	"fmt"
	"strings"
)

// I18nFixtureHeader is the fixture the endpoint uses when the request
// carries no Accept-Language of its own — the five-member example from
// RFC 9110 §12.5.4, which exercises wildcard, q-values, and a regional
// variant in one line.
const I18nFixtureHeader = "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5"

// I18nUnsupportedHeader is the fixture for the fallback beat: a
// language no catalog carries, so the default must serve.
const I18nUnsupportedHeader = "ko-KR,ko;q=0.9"

// I18nNegotiatorRunner is the port the negotiator story runs through;
// the data-layer console implements it against the real engine.
type I18nNegotiatorRunner interface {
	RunNegotiatorStory(ctx context.Context, header, headerSource string) (I18nNegotiatorRun, error)
}

// I18nCandidateRow is one parsed Accept-Language member with its
// quality value, after the parser dropped what it could not use.
type I18nCandidateRow struct {
	Range   string
	Quality float64
}

// I18nChainRow is one fallback-chain step: the range examined, its q,
// what came of it, and the supported tag it matched (if any).
type I18nChainRow struct {
	Range   string
	Quality float64
	Outcome string
	Matched string
}

// I18nMessageRow is one rendered message: the tag asked for, the text
// served, the tag that actually answered, and whether any catalog
// carried the key at all.
type I18nMessageRow struct {
	Tag    string
	Text   string
	Served string
	Found  bool
}

// I18nPluralRow is one plural rendering: locale, count, the CLDR
// category the locale's rule selected, and the resulting text.
type I18nPluralRow struct {
	Tag      string
	Count    int
	Category string
	Text     string
}

// I18nFormatRow is one locale's full format pass over the same value
// and instant: number, currency, date, and clock.
type I18nFormatRow struct {
	Tag      string
	Number   string
	Currency string
	Date     string
	Clock    string
}

// I18nZoneRow is one timezone rendering of the story instant: the
// zone's IANA name, abbreviation, UTC offset, and its clock in both
// the 12-hour (en) and 24-hour (de) conventions — same instant, same
// locale pair, different clock readings.
type I18nZoneRow struct {
	Zone         string
	Abbreviation string
	Offset       string
	ClockEN      string
	ClockDE      string
}

// I18nZoneHonestyRow records what the engine answers for a zone that
// does not exist: the error, verbatim — never a panic, never a
// fabricated zone.
type I18nZoneHonestyRow struct {
	Zone  string
	Error string
}

// I18nFallbackRow is the unsupported-language beat's run: the header,
// the chain it walked, and the default that served.
type I18nFallbackRow struct {
	Header   string
	Chain    []I18nChainRow
	Selected string
	Via      string
}

// I18nMissingKeyRow is the missing-key policy's run: the key asked
// for, the text served (the key itself), and the log line the policy
// emitted.
type I18nMissingKeyRow struct {
	Key     string
	Text    string
	LogLine string
}

// I18nNegotiatorRun is the negotiator story's raw haul. Header is the
// header the story ran (the request's own, or the RFC fixture);
// HeaderSource says which.
type I18nNegotiatorRun struct {
	Header       string
	HeaderSource string // "request" | "fixture"
	Candidates   []I18nCandidateRow
	Selected     string
	Via          string
	Chain        []I18nChainRow
	Greetings    []I18nMessageRow
	Plurals      []I18nPluralRow
	Formats      []I18nFormatRow
	Instant      string // RFC3339 — the instant every zone renders
	Zones        []I18nZoneRow
	ZoneHonesty  I18nZoneHonestyRow
	Composed     []I18nMessageRow
	Fallback     I18nFallbackRow
	MissingKey   I18nMissingKeyRow
}

// Verdict labels the i18n story's client colors: green ok, red
// rejected (a policy that did NOT hold), neutral info.
const (
	I18nVerdictOK       = "ok"
	I18nVerdictRejected = "rejected"
	I18nVerdictInfo     = "info"
)

// I18nFactRow is one label/value pair for the story's beats and
// proofs. Values are pre-rendered strings so the client never formats.
type I18nFactRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// I18nStoryBeat is one beat as the payload serves it.
type I18nStoryBeat struct {
	Beat    string        `json:"beat"`
	Detail  string        `json:"detail"`
	Verdict string        `json:"verdict"`
	Facts   []I18nFactRow `json:"facts"`
}

// I18nStoryPayload is the wire shape the negotiator endpoint serves —
// the family's story shape (mechanism, beats, proofs, reasoning) over
// i18n's own types. Every slice is non-nil so the wire renders [].
type I18nStoryPayload struct {
	Mechanism string          `json:"mechanism"`
	Beats     []I18nStoryBeat `json:"beats"`
	Proofs    []I18nFactRow   `json:"proofs"`
	Reasoning string          `json:"reasoning"`
}

func newI18nStoryPayload(mechanism string, beats []I18nStoryBeat, proofs []I18nFactRow, reasoning string) I18nStoryPayload {
	for i := range beats {
		if beats[i].Facts == nil {
			beats[i].Facts = []I18nFactRow{}
		}
	}
	if proofs == nil {
		proofs = []I18nFactRow{}
	}
	return I18nStoryPayload{Mechanism: mechanism, Beats: beats, Proofs: proofs, Reasoning: reasoning}
}

// BuildI18nNegotiatorPayload shapes the negotiator story: seven beats,
// header to honest edges, every sentence derived from the run.
func BuildI18nNegotiatorPayload(run I18nNegotiatorRun) I18nStoryPayload {
	// Beat 1 — the header as the parser saw it.
	source := "the RFC 9110 fixture (the request carried no Accept-Language header)"
	if run.HeaderSource == "request" {
		source = "THIS request's own header"
	}
	candidateFacts := make([]I18nFactRow, 0, len(run.Candidates))
	for _, candidate := range run.Candidates {
		candidateFacts = append(candidateFacts, I18nFactRow{
			Label: candidate.Range,
			Value: fmt.Sprintf("q=%.3f", candidate.Quality),
		})
	}

	// Beat 2 — the fallback chain that selected the catalog.
	chainFacts := make([]I18nFactRow, 0, len(run.Chain))
	for _, step := range run.Chain {
		outcome := step.Outcome
		if step.Matched != "" {
			outcome = fmt.Sprintf("%s → %s", step.Outcome, step.Matched)
		}
		chainFacts = append(chainFacts, I18nFactRow{
			Label: fmt.Sprintf("%s (q=%.1f)", step.Range, step.Quality),
			Value: outcome,
		})
	}

	// Beat 3 — the same message from every catalog.
	greetingFacts := make([]I18nFactRow, 0, len(run.Greetings))
	for _, message := range run.Greetings {
		greetingFacts = append(greetingFacts, I18nFactRow{Label: message.Tag, Value: message.Text})
	}

	// Beat 4 — plurals at 0/1/2, with each locale's singular counts
	// derived from the rows rather than asserted.
	pluralFacts := make([]I18nFactRow, 0, len(run.Plurals))
	singulars := map[string][]string{}
	for _, plural := range run.Plurals {
		pluralFacts = append(pluralFacts, I18nFactRow{
			Label: fmt.Sprintf("%s · %d (%s)", plural.Tag, plural.Count, plural.Category),
			Value: plural.Text,
		})
		if plural.Category == "one" {
			singulars[plural.Tag] = append(singulars[plural.Tag], fmt.Sprint(plural.Count))
		}
	}
	singularSentences := make([]string, 0, len(run.Greetings)+1)
	for _, message := range run.Greetings {
		counts := singulars[message.Tag]
		if len(counts) == 0 {
			singularSentences = append(singularSentences, fmt.Sprintf("%s: no singular at all", message.Tag))
			continue
		}
		singularSentences = append(singularSentences, fmt.Sprintf("%s: %s", message.Tag, strings.Join(counts, " and ")))
	}

	// Beat 5 — one value and one instant through every locale's formats.
	formatFacts := make([]I18nFactRow, 0, len(run.Formats))
	for _, format := range run.Formats {
		formatFacts = append(formatFacts, I18nFactRow{
			Label: format.Tag,
			Value: fmt.Sprintf("%s · %s · %s · %s", format.Number, format.Currency, format.Date, format.Clock),
		})
	}

	// Beat 6 — the same instant through three timezones (and both clock
	// conventions), plus the unknown-zone honesty row.
	zoneFacts := make([]I18nFactRow, 0, len(run.Zones)+1)
	for _, zone := range run.Zones {
		zoneFacts = append(zoneFacts, I18nFactRow{
			Label: fmt.Sprintf("%s (%s, UTC%s)", zone.Zone, zone.Abbreviation, zone.Offset),
			Value: fmt.Sprintf("de clock %s · en clock %s", zone.ClockDE, zone.ClockEN),
		})
	}
	zoneFacts = append(zoneFacts, I18nFactRow{
		Label: fmt.Sprintf("%s (unknown zone)", run.ZoneHonesty.Zone),
		Value: run.ZoneHonesty.Error,
	})

	// Beat 7 — the honest edges: unsupported language, missing key.
	missingText := run.MissingKey.Text
	missingVerdict := I18nVerdictOK
	missingNote := "served as the visible key itself, logged, never a 500"
	if run.MissingKey.Text == "" {
		missingText = "(no run)"
		missingVerdict = I18nVerdictRejected
		missingNote = "the missing-key policy did not report"
	}

	beats := []I18nStoryBeat{
		{
			Beat: "the header, parsed", Verdict: I18nVerdictInfo,
			Detail: fmt.Sprintf("the story runs against %s: %q. The parser split it into %d usable candidate(s), each with its quality value — the list is ordered by q (ties keep the sender's order), and members the parser could not use were dropped, not guessed for.",
				source, run.Header, len(run.Candidates)),
			Facts: candidateFacts,
		},
		{
			Beat: "the match", Verdict: I18nVerdictOK,
			Detail: fmt.Sprintf("the negotiator walked the candidates most-specific first — exact tag, then base language, then the wildcard — and selected the %q catalog via %s. Every step it examined is on the left; a request that matches nothing at all would end at the default catalog, never at an error.",
				run.Selected, run.Via),
			Facts: chainFacts,
		},
		{
			Beat: "one message, every catalog", Verdict: I18nVerdictOK,
			Detail: "the SAME key (demo.greeting, {name} interpolated with \"Ada\") rendered by each carried catalog — one translation per tag, all served by the one engine call. The row's tag is also WHO answered: a half-translated locale would show the default tag serving in its place.",
			Facts:  greetingFacts,
		},
		{
			Beat: "plurals across locales", Verdict: I18nVerdictOK,
			Detail: fmt.Sprintf("the plural key at counts 0, 1, and 2 — the singular counts each locale's rule selects: %s. English and German singularize exactly 1; French singularizes 0 AND 1; Japanese never distinguishes. Same counts, three rules, one engine.",
				strings.Join(singularSentences, " · ")),
			Facts: pluralFacts,
		},
		{
			Beat: "locale formats", Verdict: I18nVerdictOK,
			Detail: fmt.Sprintf("the number 1234567.89, an 1899-minor-unit price, the date of the story instant, and its clock — rendered by each locale's conventions: grouping separator, decimal separator, currency symbol placement and digits, month names, 12- vs 24-hour. Four locales, one value each, zero float money (the currency path is integer minor units end to end)."),
			Facts:  formatFacts,
		},
		{
			Beat: "timezone is not locale", Verdict: I18nVerdictInfo,
			Detail: fmt.Sprintf("the SAME instant (%s) read in three IANA zones: each row's clock differs because the ZONE differs, while the de/en clocks within a row differ only in convention — locale chooses the words and the shape, the zone chooses the reading. Unknown zones answer an error, verbatim: %q — never a panic, never a fabricated offset.",
				run.Instant, run.ZoneHonesty.Error),
			Facts: zoneFacts,
		},
		{
			Beat: "the honest edges", Verdict: missingVerdict,
			Detail: fmt.Sprintf("an unsupported language (%q — no Korean catalog) walked its chain to the default: %s via %s. A missing key (%s, asked of de) was %s: %q. Both are normal, answerable states — the i18n path never turns a language problem into a 500.",
				run.Fallback.Header, run.Fallback.Selected, run.Fallback.Via, run.MissingKey.Key, missingNote, missingText),
			Facts: []I18nFactRow{
				{Label: "fallback chain", Value: i18nChainText(run.Fallback.Chain)},
				{Label: "missing-key log", Value: run.MissingKey.LogLine},
			},
		},
	}
	return newI18nStoryPayload("i18n-negotiator",
		beats,
		i18nProofs(run),
		"Internationalization in Go is three separable engines behind one request: the NEGOTIATOR turns the Accept-Language header into one supported tag through a chain that starts specific (exact) and ends safe (default); the CATALOG turns keys plus counts into translated text with the locale's own plural rule and placeholder interpolation; and the FORMATTERS render numbers, money, dates, and clocks per locale while time.LoadLocation keeps TIME a separate axis entirely — a zone changes the clock reading, a locale changes everything else. The policies are what make it production-shaped: a language the server does not carry is the DEFAULT catalog's job, a key no catalog carries is the key itself plus a log line, and an unknown timezone is an honest error. Send the demo a different Accept-Language and every beat re-derives from your own header.")
}

func i18nProofs(run I18nNegotiatorRun) []I18nFactRow {
	numberByTag := map[string]string{}
	for _, format := range run.Formats {
		numberByTag[format.Tag] = format.Number
	}
	return []I18nFactRow{
		{Label: "the chain", Value: fmt.Sprintf("exact → base → wildcard → default; this request selected %s via %s", run.Selected, run.Via)},
		{Label: "separators at a glance", Value: strings.Join(i18nFormatProof(numberByTag), " · ")},
		{Label: "zone offsets", Value: i18nZoneProof(run.Zones)},
		{Label: "never a 500", Value: fmt.Sprintf("unsupported → %s via %s; missing key → the key itself + one log line", run.Fallback.Selected, run.Fallback.Via)},
	}
}

func i18nFormatProof(numberByTag map[string]string) []string {
	proof := make([]string, 0, len(numberByTag))
	for _, tag := range []string{"en", "de", "fr", "ja"} {
		if number, ok := numberByTag[tag]; ok {
			proof = append(proof, fmt.Sprintf("%s %s", tag, number))
		}
	}
	return proof
}

func i18nZoneProof(zones []I18nZoneRow) string {
	parts := make([]string, 0, len(zones))
	for _, zone := range zones {
		parts = append(parts, fmt.Sprintf("%s UTC%s", zone.Zone, zone.Offset))
	}
	return strings.Join(parts, " · ") + " — one instant"
}

func i18nChainText(chain []I18nChainRow) string {
	parts := make([]string, 0, len(chain))
	for _, step := range chain {
		if step.Matched != "" {
			parts = append(parts, fmt.Sprintf("%s→%s", step.Range, step.Matched))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s→%s", step.Range, step.Outcome))
	}
	return strings.Join(parts, " ")
}
