package i18n

import (
	"math"
	"reflect"
	"testing"
)

// The Accept-Language parser, table-tested against the examples RFC
// 9110 §12.5.4 itself carries (they date to RFC 7231 and survive into
// the current HTTP spec verbatim) plus the hostile-input rows a public
// endpoint earns: case, whitespace, duplicates, malformed qvalues, and
// ranges that are not language ranges at all.

func ranges(prefs []LanguagePreference) []string {
	out := make([]string, 0, len(prefs))
	for _, pref := range prefs {
		out = append(out, pref.Range)
	}
	return out
}

func TestParseAcceptLanguageRFCExamples(t *testing.T) {
	table := []struct {
		name   string
		header string
		want   []string // ranges in expected order
	}{
		{"the spec's five-member example", "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5",
			[]string{"fr-ch", "fr", "en", "de", "*"}},
		{"the spec's Danish example", "da, en-gb;q=0.8, en;q=0.7",
			[]string{"da", "en-gb", "en"}},
		{"missing q means one", "de", []string{"de"}},
		{"uppercase folds", "EN-US, ZH-Hant;q=0.5", []string{"en-us", "zh-hant"}},
		{"whitespace tolerated", "fr ; q=0.9 , de", []string{"de", "fr"}},
		{"q orders the list, position breaks ties", "en;q=0.4, de;q=0.9, fr;q=0.4", []string{"de", "en", "fr"}},
		{"wildcard alone", "*", []string{"*"}},
		{"empty header parses to nothing", "", nil},
		{"blank header parses to nothing", "   ", nil},
		{"trailing comma drops cleanly", "fr,", []string{"fr"}},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			got := ranges(ParseAcceptLanguage(tc.header))
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ranges = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseAcceptLanguageQualities(t *testing.T) {
	prefs := ParseAcceptLanguage("fr-CH, fr;q=0.85, en;q=0.8, *;q=0.5")
	want := []float64{1, 0.85, 0.8, 0.5}
	for i, expected := range want {
		if math.Abs(prefs[i].Quality-expected) > 1e-9 {
			t.Errorf("prefs[%d].Quality = %v, want %v", i, prefs[i].Quality, expected)
		}
	}
	if prefs[0].Order != 0 || prefs[3].Order != 3 {
		t.Errorf("positions must survive the sort: %+v", prefs)
	}
}

func TestParseAcceptLanguageDropsUnusableMembers(t *testing.T) {
	table := []struct {
		name   string
		header string
		want   []string
	}{
		{"malformed qvalue drops its member", "fr;q=abc, de", []string{"de"}},
		{"q above one drops its member", "fr;q=2", nil},
		{"negative q drops its member", "fr;q=-1", nil},
		{"fourth decimal digit drops its member", "fr;q=0.1234", nil},
		{"empty segment drops", "fr;;q=0.5,de", []string{"de"}},
		{"empty subtag drops its member", "fr--CH, de", []string{"de"}},
		{"oversized subtag drops its member", "averylongsubtag, de", []string{"de"}},
		{"unknown parameter drops its member", "fr;x=1, de", []string{"de"}},
		{"bare q without value drops its member", "fr;q=, de", []string{"de"}},
		{"a space inside a range is not a range", "en US, de", []string{"de"}},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			got := ranges(ParseAcceptLanguage(tc.header))
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ranges = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseAcceptLanguageKeepsFirstDuplicate(t *testing.T) {
	prefs := ParseAcceptLanguage("de;q=0.8, fr, de;q=0.9")
	got := ranges(prefs)
	if !reflect.DeepEqual(got, []string{"fr", "de"}) {
		t.Errorf("ranges = %v, want the FIRST de (q 0.8) dropped the later duplicate", got)
	}
	if prefs[1].Quality != 0.8 {
		t.Errorf("duplicate resolution changed the kept q: %v", prefs[1].Quality)
	}
}

func TestBase(t *testing.T) {
	table := map[string]string{"fr-CA": "fr", "en": "en", "zh-Hant-TW": "zh", "*": "*"}
	for range_, base := range table {
		if got := (LanguagePreference{Range: range_}).Base(); got != base {
			t.Errorf("Base(%q) = %q, want %q", range_, got, base)
		}
	}
}

// ── negotiation ──────────────────────────────────────────────────────────

var negotiateSupported = []string{"en", "de", "fr", "ja"}

func TestNegotiateFallbackChain(t *testing.T) {
	table := []struct {
		name      string
		header    string
		supported []string
		selected  string
		via       string
	}{
		{"exact match first", "de", nil, "de", ViaExact},
		{"q orders candidates", "en;q=0.4, de;q=0.9", nil, "de", ViaExact},
		{"regional variant falls to base", "fr-CA, en;q=0.8", nil, "fr", ViaBase},
		{"supported regional matches exact", "en-GB, de;q=0.9", []string{"en", "en-GB", "de"}, "en-GB", ViaExact},
		{"the spec's example prefers fr-CH via base fr", "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5", nil, "fr", ViaBase},
		{"wildcard serves the default", "ko, *;q=0.5", nil, "en", ViaWildcard},
		{"unsupported language serves the default", "ko-KR,ko;q=0.9", nil, "en", ViaDefault},
		{"empty header serves the default", "", nil, "en", ViaDefault},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			supported := tc.supported
			if supported == nil {
				supported = negotiateSupported
			}
			got := Negotiate(tc.header, supported, "en")
			if got.Selected != tc.selected || got.Via != tc.via {
				t.Errorf("negotiate = %s via %s, want %s via %s (chain %+v)",
					got.Selected, got.Via, tc.selected, tc.via, got.Chain)
			}
		})
	}
}

func TestNegotiateQZeroExcludes(t *testing.T) {
	// de refused outright; its base match must never win.
	got := Negotiate("de;q=0, en;q=0.8", negotiateSupported, "en")
	if got.Selected != "en" || got.Via != ViaExact {
		t.Errorf("de;q=0 must exclude de: %+v", got)
	}
	// A refused BASE excludes its regions too (BCP 47 prefix rule):
	// "en;q=0" refuses en-GB even though en-GB was offered at q=1.
	got = Negotiate("en-GB;q=1, en;q=0, de;q=0.9", negotiateSupported, "en")
	if got.Selected != "de" {
		t.Errorf("refused base must exclude its region: selected %s, want de (%+v)", got.Selected, got.Chain)
	}
	// Refusing the default makes the wildcard useless.
	got = Negotiate("ko, en;q=0, *;q=0.5", negotiateSupported, "en")
	if got.Selected != "en" || got.Via != ViaDefault {
		t.Errorf("wildcard must not resurrect a refused default: %+v", got)
	}
}

func TestNegotiateChainRecordsEveryStep(t *testing.T) {
	got := Negotiate("ko-KR,ko;q=0.9, *;q=0.1", negotiateSupported, "en")
	if len(got.Chain) != 3 {
		t.Fatalf("chain = %+v, want three steps (ko-kr, ko, wildcard)", got.Chain)
	}
	// The chain steps: ko-kr no exact/base match, ko no match, wildcard
	// selects the default. ko;q=0.9 outranks *;q=0.1 — both are walked,
	// ko first.
	if got.Chain[0].Range != "ko-kr" || got.Chain[0].Outcome != OutcomeNone {
		t.Errorf("step 0 = %+v", got.Chain[0])
	}
	if got.Chain[1].Range != "ko" || got.Chain[1].Outcome != OutcomeNone {
		t.Errorf("step 1 = %+v", got.Chain[1])
	}
	if got.Chain[2].Range != "*" || got.Chain[2].Outcome != ViaWildcard || got.Chain[2].Matched != "en" {
		t.Errorf("step 2 = %+v", got.Chain[2])
	}
}

func TestNegotiateNeverPanicsOnHostileInput(t *testing.T) {
	hostile := []string{
		",,,,,", ";;; ;q=;;", "a;", "*;q=", "0-0-0-0-0-0-0-0;q=0.999",
		"en;q=1.000, en;q=1.0", "*;q=0.5,*;q=0.4", "((((((((((",
	}
	for _, header := range hostile {
		got := Negotiate(header, negotiateSupported, "en")
		if got.Selected != "en" {
			t.Errorf("hostile %q selected %q, want the default en", header, got.Selected)
		}
	}
}

func TestParseQualityBoundaries(t *testing.T) {
	valid := map[string]float64{"0": 0, "1": 1, "0.0": 0, "1.0": 1, "0.001": 0.001, "0.999": 0.999, "1.000": 1}
	for raw, want := range valid {
		got, ok := parseQuality(raw)
		if !ok || math.Abs(got-want) > 1e-9 {
			t.Errorf("parseQuality(%q) = %v,%v want %v,true", raw, got, ok, want)
		}
	}
	invalid := []string{"", "2", "0.1234", "-1", "abc", "1.1", "0.", ".5", "01"}
	for _, raw := range invalid {
		if got, ok := parseQuality(raw); ok {
			t.Errorf("parseQuality(%q) = %v, want rejection", raw, got)
		}
	}
}
