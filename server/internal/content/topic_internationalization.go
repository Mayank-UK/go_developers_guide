package content

import (
	"errors"
	"fmt"
)

// This file is topic 27 internationalization (group media-realtime,
// curriculum order 27): the language half of going global — Go's
// UTF-8-everywhere default and the byte-vs-rune honesty it demands
// (demo i18n:negotiator walks one request's Accept-Language from raw
// text to a served message); Accept-Language negotiation per RFC 9110
// §12.5.4 with a real parser (q-values, wildcards, duplicates,
// malformed members dropped — never guessed for) and a fallback chain
// that runs exact → base language → wildcard → default; message
// catalogs with Go source as the store (maps, then embed.FS JSON),
// hand-rolled plural rules, {placeholder} interpolation, and a
// missing-key policy that serves the key and logs — never a 500; the
// formatting floor (numbers, integer-minor-unit money, dates, clocks)
// stated as data on a Locale profile; timezones vs locales as two
// orthogonal axes with time.LoadLocation, the tzdata story, and honest
// unknown-zone errors; and the translation workflow from extract to
// ship with pseudo-locale testing. The terminal companion closes with
// the curl tour: one endpoint, many Accept-Language headers.

// internationalizationTopic builds topic 27.
func internationalizationTopic() (Topic, error) {
	var buildErr error
	collect := func(what string, err error) {
		if err != nil {
			buildErr = errors.Join(buildErr, fmt.Errorf("%s: %w", what, err))
		}
	}
	text := func(id, body string) TextBlock {
		block, err := NewTextBlock(id, body)
		collect("text "+id, err)
		return block
	}
	code := func(id, language, caption, source, snippet string) CodeBlock {
		block, err := NewCodeBlock(id, language, caption, source, snippet)
		collect("code "+id, err)
		return block
	}
	demo := func(id, title, description string) DemoBlock {
		block, err := NewDemoBlock(id, title, description)
		collect("demo "+id, err)
		return block
	}
	list := func(id string, ordered bool, items ...string) ListBlock {
		block, err := NewListBlock(id, ordered, items)
		collect("list "+id, err)
		return block
	}
	section := func(id, title, lead string, blocks ...Block) Section {
		assembled, err := NewSection(id, title, lead, blocks)
		collect("section "+id, err)
		return assembled
	}

	sections := []Section{
		// ── 1. Global by default ────────────────────────────────────
		section("global-by-default", "Global by default: UTF-8, runes, and the bytes question",
			"Go answered the encoding question before you asked it: source is UTF-8 by specification, strings carry any script, and JSON speaks UTF-8 natively. The one honesty left is byte versus rune.",
			text("global-pitch",
				"Every stack that grew up before Unicode carries an encoding decision somewhere in its config files; Go deleted the question. The language specification declares source files UTF-8, string literals hold any script without escape sequences, and the standard library's text packages — ``strings``, ``unicode``, ``unicode/utf8``, ``strconv`` — operate on Unicode text rather than on bytes that happen to be ASCII. A German greeting is not a special case in Go: ``Willkommen zurück`` is a string literal exactly as ``Welcome back`` is, and the JSON the guide serves carries Japanese month names and a euro sign with no charset ceremony. That default is why this topic's engine can be small: nothing between the catalog file and the HTTP response ever transcodes anything, so internationalization in Go is not an encoding project — it is a NEGOTIATION, CATALOG, and FORMATTING project over text that was already global."),
			text("global-runes",
				"The one honesty the default demands is the difference between a string's BYTES and its RUNES. A Go string is an immutable slice of bytes — in well-formed programs those bytes are UTF-8, but the type does not promise it, and two of the language's most-used operations answer different questions. ``len(s)`` counts bytes: ``len(\"日本語\")`` is 9, because each of the three characters encodes to three UTF-8 bytes. A ``for ... range`` over a string, by contrast, decodes ONE RUNE per iteration — the loop body sees 日本語 as three iterations, each ``r`` a ``rune`` (an alias for ``int32`` holding a Unicode code point). Byte indexing ``s[i]`` yields a byte, never a character; slicing can split a multibyte character in half if the indices came from byte arithmetic. Nothing here is a trap once it is stated: bytes are the storage, runes are the text, and every function in this topic that walks a string walks it rune-wise."),
			list("global-three-answers", false,
				"len(s) — the byte count: what memory and the wire pay, and what indexing and slicing use.",
				"utf8.RuneCountInString(s) — the rune count: what a human calls the string's length, and what truncation limits should use.",
				"for i, r := range s — the decoded walk: one iteration per rune with the byte offset ``i``, the shape every rune-wise algorithm in this topic takes.",
			),
			code("global-countrunes-go", "go",
				"The honest length in one function: bytes counted by len, runes counted by ranging — the engine's own utilitarian little reminder.",
				"server/internal/domain/i18n/pseudo.go",
				"// countRunes counts text's runes — the honest length of a string,\n"+
					"// distinct from len(text), which counts bytes (the global-by-default\n"+
					"// section's whole point in one call).\n"+
					"func countRunes(text string) int {\n"+
					"	count := 0\n"+
					"	for range text {\n"+
					"		count++\n"+
					"	}\n"+
					"	return count\n"+
					"}"),
			text("global-json",
				"The protocol edges are already global too, each with one Go-specific honesty note. ``encoding/json`` emits UTF-8 by definition — the JSON specification allows no other encoding — and it escapes ``<``, ``>``, and ``&`` to ``\\u003c`` and friends BY DEFAULT so JSON can sit inside HTML scripts; ``json.Encoder``'s ``SetEscapeHTML(false)`` turns that off when you know the consumer is not a browser. HTTP HEADERS are the one ASCII island: the wire format predates Unicode, and Go's ``net/http`` polices it — writing a header value with invalid bytes fails the write rather than corrupting the protocol, which is why this topic puts language tags (pure ASCII by BCP 47) in headers and actual translated TEXT in bodies. URLs round it out: a path segment carrying non-ASCII should be built with ``url.PathEscape`` — the browser and Go's router will round-trip it, percent-encoded, without a surprise."),
			text("global-casing",
				"One more default worth pinning because it silently does the right thing: case operations are Unicode-aware. ``strings.ToUpper`` maps through the ``unicode`` package's case tables — ``straße`` becomes ``STRASSE`` (the ß expands), and Turkish dotless-i behaves per the tables when the locale case-folding is requested through ``golang.org/x/text/cases``, which the topic compares later. The stdlib's simple ``ToUpper``/``ToLower`` apply the UNICODE-default mapping, which is correct for machine identifiers and canonically wrong for user-facing Turkish — one of the honest limits that mark where the by-hand floor ends and the x/text import begins. What the stdlib deliberately does NOT ship is COLLATION — ``sort.Strings`` sorts by code point, so ``Ä`` lands after ``Z``; sorting for humans is a locale-shaped job, and the topic's formatting section says so plainly rather than pretending a code-point sort is an alphabetical one."),
		),
		// ── 2. Accept-Language negotiation ──────────────────────────
		section("accept-language-negotiation", "Accept-Language negotiation: the header, the parser, the chain",
			"The client states its languages in one header; the server's job is to parse it forgivingly, match it against what it actually carries, and always answer — an unsupported language is a fallback, never an error.",
			text("negotiate-pitch",
				"``Accept-Language`` is the request's language ask, and its grammar is small enough to hold at once: a comma-separated list of language ranges, each optionally carrying a quality value ``q`` between 0 and 1 that ranks it. ``fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5`` — the example RFC 9110 §12.5.4 itself carries — says: Swiss French at the implicit default q=1, then French, English, German, and finally the wildcard ANYTHING, each ranked lower. Three rules do most of the work in practice: a missing q means 1, tags are CASE-INSENSITIVE (``EN-US`` and ``en-us`` are the same range by BCP 47, so the parser lowercases), and equal q-values keep the sender's order — a client that writes ``de, en`` means German first, and a stable sort preserves that. The quality values order the candidates; the negotiation then walks them most-preferred first."),
			code("negotiate-parse-go", "go",
				"The real parser: split, parse each member, keep first duplicates, stable-sort by q.",
				"server/internal/domain/i18n/negotiate.go",
				"func ParseAcceptLanguage(header string) []LanguagePreference {\n"+
					"	var prefs []LanguagePreference\n"+
					"	for position, member := range strings.Split(header, \",\") {\n"+
					"		pref, ok := parseMember(member, position)\n"+
					"		if !ok {\n"+
					"			continue\n"+
					"		}\n"+
					"		// A repeated range keeps its FIRST occurrence — the sender's\n"+
					"		// earlier word is the one the sort should honor.\n"+
					"		duplicate := false\n"+
					"		for _, existing := range prefs {\n"+
					"			if existing.Range == pref.Range {\n"+
					"				duplicate = true\n"+
					"				break\n"+
					"			}\n"+
					"		}\n"+
					"		if duplicate {\n"+
					"			continue\n"+
					"		}\n"+
					"		prefs = append(prefs, pref)\n"+
					"	}\n"+
					"	sort.SliceStable(prefs, func(i, j int) bool {\n"+
					"		return prefs[i].Quality > prefs[j].Quality\n"+
					"	})\n"+
					"	return prefs\n"+
					"}"),
			text("negotiate-drop",
				"The parser's policy toward input it cannot use is the part public endpoints actually live or die by: DROP, never guess. A member whose qvalue is malformed (``q=abc``, ``q=2``, ``q=0.1234`` — the grammar allows at most three decimals), whose range is not a BCP 47 shape (``fr--CH``, a nine-letter subtag, a space inside a range), or whose parameter is not ``q`` at all is dropped WHOLE, and the usable members still parse — the RFC's forgiveness, with the refusal attached to the member rather than to the header. Parsing never returns an error and never panics, because the header is caller-writable input and hostile input earns a shrug, not a 500: a header that says nothing usable parses to an empty list, which the negotiator turns into the default catalog. The engine's own test suite table-tests exactly these rows against the RFC's own examples — the spec's two worked headers verbatim, plus the hostile spellings a public endpoint collects."),
			code("negotiate-member-go", "go",
				"One member parsed honestly: range shape-checked, q grammar-checked, anything unusable refused whole.",
				"server/internal/domain/i18n/negotiate.go",
				"func parseMember(member string, position int) (LanguagePreference, bool) {\n"+
					"	segments := strings.Split(strings.TrimSpace(member), \";\")\n"+
					"	range_ := normalizeRange(segments[0])\n"+
					"	if range_ == \"\" {\n"+
					"		return LanguagePreference{}, false\n"+
					"	}\n"+
					"	quality := 1.0\n"+
					"	for _, parameter := range segments[1:] {\n"+
					"		name, value, found := strings.Cut(parameter, \"=\")\n"+
					"		if !found || strings.ToLower(strings.TrimSpace(name)) != \"q\" {\n"+
					"			return LanguagePreference{}, false\n"+
					"		}\n"+
					"		parsed, ok := parseQuality(strings.TrimSpace(value))\n"+
					"		if !ok {\n"+
					"			return LanguagePreference{}, false\n"+
					"		}\n"+
					"		quality = parsed\n"+
					"	}\n"+
					"	return LanguagePreference{Range: range_, Quality: quality, Order: position}, true\n"+
					"}"),
			text("negotiate-chain",
				"Negotiation is then a FALLBACK CHAIN walked most-specific first, and naming the links is most of the design. A range equal to a supported tag wins immediately; a regional variant falls to its base language (``fr-CA`` is served ``fr`` — a French page is the right answer to French Canadian, not an error); the wildcard means anything-not-refused and answers with the server's default; and when nothing matches at all, the default catalog serves and the result SAYS it did. Before any positive match runs, every ``q=0`` range EXCLUDES the tags it covers, prefix-wise — ``en;q=0`` refuses ``en-GB`` too, which is the one part of the semantics servers routinely get wrong. Every step the walk examines is recorded as a ``ChainStep`` — range, q, outcome, match — so logs and the demo below can replay exactly why a request got the catalog it got."),
			list("negotiate-links", false,
				"ViaExact — the preference's full range equals a supported tag: ``de`` offered, ``de`` carried; the strongest possible answer.",
				"ViaBase — the range's primary subtag equals a supported tag: ``fr-CA`` offered finds ``fr`` carried; the regional variant is served, not refused.",
				"ViaWildcard — the ``*`` range matched: anything the sender has not refused, answered with the default catalog.",
				"ViaDefault — nothing matched at all; the default catalog is the never-fail last link, and the result says which link answered.",
			),
			code("negotiate-walk-go", "go",
				"The walk itself: q=0 exclusions first, then exact, then base, each step recorded — the audit trail is the function's second return value.",
				"server/internal/domain/i18n/negotiate.go",
				"func Negotiate(header string, supported []string, defaultTag string) NegotiationResult {\n"+
					"	prefs := ParseAcceptLanguage(header)\n"+
					"\n"+
					"	// …q=0 ranges exclude the tags they cover before any match runs…\n"+
					"	excluded := make(map[string]bool)\n"+
					"\n"+
					"	result := NegotiationResult{Selected: defaultTag, Via: ViaDefault}\n"+
					"	record := func(pref LanguagePreference, outcome, matched string) bool {\n"+
					"		result.Chain = append(result.Chain, ChainStep{\n"+
					"			Range: pref.Range, Quality: pref.Quality, Outcome: outcome, Matched: matched,\n"+
					"		})\n"+
					"		return outcome != OutcomeNone && outcome != OutcomeExcluded\n"+
					"	}\n"+
					"\n"+
					"	for _, pref := range prefs {\n"+
					"		if pref.Quality == 0 {\n"+
					"			record(pref, OutcomeExcluded, \"\")\n"+
					"			continue\n"+
					"		}\n"+
					"		if pref.IsWildcard() {\n"+
					"			// …the wildcard serves the default catalog, unless the sender refused it…\n"+
					"		}\n"+
					"		for _, tag := range supported {\n"+
					"			if excluded[tag] {\n"+
					"				continue\n"+
					"			}\n"+
					"			if strings.EqualFold(pref.Range, tag) {\n"+
					"				if record(pref, ViaExact, tag) {\n"+
					"					result.Selected, result.Via = tag, ViaExact\n"+
					"					return result\n"+
					"				}\n"+
					"			}\n"+
					"		}\n"+
					"		for _, tag := range supported {\n"+
					"			if excluded[tag] {\n"+
					"				continue\n"+
					"			}\n"+
					"			if pref.Base() == baseOf(tag) {\n"+
					"				if record(pref, ViaBase, tag) {\n"+
					"					result.Selected, result.Via = tag, ViaBase\n"+
					"					return result\n"+
					"				}\n"+
					"			}\n"+
					"		}\n"+
					"		record(pref, OutcomeNone, \"\")\n"+
					"	}\n"+
					"	return result\n"+
					"}"),
			text("negotiate-handler",
				"The HTTP edge around all of this is one honest line: read the request's OWN header, verbatim, and hand it down — a negotiator demo that negotiated with anything other than what actually arrived on the wire would be teaching the wrong thing. The handler below is the whole topic's presentation layer; everything else in this topic is domain and data. When the request carries no Accept-Language at all (curl without ``-H``, a health check), the source flips to the RFC fixture and the story says which ran — the fixture is the spec's own five-member example, so even the header-less case walks a real, citable header rather than a staged one."),
			code("negotiate-handler-go", "go",
				"The whole presentation layer: the request's own header, the runner port, the story payload — nothing else.",
				"server/internal/presentation/demo_i18n.go",
				"func (h *I18nNegotiatorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	// The request's OWN header, verbatim — the one thing this demo must\n"+
					"	// never do is negotiate with anything other than what actually came\n"+
					"	// in on the wire.\n"+
					"	header := r.Header.Get(\"Accept-Language\")\n"+
					"	source := \"request\"\n"+
					"	if header == \"\" {\n"+
					"		source = \"fixture\" // the console substitutes the RFC 9110 fixture\n"+
					"	}\n"+
					"	run, err := h.runner.RunNegotiatorStory(r.Context(), header, source)\n"+
					"	if err != nil {\n"+
					"		core.WriteError(w, r, core.Wrap(err, core.KindInternal,\n"+
					"			\"The i18n negotiator demo could not run its story against the catalogs.\"), h.logger)\n"+
					"		return\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusOK, demos.BuildI18nNegotiatorPayload(run))\n"+
					"}"),
			demo("i18n:negotiator", "The i18n negotiator: your header, walked to a served message",
				"One request runs the whole topic against the REAL embedded catalogs, and every beat derives from the Accept-Language header THIS request actually sent. The header, parsed: the candidates with their q-values, ordered, the unusable members dropped — your own header's rows, not a fixture's. The match: the fallback chain walked link by link — exact, base, wildcard, default — with the step that selected the catalog named. One message, every catalog: the same key interpolated per locale, plus the composed sentence that puts a localized date and clock inside a translated sentence. Plurals at 0, 1, and 2 under three different rules — English singularizes exactly 1, French 0 AND 1, Japanese never. The format matrices: 1234567.89 and an 1899-minor-unit price through four locales' separators, symbols, months, and clocks. Timezone is not locale: one fixed instant read in Berlin, New York, and Tokyo — deliberately placed between the EU and US DST changeovers so CET and EDT coexist — plus the unknown-zone error, verbatim. And the honest edges: an unsupported language walking to the default, and a missing key served as itself with its log line shown."),
		),
		// ── 3. Message catalogs ─────────────────────────────────────
		section("message-catalogs", "Message catalogs: Go source as the catalog",
			"Translated text is data, and Go already has two excellent stores for it: the map for a small service, and embed.FS for checked-in files compiled into the binary. The port keeps them interchangeable; the policy around keys is where the production shape lives.",
			text("catalogs-pitch",
				"A message catalog maps KEYS to per-locale text, and the whole design question is where the port sits. This guide's answer is the family's usual one: a small interface in the domain, everything else behind it. ``Catalog`` answers one lookup (does this tag carry this key) and lists the tags it carries — no formatting policy, no fallback chain, nothing but translated text, which is why an in-memory map and an ``embed.FS``-backed loader are INTERCHANGEABLE behind it. Handlers never see the catalog at all; they call the ``Translator`` (below), which owns the ladder — resolve the locale, find the key there or in the default catalog, pick the plural form, interpolate. The split is not ceremony: it is what lets the guide quote a map-backed catalog for a three-locale service and ship an embedded-files one for the real demo without touching a line of the negotiation or translation logic."),
			code("catalogs-port-go", "go",
				"The port: look up one key under one tag, list the carried tags — translated text and nothing else.",
				"server/internal/domain/i18n/catalog.go",
				"type Catalog interface {\n"+
					"	// Message answers the key's entry for the tag, or false when the\n"+
					"	// tag or the key is not carried.\n"+
					"	Message(tag, key string) (Message, bool)\n"+
					"	// Tags lists the carried tags in preference order — the negotiator's\n"+
					"	// supported list, default first.\n"+
					"	Tags() []string\n"+
					"}"),
			text("catalogs-map",
				"The map implementation is the whole adapter for a small service — Go source IS the catalog store, which sounds like a toy until you notice what it buys: compiler-checked keys at the call site's unit tests, diffable pull requests, zero load path. The constructor VALIDATES rather than trusting: tags must be listed once each, messages for unlisted tags are refused, every message must carry a non-empty ``other`` form (the universal plural fallback the translator assumes exists), and a bad wiring fails at construction — at boot, with the file named — never at request time. The lookup itself is two map reads, and the case-normalization on the tag is the caller's-case tolerance the family's wire contract expects:"),
			code("catalogs-map-go", "go",
				"Go maps as the catalog store: two reads per lookup, validated once at construction.",
				"server/internal/domain/i18n/catalog.go",
				"// Message implements Catalog.\n"+
					"func (c *mapCatalog) Message(tag, key string) (Message, bool) {\n"+
					"	entries, ok := c.messages[strings.ToLower(tag)]\n"+
					"	if !ok {\n"+
					"		return Message{}, false\n"+
					"	}\n"+
					"	message, ok := entries[key]\n"+
					"	return message, ok\n"+
					"}"),
			text("catalogs-json",
				"Translators, however, do not hand back Go source — they hand back FILES, and JSON is the least-ceremony shape a translation pipeline round-trips. The data layer embeds one JSON file per tag with ``embed.FS``, which compiles the files INTO THE BINARY: the guide's i18n story rides in the same single static artifact as everything else, deployable to a scratch container with no locale directory beside it. The loader is deliberately strict where the header parser was forgiving, because these files are OURS, not a stranger's: a stray locale file outside the preferred set, an unknown plural category, a key with no ``other`` form — each fails startup naming the file, so a half-merged translation PR is a deploy-time error rather than a runtime surprise. The strict/forgiving split is the topic's sharpest policy line: HOSTILE input gets tolerance, OUR input gets refusal."),
			code("catalogs-embed-go", "go",
				"The data layer's embedded catalogs: one JSON file per tag, compiled in, validated at boot.",
				"server/internal/data/i18n/console.go",
				"import (\n"+
					"	// …\n"+
					"	\"embed\" // the locale files below compile into the binary\n"+
					"	// …\n"+
					")\n"+
					"\n"+
					"//go:embed locales/*.json\n"+
					"var localeFiles embed.FS\n"+
					"\n"+
					"// preferredTags is the catalog's preference order — the negotiator's\n"+
					"// supported list, default first. The loader requires the embedded file\n"+
					"// set to match it exactly, so a stray or missing locale fails startup\n"+
					"// rather than a request.\n"+
					"var preferredTags = []string{\"en\", \"de\", \"fr\", \"ja\"}"),
			code("catalogs-de-json", "json",
				"One real catalog file: keys, plural forms as a map, {placeholders} in the text.",
				"server/internal/data/i18n/locales/de.json",
				"{\n"+
					"  \"demo.greeting\": {\n"+
					"    \"other\": \"Willkommen zurück, {name}!\"\n"+
					"  },\n"+
					"  \"demo.new_posts\": {\n"+
					"    \"one\": \"Du hast {count} neuen Beitrag in deinem Feed.\",\n"+
					"    \"other\": \"Du hast {count} neue Beiträge in deinem Feed.\"\n"+
					"  },\n"+
					"  \"demo.published_at\": {\n"+
					"    \"other\": \"Veröffentlicht am {date} um {time} ({zone}).\"\n"+
					"  }\n"+
					"}"),
			text("catalogs-plurals",
				"PLURAL is where naive catalogs fail, because plural is a per-locale RULE, not a count. CLDR defines six cardinal categories — zero, one, two, few, many, other — and every language picks its own subset: English and German use ``one`` for exactly 1 and ``other`` for everything else including 0; French treats 0 AND 1 as ``one`` (``0 nouvelle publication`` is correct French); Japanese makes no distinction at all, one form for every count. The guide implements the rules its four demo locales need, by hand, as functions — thirty honest lines that make the mechanism visible — and says plainly what that buys and what it costs: it buys a reader who can hold the whole engine in their head, and it costs you the long tail (Arabic's six categories, Polish's few-vs-many), which is exactly the job ``golang.org/x/text/message`` does with generated CLDR tables. The rule is selected at translation time, from the count, by the LOCALE's own function:"),
			code("catalogs-plural-go", "go",
				"Two of the three rules the demo locales need: French's zero-and-one singular is the one that surprises.",
				"server/internal/domain/i18n/catalog.go",
				"// pluralOneForOne is the English/German rule: one thing is singular,\n"+
					"// everything else — including zero — is plural.\n"+
					"func pluralOneForOne(count int) PluralCategory {\n"+
					"	if count == 1 {\n"+
					"		return PluralOne\n"+
					"	}\n"+
					"	return PluralOther\n"+
					"}\n"+
					"\n"+
					"// pluralOneForZeroAndOne is the French rule: zero AND one are singular\n"+
					"// (\"0 nouvelle publication\" is correct French).\n"+
					"func pluralOneForZeroAndOne(count int) PluralCategory {\n"+
					"	if count == 0 || count == 1 {\n"+
					"		return PluralOne\n"+
					"	}\n"+
					"	return PluralOther\n"+
					"}"),
			text("catalogs-translate",
				"The ``Translator`` ties catalog, locale profiles, and policy into the one call handlers make, and its ladder is worth reading top to bottom because every rung is a production decision. Resolve the LOCALE — exact tag, base language, default — so ``fr-CA`` serves the ``fr`` profile without ceremony. Find the KEY in that locale's catalog; if the locale exists but the key is untranslated, fall back to the DEFAULT catalog before giving up (a half-translated locale serves English for its missing keys, flagged, rather than an error). Select the plural form the locale's rule picks for the count, falling to ``other`` when the exact category is absent. And if NO catalog carries the key: serve the KEY ITSELF as the text, mark it not-found, and say so to a callback that production wires to a log line — a user seeing an untranslated key is a bug worth a log line, not a 500 worth an outage. The demo below shows that policy's notice verbatim."),
			code("catalogs-translate-go", "go",
				"The ladder: locale resolution, key fallback, plural selection, interpolation — every rung visible in the returned Translation.",
				"server/internal/domain/i18n/catalog.go",
				"func (t *Translator) Translate(tag, key string, count int, vars map[string]string) Translation {\n"+
					"	locale, resolved := t.localeFor(tag)\n"+
					"	fellBack := false\n"+
					"	if !resolved {\n"+
					"		locale = t.locales[t.defaultTag]\n"+
					"		fellBack = true\n"+
					"	}\n"+
					"	served := locale.Tag\n"+
					"\n"+
					"	message, found := t.catalog.Message(served, key)\n"+
					"	if !found {\n"+
					"		message, found = t.catalog.Message(t.defaultTag, key)\n"+
					"		served = t.defaultTag\n"+
					"		fellBack = true\n"+
					"	}\n"+
					"	if !found {\n"+
					"		if t.onMissing != nil {\n"+
					"			t.onMissing(strings.ToLower(strings.TrimSpace(tag)), key)\n"+
					"		}\n"+
					"		return Translation{Text: key, Served: served, Fallback: true, Found: false}\n"+
					"	}\n"+
					"\n"+
					"	form, ok := message.Forms[locale.Plural(count)]\n"+
					"	if !ok || form == \"\" {\n"+
					"		form = message.Forms[PluralOther]\n"+
					"	}\n"+
					"	return Translation{\n"+
					"		Text:     Interpolate(form, vars),\n"+
					"		Served:   served,\n"+
					"		Fallback: fellBack || served != locale.Tag,\n"+
					"		Found:    true,\n"+
					"	}\n"+
					"}"),
			text("catalogs-interpolate",
				"Interpolation is the last step and gets the same honesty treatment: ``{name}`` placeholders are substituted RUNE-WISE (the loop decodes, never byte-indexes — slicing a multibyte placeholder name in half is exactly the bug the walk prevents), placeholder names trim their whitespace, repeated placeholders substitute fine, and an UNKNOWN placeholder is left verbatim on purpose — ``{visible}``` on screen points at the code that forgot the variable, which beats silently dropping it and shipping a sentence with a hole. Nested braces are not placeholders, an unclosed brace is literal text, and ``nil`` variables leave the string untouched. The whole function is twenty lines, and it is the section's last code because it is the topic's smallest complete lesson: global by default means the string machinery itself must walk runes, even here in the last mile before the wire."),
			code("catalogs-interpolate-go", "go",
				"Placeholders, rune-wise: decode, match the whole brace pair, substitute or keep — never a byte-split surprise.",
				"server/internal/domain/i18n/catalog.go",
				"func Interpolate(text string, vars map[string]string) string {\n"+
					"	if len(vars) == 0 || !strings.Contains(text, \"{\") {\n"+
					"		return text\n"+
					"	}\n"+
					"	var out strings.Builder\n"+
					"	out.Grow(len(text))\n"+
					"	runes := []rune(text)\n"+
					"	for i := 0; i < len(runes); i++ {\n"+
					"		if runes[i] != '{' {\n"+
					"			out.WriteRune(runes[i])\n"+
					"			continue\n"+
					"		}\n"+
					"		// …find the closing brace, trim the name, substitute or keep…\n"+
					"	}\n"+
					"	return out.String()\n"+
					"}"),
		),
		// ── 4. Formatting ───────────────────────────────────────────
		section("formatting-numbers-dates", "Formatting: numbers, money, dates, clocks",
			"fmt never met a locale: %d and %.2f are locale-neutral by design. Locale-aware presentation is conventions as data — separators, symbols, month names, clock shape — on a profile the renderers read.",
			text("format-pitch",
				"The ``fmt`` package's verbs are deliberately LOCALE-NEUTRAL: ``%d`` never inserts a group separator and ``%.2f`` always uses a point, wherever the process runs — a number formatted in Berlin and Bangkok renders identically, which is exactly what a machine-readable API wants and exactly what a human-facing page does not. The honest floor for the second case is conventions AS DATA: one ``Locale`` profile per supported language carrying its separators, currency shape, month names, date shape, clock convention, and plural rule — plain values, comparable and copyable, handed to renderers that are pure functions of (profile, value). Nothing in the stdlib ships these profiles (``golang.org/x/text`` does, from CLDR), so a by-hand floor states its four axes and stops: NUMBER rendering (grouping and decimal separators), MONEY (symbol, placement, minor-unit digits), DATES (shape and month names), and CLOCKS (12- versus 24-hour)."),
			code("format-locale-go", "go",
				"The profile: every presentation convention the floor renders, stated as data on one struct.",
				"server/internal/domain/i18n/format.go",
				"type Locale struct {\n"+
					"	Tag      string // BCP 47 base tag: \"en\", \"de\", \"fr\", \"ja\"\n"+
					"	Language string // English name for prose: \"English\", \"German\"…\n"+
					"	// Number shapes: the separators between digit groups and around the\n"+
					"	// fraction. English writes 1,234.56; German 1.234,56; French groups\n"+
					"	// with a narrow no-break space (U+202F) and decimals with a comma.\n"+
					"	GroupSeparator   string\n"+
					"	DecimalSeparator string\n"+
					"	// Currency shape: the ISO code (what APIs store and exchange), the\n"+
					"	// symbol (what humans read), whether the symbol trails the amount\n"+
					"	// (\"18,99 €\"), and how many minor-unit digits the currency carries\n"+
					"	// (2 for dollars and euros, 0 for yen).\n"+
					"	CurrencyCode     string\n"+
					"	CurrencySymbol   string\n"+
					"	CurrencySuffix   bool\n"+
					"	CurrencyDecimals int\n"+
					"	// Date and clock shapes.\n"+
					"	Months    [12]string\n"+
					"	DateShape DateShape\n"+
					"	Clock12   bool // 12-hour clock with AM/PM (\"2:30 PM\") vs 24-hour (\"14:30\")\n"+
					"	AmPm      [2]string\n"+
					"	// Plural selects the locale's CLDR cardinal category for a count.\n"+
					"	Plural func(count int) PluralCategory\n"+
					"}"),
			text("format-profiles",
				"The four demo locales are chosen to cover every axis with a difference, not to be a locale census: English groups with commas and splits decimals with a point (``1,234,567.89``); German swaps BOTH (``1.234.567,89``); French groups with U+202F NARROW NO-BREAK SPACE — the CLDR-correct separator that renders like a space but never lets a number wrap across lines — and writes ``18,99 €`` with the symbol trailing; Japanese groups like English but writes dates as ``2026年10月27日`` with the multibyte literals straight into the profile, and its yen carries ZERO minor units. The month arrays carry localized names verbatim — ``Januar``, ``octobre``, ``10月`` — because a date shape without its month names is half a convention. Four profiles, every axis different somewhere, and the demo renders the same value through all four so the differences are rows in one table rather than claims in prose:"),
			code("format-profiles-go", "go",
				"Two of the four profiles: German's swapped separators and suffix euro; French's narrow no-break space and zero-singular rule.",
				"server/internal/domain/i18n/format.go",
				"{\n"+
					"	Tag: \"de\", Language: \"German\",\n"+
					"	GroupSeparator: \".\", DecimalSeparator: \",\",\n"+
					"	CurrencyCode: \"EUR\", CurrencySymbol: \"€\", CurrencySuffix: true, CurrencyDecimals: 2,\n"+
					"	Months:    [12]string{\"Januar\", \"Februar\", \"März\", \"April\", \"Mai\", \"Juni\", \"Juli\", \"August\", \"September\", \"Oktober\", \"November\", \"Dezember\"},\n"+
					"	DateShape: DateDayMonthYear,\n"+
					"	Plural:    pluralOneForOne,\n"+
					"},\n"+
					"{\n"+
					"	Tag: \"fr\", Language: \"French\",\n"+
					"	// U+202F NARROW NO-BREAK SPACE — the CLDR French group separator:\n"+
					"	// \"1 234,56\", a space the renderer never lets wrap.\n"+
					"	GroupSeparator: \"\\u202f\", DecimalSeparator: \",\",\n"+
					"	CurrencyCode: \"EUR\", CurrencySymbol: \"€\", CurrencySuffix: true, CurrencyDecimals: 2,\n"+
					"	Months:    [12]string{\"janvier\", \"février\", \"mars\", \"avril\", \"mai\", \"juin\", \"juillet\", \"août\", \"septembre\", \"octobre\", \"novembre\", \"décembre\"},\n"+
					"	DateShape: DateDayMonthNameYear,\n"+
					"	Plural:    pluralOneForZeroAndOne,\n"+
					"},"),
			text("format-money",
				"MONEY gets its own renderer because the honest money rule predates every locale: amounts are INTEGER MINOR UNITS — cents, sen — from the database column to the screen, with no float ever standing between the stored value and the rendered digits. The float path (store 18.99, format with %.2f) accumulates representation error that rounding at the render cannot reliably undo, and the integer path simply does not have the problem: ``FormatCurrency`` divides the minor units out with integer arithmetic, pads the remainder to the currency's fixed width, groups the digits with the locale's separator, and places the symbol per the profile — prefix for dollars, suffix with a space for euros, and none of the decimal ceremony at all for yen, whose minor-unit count is zero and whose minor unit IS the unit. The negative sign rides the amount, not the symbol; the demo's 1899 minor units render ``$18.99``, ``18,99 €``, and ``¥1,899`` from the same integer:"),
			code("format-money-go", "go",
				"Integer minor units end to end: divide, pad, group, place the symbol — no float on the path.",
				"server/internal/domain/i18n/format.go",
				"func FormatCurrency(l Locale, minor int64) string {\n"+
					"	scale := int64(1)\n"+
					"	for i := 0; i < l.CurrencyDecimals; i++ {\n"+
					"		scale *= 10\n"+
					"	}\n"+
					"	units, remainder := minor/scale, minor%scale\n"+
					"	negative := units < 0\n"+
					"	digits := strconv.FormatInt(absInt64(units), 10)\n"+
					"	var out strings.Builder\n"+
					"	if negative {\n"+
					"		out.WriteByte('-')\n"+
					"	}\n"+
					"	// …suffix locales write \"18,99 €\", prefix locales \"$18.99\"…\n"+
					"	if l.CurrencySuffix {\n"+
					"		out.WriteString(groupDigits(l, digits))\n"+
					"		if l.CurrencyDecimals > 0 {\n"+
					"			out.WriteString(l.DecimalSeparator)\n"+
					"			out.WriteString(zeroPadded(int(remainder), l.CurrencyDecimals))\n"+
					"		}\n"+
					"		out.WriteString(\" \")\n"+
					"		out.WriteString(l.CurrencySymbol)\n"+
					"		return out.String()\n"+
					"	}\n"+
					"	// …the prefix branch mirrors it: symbol, digits, decimals…\n"+
					"}"),
			text("format-dates",
				"Dates and clocks close the floor, and the teaching move is the same one the range-request topic used: the convention is a named SHAPE, and one switch turns shapes into text. ``Oct 27, 2026`` for English; ``27.10.2026`` for German's leading-zero dots; ``27 octobre 2026`` for French's spelled-out month; ``2026年10月27日`` for Japanese — the multibyte literals sit in the renderer next to the ASCII shapes, UTF-8 by default made ordinary. Clocks split on the profile's ``Clock12`` flag: twelve-hour with AM/PM (and the two honest edge cases — midnight is ``12:07 AM``, noon is ``12:00 PM``) or twenty-four-hour ``14:05``. When the floor stops being enough — full CLDR number patterns, non-Gregorian calendars, transliteration — that is the moment ``golang.org/x/text/number`` and friends earn their import, and the guide's stance is to say so at that line rather than to grow the floor one convention at a time into a half-correct library:"),
			code("format-date-go", "go",
				"Shapes into text: the US form, the Japanese form with its multibyte literals, the others between.",
				"server/internal/domain/i18n/format.go",
				"func FormatDate(l Locale, t time.Time) string {\n"+
					"	month := l.Months[int(t.Month())-1]\n"+
					"	switch l.DateShape {\n"+
					"	case DateMonthDayYear:\n"+
					"		return month + \" \" + strconv.Itoa(t.Day()) + \", \" + strconv.Itoa(t.Year())\n"+
					"	// …day-month-year dots, day month-name year…\n"+
					"	default: // DateYearMonthDay\n"+
					"		return strconv.Itoa(t.Year()) + \"年\" + strconv.Itoa(int(t.Month())) + \"月\" + strconv.Itoa(t.Day()) + \"日\"\n"+
					"	}\n"+
					"}"),
		),
		// ── 5. Timezones vs locales ─────────────────────────────────
		section("timezones-vs-locales", "Timezones are not locales",
			"A timezone chooses the clock reading; a locale chooses everything else. Go keeps them separate, and conflating them is the classic i18n bug — time.LoadLocation is the zone axis, the Locale profile is the language axis, and a rendering needs both.",
			text("zones-pitch",
				"The two axes answer different questions and Go gives each its own machinery. A TIMEZONE (``Europe/Berlin``, ``America/New_York``, ``Asia/Tokyo``) is a rule about offsets and daylight-saving transitions — it determines what a wall clock READS at one instant. A LOCALE (``de``, ``fr``, ``ja``) is a bundle of presentation conventions — it determines the words, the separators, the clock's SHAPE (12- or 24-hour), never the reading. A German reader in New York wants German words around a New York clock; a Japanese reader in Berlin wants Japanese shapes around a Berlin clock. The implementation split follows the concept split: ``time.LoadLocation`` loads IANA zones for the first axis, the ``Locale`` profile drives the second, and a full rendering composes them — convert the instant into the zone, then format the result with the locale. The demo makes the orthogonality VISIBLE: one instant, three zones, two locales, six cells, and only the zone changes the hour."),
			text("zones-tzdata",
				"``time.LoadLocation`` needs a zone database, and honesty about WHERE it comes from prevents a container classic. Go looks in order: the ``$ZONEINFO`` environment variable's directory, then the system's ``/usr/share/zoneinfo``, then — if the binary blank-imports ``time/tzdata`` — an IANA database EMBEDDED in the binary itself. That third source is the one a scratch or distroless image needs: those images ship no zoneinfo directory, and without the import every ``LoadLocation`` call fails at runtime. The guide's data layer takes the import — one line, ~450 KiB, and the zone axis becomes as deploy-proof as the embedded catalogs made the language axis. Two honest notes travel with it: the embedded snapshot is FROZEN at build time (a zone rule changed by legislation after your build needs a rebuild — the same discipline as any pinned dependency), and an unknown zone name answers a plain error — never a fabricated offset, never a panic:"),
			code("zones-tzdata-go", "go",
				"The zone axis made deploy-proof: tzdata embedded beside the catalogs, and the demo's deliberately DST-straddling instant.",
				"server/internal/data/i18n/console.go",
				"import (\n"+
					"	// …\n"+
					"	_ \"time/tzdata\" // embedded zone database: LoadLocation's last resort\n"+
					")\n"+
					"\n"+
					"// storyInstant is the fixed instant every rendering in the demo\n"+
					"// derives from — deterministic request over request, and deliberately\n"+
					"// placed two days after the EU DST changeover and five before the US\n"+
					"// one, so Berlin and New York render under DIFFERENT DST rules (CET\n"+
					"// vs EDT) from one moment.\n"+
					"var storyInstant = time.Date(2026, time.October, 27, 12, 30, 0, 0, time.UTC)\n"+
					"\n"+
					"// storyZones are the IANA zones the timezone beat renders.\n"+
					"var storyZones = []string{\"Europe/Berlin\", \"America/New_York\", \"Asia/Tokyo\"}"),
			text("zones-one-instant",
				"The demo instant is not chosen at random: 2026-10-27T12:30:00Z sits TWO DAYS after the European Union's DST exit (October 25) and FIVE BEFORE the United States' (November 1) — the one week a year when Berlin is already on standard time and New York is still on daylight, so the three zones answer CET (+01:00), EDT (−04:00), and JST (+09:00, no DST at all) from a single moment. Each zone row reports the abbreviation, the numeric offset, and the clock in both conventions — and the abbreviations carry their own honesty: they are what the zone database says (``CET``, ``EDT``), not a mapping any code of ours performs. Go's reference-time layout makes the two offset renderings one-liners once you know the trick — ``Format(\"-07:00\")`` prints the numeric offset with sign, and the abbreviation comes from the zone name in the layout string — and the demo pins all three clocks so the wire can be checked against an almanac, not against vibes:"),
			code("zones-render-go", "go",
				"One instant, three zones, two locales: load, convert, format — and the unknown zone's verbatim error.",
				"server/internal/data/i18n/console.go",
				"for _, name := range storyZones {\n"+
					"	location, err := time.LoadLocation(name)\n"+
					"	if err != nil {\n"+
					"		return demos.I18nNegotiatorRun{}, fmt.Errorf(\"load zone %s: %w\", name, err)\n"+
					"	}\n"+
					"	local := storyInstant.In(location)\n"+
					"	zones = append(zones, demos.I18nZoneRow{\n"+
					"		Zone:         name,\n"+
					"		Abbreviation: local.Format(\"MST\"),\n"+
					"		Offset:       local.Format(\"-07:00\"),\n"+
					"		ClockEN:      domi18n.FormatClock(english, local),\n"+
					"		ClockDE:      domi18n.FormatClock(german, local),\n"+
					"	})\n"+
					"}\n"+
					"_, honestyErr := time.LoadLocation(honestyZone)"),
			text("zones-compose",
				"Composition is where the two axes meet a real feature: the ``demo.published_at`` key interpolates a localized DATE, a localized CLOCK, and the zone NAME into one translated sentence — ``Veröffentlicht am 27.10.2026 um 13:30 (Europe/Berlin)`` — German words, German date shape, Berlin reading. The storage rule that makes this safe is older than Go: PERSIST instants in UTC (or as Unix timestamps), convert at render time, and never store a wall-clock reading without its zone — a timestamp column that has silently baked in one server's zone is a data bug no formatter can undo. The one thing the composed sentence deliberately does NOT do is derive the zone from the locale: it takes the zone as its own input, because German does not imply Berlin (Vienna and Zurich speak it too), and Japanese does not imply Tokyo. Language from the negotiator, zone from the data or the user's explicit choice, words and shapes from the catalog — three inputs, one sentence, zero guesswork."),
		),
		// ── 6. Translation workflow ─────────────────────────────────
		section("translation-workflow", "The translation workflow: extract, translate, review, ship",
			"Code is written once; catalogs are written, rewritten, and reviewed forever. The workflow that keeps them honest has four stages and one pre-flight test that catches what none of the stages can.",
			text("workflow-pitch",
				"An internationalized codebase settles into a rhythm that is a PROCESS, not a feature: developers mint keys and ship them in the default catalog; translators fill the other catalogs; native speakers review; the build embeds and deploys. Each stage has a failure mode the guide's own machinery is aimed at. EXTRACTION fails by hardcoded strings — text glued together in code that no catalog will ever see; TRANSLATION fails by stale or missing keys — a catalog that lags the code's keys; REVIEW fails by machine-translated confidence — fluent-looking text a native speaker would rewrite; SHIPPING fails by drift — the running binary carrying older catalogs than the repository. The mitigations are respectively: the catalog-or-nothing rule (every user-facing string routes through the translator), boot-time validation (the loader refuses a mismatched file set at startup, so lag is a deploy error), human review as a required stage, and embedding (the binary IS the deployment unit — catalogs cannot drift from code because they compile together)."),
			list("workflow-stages", true,
				"Extract — mint the key, ship it in the default catalog with the placeholder set the sentence needs; the key is a CONTRACT, stable across refactors.",
				"Translate — hand the catalog files to translators (or a machine first pass); JSON round-trips through every translation toolchain without ceremony.",
				"Review — a native speaker reads every string in place; the review stage is where tone, formality level, and plural honesty are actually earned.",
				"Ship — embed, build, deploy: one artifact carrying code and catalogs together, with boot validation refusing a broken set before traffic arrives.",
			),
			text("workflow-pseudo",
				"The pre-flight test is PSEUDO-LOCALE testing, and it is the one stage that needs no translator at all. The idea: mechanically transform the default catalog into a visibly WRONG-but-structured pseudo language, and exercise the whole UI in it before anyone is hired. The guide's ``PseudoLocalize`` brackets every string (so pseudo text is unmistakable on screen), accents the ASCII vowels (so a pipeline that mangles UTF-8 mangles these visibly), and pads every string 30% longer — the growth localization teams quote for real translations out of English — so every fixed-width box, truncation, and overflow breaks NOW, in the pseudo build, instead of later, in production German. Two defect classes surface that nothing else catches early: HARDCODED strings (a pseudo-ized screen still shows plain English wherever code glued text together — the extraction stage's failure, made visible) and LAYOUT assumptions (the padding stress test). CLDR formalizes this as the ``en-XA`` pseudo-locale; the by-hand version is small enough to quote whole:"),
			code("workflow-pseudo-go", "go",
				"The pre-flight transform: bracket, accent, pad — placeholders preserved so the pseudo string still interpolates.",
				"server/internal/domain/i18n/pseudo.go",
				"// PseudoLocalize transforms message text for pseudo-locale testing:\n"+
					"// bracketed so it is unmistakably not a real translation, accented so\n"+
					"// encoding bugs surface, and padded with '×' so length-sensitive layout\n"+
					"// breaks before a single translator is hired. Placeholders ({name}) are\n"+
					"// preserved verbatim — the pseudo string must still interpolate, or the\n"+
					"// test itself would be lying.\n"+
					"func PseudoLocalize(text string) string {\n"+
					"	var out strings.Builder\n"+
					"	out.WriteString(\"[\")\n"+
					"	out.WriteString(pseudoBody(text))\n"+
					"	padding := int(float64(countRunes(text))*pseudoPadFraction) + 1\n"+
					"	for i := 0; i < padding; i++ {\n"+
					"		out.WriteRune('×')\n"+
					"	}\n"+
					"	out.WriteString(\"]\")\n"+
					"	return out.String()\n"+
					"}"),
			text("workflow-ship",
				"Two closing policies keep the workflow honest at the edges. First, MISSING KEYS ARE VISIBLE BY DESIGN: the translator serves the key itself and logs the notice, so an untranslated string ships as its own key on screen — ugly, findable, and never an outage — and the log line is the extraction queue for the next translation round. A service that silently fell back to English for missing keys would look fine while its German catalog quietly rotted; the visible-key policy trades a bruise for a signal. Second, the loader is the SEAM: everything between the embed.FS JSON this guide ships and a Gettext ``.po`` pipeline or an XLIFF export from a translation platform is one adapter behind the ``Catalog`` port — swap the loader, keep the negotiator, the translator, the formatters, and every handler untouched. The workflow outgrows the by-hand floor long before the engine does, and the port boundary is drawn exactly where that growth will happen."),
		),
		// ── 7. Terminal companion ───────────────────────────────────
		section("terminal-companion", "Terminal companion: the i18n curl tour",
			"One endpoint, many Accept-Language headers: exact matches, the RFC's own example, q=0 refusals, unsupported languages, and the header-less fixture — every beat off the wire on :8100.",
			text("term-i18n-pitch",
				"With the stack up (server on :8100), this tour exercises the whole negotiation with curl's ``-H`` — each request's own header walked to its served message, exactly as the inline demo renders it. The first block runs the fixture case (no header sent — the console substitutes the RFC's example and the story says so) and a plain exact match; the second walks the interesting headers: the spec's five-member example (Swiss French resolving to the French catalog via BASE), a refused language (``de;q=0`` excluding German before the wildcard), an unsupported language (Korean walking its whole chain to the default), and a malformed member (dropped, not guessed for). The python3 one-liners pull single fields out of the story so the tour stays readable:"),
			code("term-negotiate-bash", "bash", "", "",
				"# no header sent: the story runs the RFC 9110 fixture and SAYS so\n"+
					"curl -s localhost:8100/api/demo/i18n/negotiator | python3 -m json.tool | head -20\n"+
					"\n"+
					"# an exact match: German selected via the chain's first link\n"+
					"curl -s -H 'Accept-Language: de' localhost:8100/api/demo/i18n/negotiator | \\\n"+
					"  python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"beats\"][1][\"detail\"])'"),
			code("term-chain-bash", "bash", "", "",
				"# the spec's own example: fr-CH has no catalog, base fr serves\n"+
					"curl -s -H 'Accept-Language: fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5' \\\n"+
					"  localhost:8100/api/demo/i18n/negotiator | \\\n"+
					"  python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"proofs\"][0])'\n"+
					"\n"+
					"# q=0 refuses German outright: the wildcard must not resurrect it\n"+
					"curl -s -H 'Accept-Language: de;q=0, *;q=0.5' localhost:8100/api/demo/i18n/negotiator | \\\n"+
					"  python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"beats\"][0][\"facts\"])'\n"+
					"\n"+
					"# unsupported language: the honest walk to the default catalog\n"+
					"curl -s -H 'Accept-Language: ko-KR,ko;q=0.9' localhost:8100/api/demo/i18n/negotiator | \\\n"+
					"  python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"beats\"][6][\"detail\"])'\n"+
					"\n"+
					"# a malformed member is dropped whole; the usable one still parses\n"+
					"curl -s -H 'Accept-Language: fr;q=nonsense, de' localhost:8100/api/demo/i18n/negotiator | \\\n"+
					"  python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"beats\"][0][\"detail\"])'"),
			code("term-content-bash", "bash", "", "",
				"# the plural and format beats, straight off the wire\n"+
					"curl -s -H 'Accept-Language: ja' localhost:8100/api/demo/i18n/negotiator | \\\n"+
					"  python3 -c 'import json,sys; d=json.load(sys.stdin); \\\n"+
					"print([f[\"value\"] for f in d[\"beats\"][4][\"facts\"]])'\n"+
					"\n"+
					"# the topic on the wire — order 27 in the media-realtime group\n"+
					"curl -s localhost:8100/api/content | python3 -c 'import json,sys; d=json.load(sys.stdin); \\\n"+
					"print([t[\"id\"] for t in d[\"topics\"] if t[\"group\"]==\"media-realtime\"])'"),
			text("term-i18n-coda",
				"Read the tour as the topic's proofs, re-runnable with any header you can type: the chain's selection named in every response (exact for ``de``, base for ``fr-CH``, default for Korean, the wildcard unable to resurrect a refused ``de``), the drop-not-guess parser (a nonsense qvalue removing its member while German still parses), the plural rules differing per locale at the same counts, the format matrices separating four locales' conventions over one value, and the timezone beat holding six clock readings for one instant. Internationalization in Go ends where it began: the language was global by default, so the work was never transcoding — it was negotiation that always answers, catalogs that never 500, formatters that take conventions as data, and time kept on its own axis. The realtime topic next is about cargo that never stops arriving."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"internationalization",
		"Internationalization from Go",
		"Go is global by default — UTF-8 everywhere, strings walked rune-wise — so i18n is three engines, not an encoding project: language negotiation, message catalogs, and locale-aware formatting — with time kept a separate axis.",
		"media-realtime",
		27,
		sections,
	)
}
