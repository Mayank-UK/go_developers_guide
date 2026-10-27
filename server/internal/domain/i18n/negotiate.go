package i18n

import (
	"sort"
	"strings"
)

// The Accept-Language half of the guide's i18n engine (topic 27): parse
// the header the caller actually sent, then walk a fallback chain
// against the catalogs the server actually carries. Everything here is
// stdlib on purpose — golang.org/x/text/language exists, is excellent,
// and is compared against in the topic's prose, but the engine a reader
// can hold in their head fits in this file.

// LanguagePreference is one parsed Accept-Language member: the language
// range (lowercased, trimmed — ranges are case-insensitive by BCP 47),
// its quality value q (1.0 when absent, per the HTTP spec's default),
// and its position in the header. Position breaks q ties: a sender who
// writes "de, en" means German first, and the parser must not lose that.
type LanguagePreference struct {
	Range   string
	Quality float64
	Order   int
}

// IsWildcard reports whether the range is the "*" catch-all.
func (p LanguagePreference) IsWildcard() bool { return p.Range == "*" }

// Base returns the range's primary subtag — the language without region
// or script ("fr-CA" → "fr"). A bare language is its own base.
func (p LanguagePreference) Base() string {
	if tag, _, _ := strings.Cut(p.Range, "-"); tag != "" {
		return tag
	}
	return p.Range
}

