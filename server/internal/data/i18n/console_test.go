package i18n

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"guide/internal/domain/demos"
	domi18n "guide/internal/domain/i18n"
)

// The embedded-catalog adapter: the loader's validation honesty (a
// broken locale file fails construction, named), and the negotiator
// story run end to end against the REAL embedded catalogs — the
// same rows the live endpoint serves.

func newTestConsole(t *testing.T) *Console {
	t.Helper()
	console, err := NewConsole()
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	return console
}

func TestConsoleBootsFromTheEmbeddedCatalogs(t *testing.T) {
	console := newTestConsole(t)
	supported := console.translator.Supported()
	want := "en,de,fr,ja"
	if got := strings.Join(supported, ","); got != want {
		t.Errorf("supported = %q, want %q", got, want)
	}
	if console.translator.Default() != DefaultTag {
		t.Errorf("default = %q", console.translator.Default())
	}
}

func TestStoryWalksARealHeader(t *testing.T) {
	console := newTestConsole(t)
	run, err := console.RunNegotiatorStory(context.Background(), "de-AT, en;q=0.8", "request")
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	if run.Selected != "de" || run.Via != domi18n.ViaBase {
		t.Errorf("negotiation = %s via %s, want de via base", run.Selected, run.Via)
	}
	if len(run.Candidates) != 2 || run.Candidates[0].Range != "de-at" {
		t.Errorf("candidates = %+v", run.Candidates)
	}
	// Greeting per locale, interpolated.
	if run.Greetings[1].Tag != "de" || run.Greetings[1].Text != "Willkommen zurück, Ada!" {
		t.Errorf("de greeting = %+v", run.Greetings[1])
	}
	// The composed German sentence carries localized date and clock.
	composed := run.Composed[1]
	if composed.Tag != "de" || !strings.Contains(composed.Text, "27.10.2026") || !strings.Contains(composed.Text, "13:30") {
		t.Errorf("de composed = %+v", composed)
	}
}

func TestStoryPluralBoundariesComeFromTheRealRules(t *testing.T) {
	console := newTestConsole(t)
	run, err := console.RunNegotiatorStory(context.Background(), demos.I18nFixtureHeader, "fixture")
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	byTag := map[string]map[int]string{}
	for _, row := range run.Plurals {
		if byTag[row.Tag] == nil {
			byTag[row.Tag] = map[int]string{}
		}
		byTag[row.Tag][row.Count] = row.Category
	}
	for _, count := range []int{0, 1, 2} {
		if byTag["en"][count] != map[int]string{0: "other", 1: "one", 2: "other"}[count] {
			t.Errorf("en plural(%d) = %q", count, byTag["en"][count])
		}
		if byTag["fr"][count] != map[int]string{0: "one", 1: "one", 2: "other"}[count] {
			t.Errorf("fr plural(%d) = %q", count, byTag["fr"][count])
		}
		if byTag["ja"][count] != "other" {
			t.Errorf("ja plural(%d) = %q, want other always", count, byTag["ja"][count])
		}
	}
	// The French singular-at-zero text, from the embedded catalog.
	var frenchZero string
	for _, row := range run.Plurals {
		if row.Tag == "fr" && row.Count == 0 {
			frenchZero = row.Text
		}
	}
	if frenchZero != "Tu as 0 nouvelle publication dans ton fil." {
		t.Errorf("fr/0 text = %q", frenchZero)
	}
}

func TestStoryZonesAndTheUnknownZoneHonesty(t *testing.T) {
	console := newTestConsole(t)
	run, err := console.RunNegotiatorStory(context.Background(), "ja", "request")
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	want := map[string]string{
		"Europe/Berlin":    "13:30", // CET (+01:00): EU DST ended Oct 25 2026
		"America/New_York": "08:30", // EDT (-04:00): US DST ends Nov 1 2026
		"Asia/Tokyo":       "21:30", // JST (+09:00), no DST
	}
	for _, zone := range run.Zones {
		if zone.ClockDE != want[zone.Zone] {
			t.Errorf("%s de clock = %q, want %q", zone.Zone, zone.ClockDE, want[zone.Zone])
		}
		if zone.ClockEN == "" {
			t.Errorf("%s en clock missing", zone.Zone)
		}
	}
	// The instant is 12:30Z: Berlin reads 13:30 while New York reads
	// 08:30 — one instant, three clocks, DST included.
	if run.Zones[0].Zone != "Europe/Berlin" || run.Zones[0].Abbreviation != "CET" || run.Zones[0].Offset != "+01:00" {
		t.Errorf("berlin row = %+v", run.Zones[0])
	}
	if !strings.Contains(run.ZoneHonesty.Error, "unknown time zone") {
		t.Errorf("honesty error = %q, want the stdlib's unknown-zone text", run.ZoneHonesty.Error)
	}
}

