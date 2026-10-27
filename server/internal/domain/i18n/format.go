package i18n

import (
	"strconv"
	"strings"
	"time"
)

// The formatting half of the guide's i18n engine (topic 27): one Locale
// profile per supported language carrying its presentation conventions,
// and the by-hand renderers for numbers, money, dates, and clocks. This
// is deliberately NOT a formatting library — it is the honest floor a
// service can ship with stdlib alone: grouping separators, decimal
// comma vs point, currency symbol placement, localized month names, and
// 12-vs-24-hour clocks, each stated as data on the profile. When the
// needs grow past this (collation, complex scripts, full CLDR number
// patterns), that is the moment golang.org/x/text earns its import —
// the topic's prose makes the comparison explicitly.

// DateShape names the date convention a locale renders with.
type DateShape string

// The four date shapes the demo locales use. The names describe the
// pattern the renderer emits, not a CLDR pattern string — by hand means
// readable, and the renderer below is the single place that turns a
// shape into text.
const (
	// DateMonthDayYear is the US shape: "Jan 2, 2006".
	DateMonthDayYear DateShape = "month-day-year"
	// DateDayMonthYear is the shape with a leading zero and dots:
	// "02.01.2006".
	DateDayMonthYear DateShape = "day-month-year"
	// DateDayMonthNameYear spells the month: "2 janvier 2006".
	DateDayMonthNameYear DateShape = "day-month-name-year"
	// DateYearMonthDay is the Japanese shape: "2006年1月2日" — note the
	// multibyte literals; Go source is UTF-8 by specification.
	DateYearMonthDay DateShape = "year-month-day"
)

// Locale is one locale's presentation conventions: plural rule, number
// and currency shapes, month names, date shape, and clock convention.
// A Locale is plain data plus two rule fields (Plural, and the shape
// constants), so profiles are values — comparable, copyable, and cheap
// to hand to any renderer.
type Locale struct {
	Tag      string // BCP 47 base tag: "en", "de", "fr", "ja"
	Language string // English name for prose: "English", "German"…
	// Number shapes: the separators between digit groups and around the
	// fraction. English writes 1,234.56; German 1.234,56; French groups
	// with a narrow no-break space (U+202F) and decimals with a comma.
	GroupSeparator   string
	DecimalSeparator string
	// Currency shape: the ISO code (what APIs store and exchange), the
	// symbol (what humans read), whether the symbol trails the amount
	// ("18,99 €"), and how many minor-unit digits the currency carries
	// (2 for dollars and euros, 0 for yen).
	CurrencyCode     string
	CurrencySymbol   string
	CurrencySuffix   bool
	CurrencyDecimals int
	// Date and clock shapes.
	Months    [12]string
	DateShape DateShape
	Clock12   bool // 12-hour clock with AM/PM ("2:30 PM") vs 24-hour ("14:30")
	AmPm      [2]string
	// Plural selects the locale's CLDR cardinal category for a count.
	Plural func(count int) PluralCategory
}

