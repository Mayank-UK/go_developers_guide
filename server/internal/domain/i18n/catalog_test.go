package i18n

import (
	"strings"
	"testing"
)

// The catalog engine: plural boundaries at 0/1/2 per locale (the three
// rules the guide carries), interpolation's missing-arg honesty, the
// constructor's wiring validation, and the missing-key policy — fall
// back to the key, log the notice, never panic, never 500.

func demoMessages() map[string]map[string]Message {
	return map[string]map[string]Message{
		"en": {
			"demo.greeting":  {Forms: map[PluralCategory]string{PluralOther: "Welcome back, {name}!"}},
			"demo.new_posts": {Forms: map[PluralCategory]string{PluralOne: "You have {count} new post.", PluralOther: "You have {count} new posts."}},
		},
		"de": {
			"demo.greeting":  {Forms: map[PluralCategory]string{PluralOther: "Willkommen zurück, {name}!"}},
			"demo.new_posts": {Forms: map[PluralCategory]string{PluralOne: "Du hast {count} neuen Beitrag.", PluralOther: "Du hast {count} neue Beiträge."}},
		},
		"fr": {
			"demo.greeting":  {Forms: map[PluralCategory]string{PluralOther: "Bienvenue, {name} !"}},
			"demo.new_posts": {Forms: map[PluralCategory]string{PluralOne: "Tu as {count} nouvelle publication.", PluralOther: "Tu as {count} nouvelles publications."}},
		},
		"ja": {
			"demo.greeting":  {Forms: map[PluralCategory]string{PluralOther: "おかえりなさい、{name}さん！"}},
			"demo.new_posts": {Forms: map[PluralCategory]string{PluralOther: "新しい投稿が{count}件あります。"}},
		},
	}
}

func newDemoTranslator(t *testing.T, notices *[]string) *Translator {
	t.Helper()
	catalog, err := NewMapCatalog(demoMessages(), []string{"en", "de", "fr", "ja"})
	if err != nil {
		t.Fatalf("map catalog: %v", err)
	}
	var onMissing MissingKeyFunc
	if notices != nil {
		onMissing = func(tag, key string) { *notices = append(*notices, tag+" "+key) }
	}
	translator, err := NewTranslator(catalog, SupportedLocales(), "en", onMissing)
	if err != nil {
		t.Fatalf("translator: %v", err)
	}
	return translator
}

func TestPluralBoundariesPerLocale(t *testing.T) {
	table := []struct {
		tag            string
		zero, one, two PluralCategory
	}{
		{"en", PluralOther, PluralOne, PluralOther},   // exactly 1 is singular
		{"de", PluralOther, PluralOne, PluralOther},   // same rule as English
		{"fr", PluralOne, PluralOne, PluralOther},     // 0 AND 1 are singular
		{"ja", PluralOther, PluralOther, PluralOther}, // no plural distinction
	}
	for _, tc := range table {
		var profile Locale
		for _, candidate := range SupportedLocales() {
			if candidate.Tag == tc.tag {
				profile = candidate
			}
		}
		for count, want := range map[int]PluralCategory{0: tc.zero, 1: tc.one, 2: tc.two} {
			if got := profile.Plural(count); got != want {
				t.Errorf("%s plural(%d) = %q, want %q", tc.tag, count, got, want)
			}
		}
	}
}

func TestTranslateSelectsPluralFormsAndInterpolates(t *testing.T) {
	translator := newDemoTranslator(t, nil)
	got := translator.Translate("en", "demo.new_posts", 1, map[string]string{"count": "1"})
	if got.Text != "You have 1 new post." || got.Served != "en" || got.Fallback || !got.Found {
		t.Errorf("en/1 = %+v", got)
	}
	if got := translator.Translate("en", "demo.new_posts", 2, map[string]string{"count": "2"}); got.Text != "You have 2 new posts." {
		t.Errorf("en/2 = %+v", got)
	}
	// French 0 is singular — the category the count selects, verbatim.
	if got := translator.Translate("fr", "demo.new_posts", 0, map[string]string{"count": "0"}); got.Text != "Tu as 0 nouvelle publication." {
		t.Errorf("fr/0 = %+v", got)
	}
	// Japanese carries one form; count 2 interpolates into it.
	if got := translator.Translate("ja", "demo.new_posts", 2, map[string]string{"count": "2"}); got.Text != "新しい投稿が2件あります。" {
		t.Errorf("ja/2 = %+v", got)
	}
}

func TestTranslateResolvesLocaleFallbacks(t *testing.T) {
	translator := newDemoTranslator(t, nil)
	// Regional tag resolves to its base locale and serves it.
	if got := translator.Translate("fr-CA", "demo.greeting", 1, map[string]string{"name": "Ada"}); got.Served != "fr" || got.Fallback {
		t.Errorf("fr-CA should serve fr directly: %+v", got)
	}
	// Unknown locale falls back to the default LOCALE (en) with the flag.
	if got := translator.Translate("ko", "demo.greeting", 1, map[string]string{"name": "Ada"}); got.Served != "en" || !got.Fallback || !got.Found {
		t.Errorf("ko must fall back to en: %+v", got)
	}
	// Case and whitespace are the caller's business, not the catalog's.
	if got := translator.Translate("  DE ", "demo.greeting", 1, map[string]string{"name": "Ada"}); got.Served != "de" {
		t.Errorf("trimmed/upper tag must resolve: %+v", got)
	}
}