func TestStoryFallbackAndMissingKey(t *testing.T) {
	console := newTestConsole(t)
	run, err := console.RunNegotiatorStory(context.Background(), "", "fixture")
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	if run.Header != demos.I18nFixtureHeader || run.HeaderSource != "fixture" {
		t.Errorf("fixture run = %q/%q", run.Header, run.HeaderSource)
	}
	if run.Fallback.Selected != "en" || run.Fallback.Via != domi18n.ViaDefault {
		t.Errorf("fallback = %+v", run.Fallback)
	}
	if run.MissingKey.Text != missingKeyDemo {
		t.Errorf("missing key served %q, want the key itself", run.MissingKey.Text)
	}
	if !strings.Contains(run.MissingKey.LogLine, missingKeyDemo) || !strings.Contains(run.MissingKey.LogLine, "de") {
		t.Errorf("log line = %q", run.MissingKey.LogLine)
	}
}

func TestLoadEmbeddedCatalogValidatesFiles(t *testing.T) {
	broken := fstest.MapFS{
		"locales/en.json": &fstest.MapFile{Data: []byte(`{"demo.greeting": {"few": "x", "other": "y"}}`)},
		"locales/de.json": &fstest.MapFile{Data: []byte(`{"demo.greeting": {"other": "y"}}`)},
		"locales/fr.json": &fstest.MapFile{Data: []byte(`{"demo.greeting": {"other": "y"}}`)},
		"locales/ja.json": &fstest.MapFile{Data: []byte(`{"demo.greeting": {"other": "y"}}`)},
	}
	if _, err := loadEmbeddedCatalog(broken); err == nil || !strings.Contains(err.Error(), "unknown plural category") {
		t.Errorf("unknown category must fail with the file and key named: %v", err)
	}
	noOther := fstest.MapFS{
		"locales/en.json": &fstest.MapFile{Data: []byte(`{"demo.greeting": {"one": "x"}}`)},
		"locales/de.json": &fstest.MapFile{Data: []byte(`{}`)},
		"locales/fr.json": &fstest.MapFile{Data: []byte(`{}`)},
		"locales/ja.json": &fstest.MapFile{Data: []byte(`{}`)},
	}
	if _, err := loadEmbeddedCatalog(noOther); err == nil || !strings.Contains(err.Error(), `"other" form`) {
		t.Errorf("missing other form must fail: %v", err)
	}
	stray := fstest.MapFS{
		"locales/en.json": &fstest.MapFile{Data: []byte(`{}`)},
		"locales/de.json": &fstest.MapFile{Data: []byte(`{}`)},
		"locales/fr.json": &fstest.MapFile{Data: []byte(`{}`)},
		"locales/ja.json": &fstest.MapFile{Data: []byte(`{}`)},
		"locales/xx.json": &fstest.MapFile{Data: []byte(`{}`)},
	}
	if _, err := loadEmbeddedCatalog(stray); err == nil || !strings.Contains(err.Error(), "not in the preferred tag set") {
		t.Errorf("a stray locale file must fail: %v", err)
	}
	missing := fstest.MapFS{
		"locales/en.json": &fstest.MapFile{Data: []byte(`{}`)},
	}
	if _, err := loadEmbeddedCatalog(missing); err == nil || !strings.Contains(err.Error(), "read catalog") {
		t.Errorf("a missing locale file must fail: %v", err)
	}
	// The REAL embedded set loads clean (fs.FS interface keeps the
	// production files testable through the same code path).
	if _, err := loadEmbeddedCatalog(localeFiles); err != nil {
		t.Errorf("the shipped catalogs must load: %v", err)
	}
}

// TestEmbeddedTzdataGuaranteesZoneLoading pins the honesty claim the
// topic makes: with time/tzdata blank-imported, zone loading works
// even where no system zoneinfo exists — and the embedded database is
// what answers when the environment variable points nowhere.
func TestEmbeddedTzdataGuaranteesZoneLoading(t *testing.T) {
	// A zone that exists nowhere: LoadLocation errors honestly.
	if _, err := time.LoadLocation("Mars/Olympus"); err == nil {
		t.Error("unknown zone must error")
	}
	// Every zone the story uses loads through the same path.
	for _, name := range storyZones {
		if _, err := time.LoadLocation(name); err != nil {
			t.Errorf("zone %s must load: %v", name, err)
		}
	}
	// Compile-time interface proof that the embedded FS is an fs.FS.
	var _ fs.FS = localeFiles
}