// SupportedLocales returns the guide's four demo locale profiles in
// catalog-preference order (default first). Four is enough to cover
// every convention axis the topic teaches: two separators regimes,
// three date shapes, both clock conventions, and three distinct plural
// rules.
func SupportedLocales() []Locale {
	return []Locale{
		{
			Tag: "en", Language: "English",
			GroupSeparator: ",", DecimalSeparator: ".",
			CurrencyCode: "USD", CurrencySymbol: "$", CurrencyDecimals: 2,
			Months:    [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
			DateShape: DateMonthDayYear, Clock12: true, AmPm: [2]string{"AM", "PM"},
			Plural: pluralOneForOne,
		},
		{
			Tag: "de", Language: "German",
			GroupSeparator: ".", DecimalSeparator: ",",
			CurrencyCode: "EUR", CurrencySymbol: "€", CurrencySuffix: true, CurrencyDecimals: 2,
			Months:    [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
			DateShape: DateDayMonthYear,
			Plural:    pluralOneForOne,
		},
		{
			Tag: "fr", Language: "French",
			// U+202F NARROW NO-BREAK SPACE — the CLDR French group separator:
			// "1 234,56", a space the renderer never lets wrap.
			GroupSeparator: "\u202f", DecimalSeparator: ",",
			CurrencyCode: "EUR", CurrencySymbol: "€", CurrencySuffix: true, CurrencyDecimals: 2,
			Months:    [12]string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
			DateShape: DateDayMonthNameYear,
			Plural:    pluralOneForZeroAndOne,
		},
		{
			Tag: "ja", Language: "Japanese",
			GroupSeparator: ",", DecimalSeparator: ".",
			CurrencyCode: "JPY", CurrencySymbol: "¥", CurrencyDecimals: 0,
			Months:    [12]string{"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
			DateShape: DateYearMonthDay,
			Plural:    pluralAlwaysOther,
		},
	}
}

// groupDigits inserts the locale's group separator every three digits
// from the right of a plain digit string ("1234567" → "1,234,567" for
// English). The leading "-" and any fraction are handled by the callers
// — this function only ever sees digits.
func groupDigits(l Locale, digits string) string {
	var grouped strings.Builder
	first := len(digits) % 3
	if first == 0 {
		first = 3
	}
	grouped.WriteString(digits[:first])
	for position := first; position < len(digits); position += 3 {
		grouped.WriteString(l.GroupSeparator)
		grouped.WriteString(digits[position : position+3])
	}
	return grouped.String()
}

// FormatNumber renders a number with the locale's grouping and decimal
// separators, at the requested fixed decimals — the honest equivalent
// of a CLDR decimal pattern: fixed fractions, threes grouping, sign in
// front. (Formatting money with this is a smell: money has minor units
// and its own digit count — see FormatCurrency.)
func FormatNumber(l Locale, value float64, decimals int) string {
	if decimals < 0 {
		decimals = 0
	}
	text := strconv.FormatFloat(abs(value), 'f', decimals, 64)
	integer, fraction := text, ""
	if separator := strings.IndexByte(text, '.'); separator >= 0 {
		integer, fraction = text[:separator], text[separator+1:]
	}
	out := strings.Builder{}
	if value < 0 {
		out.WriteByte('-')
	}
	out.WriteString(groupDigits(l, integer))
	if fraction != "" {
		out.WriteString(l.DecimalSeparator)
		out.WriteString(fraction)
	}
	return out.String()
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

// FormatCurrency renders a minor-unit amount (cents, sen — money as
// integers, the shape a ledger and a database column should already
// use) with the locale's conventions, in integer arithmetic end to end:
// no float ever stands between the stored value and the rendered
// digits. 1899 minor units of USD is "$18.99"; of EUR in German,
// "18,99 €"; of JPY, "¥1,899" (yen carry no minor units, so the minor
// unit IS the unit).
func FormatCurrency(l Locale, minor int64) string {
	scale := int64(1)
	for i := 0; i < l.CurrencyDecimals; i++ {
		scale *= 10
	}
	units, remainder := minor/scale, minor%scale
	negative := units < 0
	digits := strconv.FormatInt(absInt64(units), 10)
	var out strings.Builder
	if negative {
		out.WriteByte('-')
	}
	if l.CurrencySuffix {
		out.WriteString(groupDigits(l, digits))
		if l.CurrencyDecimals > 0 {
			out.WriteString(l.DecimalSeparator)
			out.WriteString(zeroPadded(int(remainder), l.CurrencyDecimals))
		}
		out.WriteString(" ")
		out.WriteString(l.CurrencySymbol)
		return out.String()
	}
	out.WriteString(l.CurrencySymbol)
	out.WriteString(groupDigits(l, digits))
	if l.CurrencyDecimals > 0 {
		out.WriteString(l.DecimalSeparator)
		out.WriteString(zeroPadded(int(remainder), l.CurrencyDecimals))
	}
	return out.String()
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// zeroPadded renders a value at fixed width ("5" → "05" for two
// decimals). The sign never reaches here — it is taken from the units
// before the remainder is split.
func zeroPadded(value, width int) string {
	text := strconv.Itoa(absInt(value))
	for len(text) < width {
		text = "0" + text
	}
	return text
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// FormatDate renders a date with the locale's shape and month names.
// Callers pass the time in the zone they want it read in — formatting
// never changes the clock, which is the topic's timezone-vs-locale
// distinction in one sentence.
func FormatDate(l Locale, t time.Time) string {
	month := l.Months[int(t.Month())-1]
	switch l.DateShape {
	case DateMonthDayYear:
		return month + " " + strconv.Itoa(t.Day()) + ", " + strconv.Itoa(t.Year())
	case DateDayMonthYear:
		return zeroPadded(t.Day(), 2) + "." + zeroPadded(int(t.Month()), 2) + "." + strconv.Itoa(t.Year())
	case DateDayMonthNameYear:
		return strconv.Itoa(t.Day()) + " " + month + " " + strconv.Itoa(t.Year())
	default: // DateYearMonthDay
		return strconv.Itoa(t.Year()) + "年" + strconv.Itoa(int(t.Month())) + "月" + strconv.Itoa(t.Day()) + "日"
	}
}

// FormatClock renders the time of day with the locale's clock
// convention: "2:30 PM" for twelve-hour English, "14:30" for the other
// three profiles.
func FormatClock(l Locale, t time.Time) string {
	hour, minute := t.Hour(), t.Minute()
	if !l.Clock12 {
		return zeroPadded(hour, 2) + ":" + zeroPadded(minute, 2)
	}
	suffix := l.AmPm[0]
	if hour >= 12 {
		suffix = l.AmPm[1]
	}
	switch {
	case hour == 0:
		hour = 12
	case hour > 12:
		hour -= 12
	}
	return strconv.Itoa(hour) + ":" + zeroPadded(minute, 2) + " " + suffix
}