func TestTranslateMissingKeyPolicyServesTheKeyAndLogs(t *testing.T) {
	var notices []string
	translator := newDemoTranslator(t, &notices)
	got := translator.Translate("de", "demo.never_translated", 1, nil)
	if got.Text != "demo.never_translated" || got.Found || !got.Fallback {
		t.Errorf("missing key must serve itself: %+v", got)
	}
	if len(notices) != 1 || notices[0] != "de demo.never_translated" {
		t.Errorf("notices = %v, want one de notice", notices)
	}
}

func TestTranslateFallsBackToDefaultCatalogForHalfTranslatedLocale(t *testing.T) {
	catalog, err := NewMapCatalog(map[string]map[string]Message{
		"en": {"demo.greeting": {Forms: map[PluralCategory]string{PluralOther: "Welcome back, {name}!"}}},
		"de": {}, // German exists but does not carry this key.
	}, []string{"en", "de"})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	locales := SupportedLocales()[:2] // en, de profiles
	translator, err := NewTranslator(catalog, locales, "en", nil)
	if err != nil {
		t.Fatalf("translator: %v", err)
	}
	got := translator.Translate("de", "demo.greeting", 1, map[string]string{"name": "Ada"})
	if got.Text != "Welcome back, Ada!" || got.Served != "en" || !got.Fallback || !got.Found {
		t.Errorf("half-translated locale must serve the default's message: %+v", got)
	}
}

func TestTranslateMissingFormFallsToOther(t *testing.T) {
	catalog, err := NewMapCatalog(map[string]map[string]Message{
		"en": {"demo.only_other": {Forms: map[PluralCategory]string{PluralOther: "always this"}}},
	}, []string{"en"})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	translator, err := NewTranslator(catalog, SupportedLocales()[:1], "en", nil)
	if err != nil {
		t.Fatalf("translator: %v", err)
	}
	if got := translator.Translate("en", "demo.only_other", 1, nil); got.Text != "always this" || !got.Found {
		t.Errorf("missing 'one' form must fall to other: %+v", got)
	}
}

func TestInterpolate(t *testing.T) {
	vars := map[string]string{"name": "Ada", "count": "2"}
	table := map[string]string{
		"Welcome back, {name}!":   "Welcome back, Ada!",
		"{count} × {count}":       "2 × 2",
		"{ name }":                "Ada", // placeholder names trim
		"no placeholders":         "no placeholders",
		"unknown stays {visible}": "unknown stays {visible}", // honesty over silence
		"unclosed { brace":        "unclosed { brace",
		"nested {outer {inner}}":  "nested {outer {inner}}", // nested braces are not placeholders
		"名前：{name}":               "名前：Ada",                 // multibyte text, ASCII placeholder
	}
	for text, want := range table {
		if got := Interpolate(text, vars); got != want {
			t.Errorf("Interpolate(%q) = %q, want %q", text, got, want)
		}
	}
	if got := Interpolate("hi {name}", nil); got != "hi {name}" {
		t.Errorf("nil vars must leave text untouched: %q", got)
	}
}

func TestNewTranslatorValidatesWiring(t *testing.T) {
	catalog, err := NewMapCatalog(demoMessages(), []string{"en", "de", "fr", "ja"})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := NewTranslator(nil, SupportedLocales(), "en", nil); err == nil || !strings.Contains(err.Error(), "catalog is nil") {
		t.Errorf("nil catalog must be refused: %v", err)
	}
	if _, err := NewTranslator(catalog, nil, "en", nil); err == nil || !strings.Contains(err.Error(), "no locale profiles") {
		t.Errorf("no locales must be refused: %v", err)
	}
	// A catalog tag without a locale profile is a wiring bug: fail at boot.
	if _, err := NewTranslator(catalog, SupportedLocales()[:3], "en", nil); err == nil || !strings.Contains(err.Error(), "no locale profile") {
		t.Errorf("missing profile must be refused: %v", err)
	}
	// The default must be carried by both sides.
	if _, err := NewTranslator(catalog, SupportedLocales(), "ko", nil); err == nil || !strings.Contains(err.Error(), "default tag") {
		t.Errorf("unknown default must be refused: %v", err)
	}
}

func TestNewMapCatalogValidates(t *testing.T) {
	if _, err := NewMapCatalog(demoMessages(), nil); err == nil || !strings.Contains(err.Error(), "no tags") {
		t.Errorf("empty tags must be refused: %v", err)
	}
	if _, err := NewMapCatalog(demoMessages(), []string{"en", "en"}); err == nil || !strings.Contains(err.Error(), "duplicate tag") {
		t.Errorf("duplicate tags must be refused: %v", err)
	}
	if _, err := NewMapCatalog(map[string]map[string]Message{
		"en": {"demo.broken": {Forms: map[PluralCategory]string{PluralOne: "only singular"}}},
	}, []string{"en"}); err == nil || !strings.Contains(err.Error(), `"other" form`) {
		t.Errorf("messages without an other form must be refused: %v", err)
	}
	if _, err := NewMapCatalog(map[string]map[string]Message{
		"en": {}, "zz": {"demo.stray": {Forms: map[PluralCategory]string{PluralOther: "x"}}},
	}, []string{"en"}); err == nil || !strings.Contains(err.Error(), "unlisted tag") {
		t.Errorf("messages for unlisted tags must be refused: %v", err)
	}
}