// ParseAcceptLanguage parses an Accept-Language header value into
// quality-ordered preferences (stable: equal q keeps the sender's
// order). The parser is deliberately forgiving in the RFC 9110 §12.5.4
// sense: a member it cannot use — blank, malformed range, bad qvalue,
// unknown parameter — is dropped, and the usable members still parse.
// A header that says nothing usable parses to nil, which the negotiator
// turns into the default catalog. Parsing never fails and never panics:
// the header is caller-writable input, and hostile input earns a shrug,
// not a 500.
func ParseAcceptLanguage(header string) []LanguagePreference {
	var prefs []LanguagePreference
	for position, member := range strings.Split(header, ",") {
		pref, ok := parseMember(member, position)
		if !ok {
			continue
		}
		// A repeated range keeps its FIRST occurrence — the sender's
		// earlier word is the one the sort should honor.
		duplicate := false
		for _, existing := range prefs {
			if existing.Range == pref.Range {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		prefs = append(prefs, pref)
	}
	sort.SliceStable(prefs, func(i, j int) bool {
		return prefs[i].Quality > prefs[j].Quality
	})
	return prefs
}

// parseMember parses one comma-separated member ("fr-CH", "en;q=0.8",
// "*;q=0.5"). Whitespace around every token is tolerated ("fr ; q=0.9"
// is what hand-rolled clients send).
func parseMember(member string, position int) (LanguagePreference, bool) {
	segments := strings.Split(strings.TrimSpace(member), ";")
	range_ := normalizeRange(segments[0])
	if range_ == "" {
		return LanguagePreference{}, false
	}
	quality := 1.0
	for _, parameter := range segments[1:] {
		name, value, found := strings.Cut(parameter, "=")
		if !found || strings.ToLower(strings.TrimSpace(name)) != "q" {
			return LanguagePreference{}, false
		}
		parsed, ok := parseQuality(strings.TrimSpace(value))
		if !ok {
			return LanguagePreference{}, false
		}
		quality = parsed
	}
	return LanguagePreference{Range: range_, Quality: quality, Order: position}, true
}

// normalizeRange lowercases and shape-checks one language range. BCP 47
// subtags are 1–8 alphanumerics; the wildcard "*" stands alone. Anything
// else ("fr--CH", "a very long sentence") is not a range this server
// can match, so it drops.
func normalizeRange(raw string) string {
	range_ := strings.ToLower(strings.TrimSpace(raw))
	if range_ == "*" {
		return range_
	}
	if range_ == "" {
		return ""
	}
	for _, subtag := range strings.Split(range_, "-") {
		if len(subtag) < 1 || len(subtag) > 8 {
			return ""
		}
		for _, r := range subtag {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				return ""
			}
		}
	}
	return range_
}

// parseQuality parses one qvalue per the HTTP grammar: 0 to 1, at most
// three decimal digits, no sign. "0.85" is fine; "2", "0.1234", "-1",
// and "abc" are not — a member carrying one is dropped whole, because a
// qvalue the server had to guess is a preference the sender did not
// state.
func parseQuality(raw string) (float64, bool) {
	if raw == "" || len(raw) > 5 || strings.HasPrefix(raw, "-") {
		return 0, false
	}
	whole, fraction, hasFraction := strings.Cut(raw, ".")
	if whole != "0" && whole != "1" {
		return 0, false
	}
	if !hasFraction {
		return float64(whole[0] - '0'), true
	}
	if len(fraction) < 1 || len(fraction) > 3 {
		return 0, false
	}
	value := float64(whole[0] - '0')
	for i, r := range fraction {
		if r < '0' || r > '9' {
			return 0, false
		}
		value += float64(r-'0') / pow10(i+1)
	}
	return value, value <= 1
}

// pow10 is the one float power the qvalue parser needs — 10, 100, 1000.
// (math.Pow works too; this keeps the arithmetic visibly exact.)
func pow10(exponent int) float64 {
	value := 1.0
	for i := 0; i < exponent; i++ {
		value *= 10
	}
	return value
}

// ── negotiation ──────────────────────────────────────────────────────────

// The ways a negotiator can land on a tag — the fallback chain's links,
// most to least specific.
const (
	// ViaExact: the preference's full range equals a supported tag
	// ("de" offered, "de" carried).
	ViaExact = "exact"
	// ViaBase: the preference's primary subtag equals a supported tag
	// ("fr-CA" offered, "fr" carried) — the regional variant is better
	// served by the base language than by the default catalog.
	ViaBase = "base"
	// ViaWildcard: the "*" range matched — "anything but my exclusions";
	// the server answers with its default catalog.
	ViaWildcard = "wildcard"
	// ViaDefault: nothing matched at all; the default catalog is the
	// never-fail last link.
	ViaDefault = "default"
	// OutcomeNone marks a chain step that matched nothing.
	OutcomeNone = "none"
	// OutcomeExcluded marks a step whose q=0 range removed candidates.
	OutcomeExcluded = "excluded"
)

// ChainStep is one preference the negotiator examined and what came of
// it — the audit trail the demo (and your logs) can replay.
type ChainStep struct {
	Range   string
	Quality float64
	Outcome string // ViaExact / ViaBase / ViaWildcard / OutcomeNone / OutcomeExcluded
	Matched string // the supported tag chosen, when the step matched
}

// NegotiationResult is a negotiation outcome: the selected supported
// tag, the chain link that selected it, and every step walked in order.
type NegotiationResult struct {
	Selected string
	Via      string
	Chain    []ChainStep
}

// Negotiate walks the parsed preferences against the supported tags in
// three passes, most specific first: an EXACT range/tag match, then a
// BASE-language match (the range's primary subtag against the tags,
// then the tags' primary subtags — "fr-CA" finds "fr" either way), and
// only then the wildcard. A q=0 range removes the tags it matches
// (prefix-wise: "en;q=0" excludes "en-GB" too) before any positive
// match is considered. When nothing matches, the default tag serves and
// Via says so — negotiation never errors, because an unsupported
// language is a normal, answerable state, not a failure.
func Negotiate(header string, supported []string, defaultTag string) NegotiationResult {
	prefs := ParseAcceptLanguage(header)

	excluded := make(map[string]bool)
	for _, pref := range prefs {
		if pref.Quality > 0 {
			continue
		}
		for _, tag := range supported {
			if rangeMatchesTag(pref.Range, tag) {
				excluded[tag] = true
			}
		}
	}

	result := NegotiationResult{Selected: defaultTag, Via: ViaDefault}
	record := func(pref LanguagePreference, outcome, matched string) bool {
		result.Chain = append(result.Chain, ChainStep{
			Range: pref.Range, Quality: pref.Quality, Outcome: outcome, Matched: matched,
		})
		return outcome != OutcomeNone && outcome != OutcomeExcluded
	}

	for _, pref := range prefs {
		if pref.Quality == 0 {
			record(pref, OutcomeExcluded, "")
			continue
		}
		if pref.IsWildcard() {
			// The wildcard means "anything I have not refused" — the
			// server's answer is its default catalog, unless the sender
			// refused exactly that (a q=0 default excluded it).
			if excluded[defaultTag] {
				record(pref, OutcomeNone, "")
				continue
			}
			if record(pref, ViaWildcard, defaultTag) {
				result.Selected, result.Via = defaultTag, ViaWildcard
				return result
			}
			continue
		}
		for _, tag := range supported {
			if excluded[tag] {
				continue
			}
			if strings.EqualFold(pref.Range, tag) {
				if record(pref, ViaExact, tag) {
					result.Selected, result.Via = tag, ViaExact
					return result
				}
			}
		}
		for _, tag := range supported {
			if excluded[tag] {
				continue
			}
			if pref.Base() == baseOf(tag) {
				if record(pref, ViaBase, tag) {
					result.Selected, result.Via = tag, ViaBase
					return result
				}
			}
		}
		record(pref, OutcomeNone, "")
	}
	return result
}

// rangeMatchesTag answers whether a basic language range covers a tag:
// equal, or the range is a prefix of the tag at a subtag boundary
// ("en" covers "en-GB"). The wildcard covers everything; used by the
// q=0 exclusions.
func rangeMatchesTag(range_, tag string) bool {
	if range_ == "*" {
		return true
	}
	lower := strings.ToLower(tag)
	return lower == range_ || strings.HasPrefix(lower, range_+"-")
}

// baseOf returns a tag's primary subtag.
func baseOf(tag string) string {
	if base, _, _ := strings.Cut(tag, "-"); base != "" {
		return strings.ToLower(base)
	}
	return strings.ToLower(tag)
}
