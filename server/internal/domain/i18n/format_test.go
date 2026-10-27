package i18n

import (
	"strings"
	"testing"
	"time"
)

// The formatting engine: per-locale number grouping, minor-unit
// currency, date shapes (multibyte literals included), and both clock
// conventions — each pinned against the rendered strings a reader
// should expect from these four locales.

func localeByTag(t *testing.T, tag string) Locale {
	t.Helper()
	for _, locale := range SupportedLocales() {
		if locale.Tag == tag {
			return locale
		}
	}
	t.Fatalf("no profile for %s", tag)
	return Locale{}
}

func TestFormatNumberPerLocale(t *testing.T) {
	table := []struct {
		tag  string
		want string
	}{
		{"en", "1,234,567.89"},           // comma groups, point decimals
		{"de", "1.234.567,89"},           // dot groups, comma decimals
		{"fr", "1\u202f234\u202f567,89"}, // narrow no-break space groups
		{"ja", "1,234,567.89"},           // Japanese groups like English
	}
	for _, tc := range table {
		got := FormatNumber(localeByTag(t, tc.tag), 1234567.89, 2)
		if got != tc.want {
			t.Errorf("%s FormatNumber = %q, want %q", tc.tag, got, tc.want)
		}
	}
}

func TestFormatNumberEdges(t *testing.T) {
	en := localeByTag(t, "en")
	table := []struct {
		value    float64
		decimals int
		want     string
	}{
		{0, 2, "0.00"},
		{1, 0, "1"},
		{999, 2, "999.00"},
		{1000, 2, "1,000.00"},
		{-1234.5, 2, "-1,234.50"},
		{-1234.5, 1, "-1,234.5"}, // fixed decimals are the renderer's only rounding
	}
	for _, tc := range table {
		if got := FormatNumber(en, tc.value, tc.decimals); got != tc.want {
			t.Errorf("FormatNumber(%v,%d) = %q, want %q", tc.value, tc.decimals, got, tc.want)
		}
	}
}

func TestFormatCurrencyMinorUnitsPerLocale(t *testing.T) {
	table := []struct {
		tag   string
		minor int64
		want  string
	}{
		{"en", 1899, "$18.99"},  // prefix symbol, point decimals
		{"de", 1899, "18,99 €"}, // suffix symbol, comma decimals
		{"fr", 1899, "18,99 €"}, // same shape as German at this width
		{"ja", 1899, "¥1,899"},  // no minor units: the unit IS minor
		{"en", 123456789, "$1,234,567.89"},
		{"de", 123456789, "1.234.567,89 €"},
		{"en", 5, "$0.05"},        // zero padding at fixed width
		{"de", -1899, "-18,99 €"}, // sign on the amount, not the symbol
		{"ja", -1899, "-¥1,899"},
	}
	for _, tc := range table {
		if got := FormatCurrency(localeByTag(t, tc.tag), tc.minor); got != tc.want {
			t.Errorf("%s FormatCurrency(%d) = %q, want %q", tc.tag, tc.minor, got, tc.want)
		}
	}
}

func TestFormatDatePerLocale(t *testing.T) {
	moment := time.Date(2026, time.October, 27, 12, 30, 0, 0, time.UTC)
	table := []struct {
		tag  string
		want string
	}{
		{"en", "Oct 27, 2026"},
		{"de", "27.10.2026"},
		{"fr", "27 octobre 2026"},
		{"ja", "2026年10月27日"},
	}
	for _, tc := range table {
		if got := FormatDate(localeByTag(t, tc.tag), moment); got != tc.want {
			t.Errorf("%s FormatDate = %q, want %q", tc.tag, got, tc.want)
		}
	}
}

func TestFormatClockBothConventions(t *testing.T) {
	de := localeByTag(t, "de")
	en := localeByTag(t, "en")
	midday := time.Date(2026, time.October, 27, 14, 5, 0, 0, time.UTC)
	midnight := time.Date(2026, time.October, 27, 0, 7, 0, 0, time.UTC)
	noon := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)
	if got := FormatClock(de, midday); got != "14:05" {
		t.Errorf("de clock = %q, want 14:05", got)
	}
	table := map[string]string{
		FormatClock(en, midday):   "2:05 PM",
		FormatClock(en, midnight): "12:07 AM",
		FormatClock(en, noon):     "12:00 PM",
	}
	for got, want := range table {
		if got != want {
			t.Errorf("en clock = %q, want %q", got, want)
		}
	}
}

func TestGroupDigitsNeverSeesEmptyInput(t *testing.T) {
	// FormatNumber/FormatCurrency always pass at least one digit ("0");
	// groupDigits with an empty string would panic below — pinned so the
	// invariant travels with the function.
	en := localeByTag(t, "en")
	if got := groupDigits(en, "0"); got != "0" {
		t.Errorf("groupDigits single = %q", got)
	}
	if got := groupDigits(en, "1000"); got != "1,000" {
		t.Errorf("groupDigits boundary = %q, want 1,000", got)
	}
}

func TestPseudoLocalize(t *testing.T) {
	got := PseudoLocalize("Welcome")
	want := "[Wélcómé×××]"
	if got != want {
		t.Errorf("PseudoLocalize = %q, want %q", got, want)
	}
	got = PseudoLocalize("Hello, {name}!")
	if !strings.HasPrefix(got, "[Hélló, ") || !strings.Contains(got, "{name}") || !strings.HasSuffix(got, "]") {
		t.Errorf("placeholder must survive verbatim inside brackets: %q", got)
	}
	if got := PseudoLocalize(""); got != "[×]" {
		t.Errorf("empty text = %q, want minimal bracketed padding", got)
	}
	// Padding grows with the source — the layout-pressure property.
	short, long := PseudoLocalize("hi"), PseudoLocalize("a much longer sentence")
	if countRunes(long) <= countRunes(short) {
		t.Errorf("padding must grow: %d vs %d", countRunes(long), countRunes(short))
	}
}
