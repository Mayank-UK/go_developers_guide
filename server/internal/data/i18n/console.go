// Package i18n is the guide's i18n data layer (topic 27): the message
// catalogs as checked-in JSON, embedded into the binary with embed.FS
// and loaded into the domain Catalog at boot — the single-binary story
// Go gives internationalized services for free. The tzdata blank
// import below embeds the IANA zone database too, so time.LoadLocation
// has a guaranteed last resort in slim containers (a scratch or
// distroless image has no /usr/share/zoneinfo to read).
package i18n

import (
	"context"
	"embed" // the locale files below compile into the binary
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embedded zone database: LoadLocation's last resort

	"guide/internal/domain/demos"
	domi18n "guide/internal/domain/i18n"
)

// localeDir is the embedded directory holding one JSON catalog per
// language tag.
const localeDir = "locales"

//go:embed locales/*.json
var localeFiles embed.FS

// preferredTags is the catalog's preference order — the negotiator's
// supported list, default first. The loader requires the embedded file
// set to match it exactly, so a stray or missing locale fails startup
// rather than a request.
var preferredTags = []string{"en", "de", "fr", "ja"}

// DefaultTag is the fallback catalog: the chain's last link.
const DefaultTag = "en"

// storyInstant is the fixed instant every rendering in the demo
// derives from — deterministic request over request, and deliberately
// placed two days after the EU DST changeover and five before the US
// one, so Berlin and New York render under DIFFERENT DST rules (CET
// vs EDT) from one moment.
var storyInstant = time.Date(2026, time.October, 27, 12, 30, 0, 0, time.UTC)

// storyZones are the IANA zones the timezone beat renders.
var storyZones = []string{"Europe/Berlin", "America/New_York", "Asia/Tokyo"}

// honestyZone does not exist — the beat shows LoadLocation's honest
// error for it.
const honestyZone = "Mars/Olympus"

// missingKeyDemo is the key the missing-key policy beat asks for; no
// catalog carries it, on purpose.
const missingKeyDemo = "demo.never_translated"

// Console is the topic's substrate, wired once at the composition
// root: the translator over the embedded catalogs (with missing keys
// logged through slog) and the raw catalog for the story's per-run
// translator. The negotiator STORY is read-only over this state, so
// every request tells the same deterministic walk.
type Console struct {
	translator *domi18n.Translator
	catalog    domi18n.Catalog
}

// NewConsole builds the console: load and validate the embedded
// catalogs, then wire the domain translator over them. Any bad locale
// file — unknown plural category, missing "other" form, file set
// mismatch — fails here, at boot, with the file named.
func NewConsole() (*Console, error) {
	catalog, err := loadEmbeddedCatalog(localeFiles)
	if err != nil {
		return nil, fmt.Errorf("i18n console: %w", err)
	}
	translator, err := domi18n.NewTranslator(catalog, domi18n.SupportedLocales(), DefaultTag, func(tag, key string) {
		slog.Warn("missing i18n catalog key — serving the key itself", "tag", tag, "key", key)
	})
	if err != nil {
		return nil, fmt.Errorf("i18n console: %w", err)
	}
	return &Console{translator: translator, catalog: catalog}, nil
}

// loadEmbeddedCatalog reads one JSON file per tag ({"key": {"one":
// "…", "other": "…"}}) and builds the domain Catalog. JSON is chosen
// because translators hand back JSON-shaped files; the loader is the
// single seam where a Gettext or XLIFF pipeline would slot in.
func loadEmbeddedCatalog(files fs.FS) (domi18n.Catalog, error) {
	messages := map[string]map[string]domi18n.Message{}
	for _, tag := range preferredTags {
		name := path.Join(localeDir, tag+".json")
		raw, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("read catalog %s: %w", name, err)
		}
		var entries map[string]map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("parse catalog %s: %w", name, err)
		}
		byKey := make(map[string]domi18n.Message, len(entries))
		keys := make([]string, 0, len(entries))
		for key := range entries {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			forms := map[domi18n.PluralCategory]string{}
			for category, text := range entries[key] {
				switch domi18n.PluralCategory(category) {
				case domi18n.PluralOne, domi18n.PluralOther:
					forms[domi18n.PluralCategory(category)] = text
				default:
					return nil, fmt.Errorf("catalog %s: key %q carries unknown plural category %q", name, key, category)
				}
			}
			if strings.TrimSpace(forms[domi18n.PluralOther]) == "" {
				return nil, fmt.Errorf("catalog %s: key %q lacks a non-empty %q form", name, key, domi18n.PluralOther)
			}
			byKey[key] = domi18n.Message{Key: key, Forms: forms}
		}
		messages[tag] = byKey
	}
	// A stray locale file outside the preferred set is a wiring smell:
	// name it instead of silently ignoring it.
	listing, err := fs.ReadDir(files, localeDir)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", localeDir, err)
	}
	for _, entry := range listing {
		tag := strings.TrimSuffix(entry.Name(), ".json")
		if !carries(preferredTags, tag) {
			return nil, fmt.Errorf("catalog file %s is not in the preferred tag set", entry.Name())
		}
	}
	return domi18n.NewMapCatalog(messages, preferredTags)
}

func carries(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

// RunNegotiatorStory walks the whole topic from one header: parse it,
// negotiate the catalog, render the same message everywhere, run the
// plural and format matrices, read one instant through three zones,
// and exercise the two honest edges (an unsupported language, a
// missing key). Everything derives from the real engines — nothing is
// staged. A header with nothing usable in it runs the RFC 9110 fixture
// instead (and says so), so the story always has something to walk.
func (c *Console) RunNegotiatorStory(ctx context.Context, header, headerSource string) (demos.I18nNegotiatorRun, error) {
	if strings.TrimSpace(header) == "" {
		header = demos.I18nFixtureHeader
		headerSource = "fixture"
	}
	supported := c.translator.Supported()
	negotiation := domi18n.Negotiate(header, supported, DefaultTag)

	parsed := domi18n.ParseAcceptLanguage(header)
	candidates := make([]demos.I18nCandidateRow, 0, len(parsed))
	for _, pref := range parsed {
		candidates = append(candidates, demos.I18nCandidateRow{Range: pref.Range, Quality: pref.Quality})
	}
	chain := make([]demos.I18nChainRow, 0, len(negotiation.Chain))
	for _, step := range negotiation.Chain {
		chain = append(chain, demos.I18nChainRow{
			Range: step.Range, Quality: step.Quality, Outcome: step.Outcome, Matched: step.Matched,
		})
	}

	greetings := make([]demos.I18nMessageRow, 0, len(supported))
	for _, tag := range supported {
		translation := c.translator.Translate(tag, "demo.greeting", 1, map[string]string{"name": "Ada"})
		greetings = append(greetings, demos.I18nMessageRow{
			Tag: tag, Text: translation.Text, Served: translation.Served, Found: translation.Found,
		})
	}

	plurals := make([]demos.I18nPluralRow, 0, len(supported)*3)
	for _, tag := range supported {
		locale := c.translator.LocaleFor(tag)
		for _, count := range []int{0, 1, 2} {
			translation := c.translator.Translate(tag, "demo.new_posts", count, map[string]string{
				"count": strconv.Itoa(count),
			})
			plurals = append(plurals, demos.I18nPluralRow{
				Tag: tag, Count: count, Category: string(locale.Plural(count)), Text: translation.Text,
			})
		}
	}

	formats := make([]demos.I18nFormatRow, 0, len(supported))
	for _, tag := range supported {
		locale := c.translator.LocaleFor(tag)
		formats = append(formats, demos.I18nFormatRow{
			Tag:      tag,
			Number:   domi18n.FormatNumber(locale, 1234567.89, 2),
			Currency: domi18n.FormatCurrency(locale, 1899),
			Date:     domi18n.FormatDate(locale, storyInstant),
			Clock:    domi18n.FormatClock(locale, storyInstant),
		})
	}

	zones := make([]demos.I18nZoneRow, 0, len(storyZones))
	english := c.translator.LocaleFor("en")
	german := c.translator.LocaleFor("de")
	for _, name := range storyZones {
		location, err := time.LoadLocation(name)
		if err != nil {
			return demos.I18nNegotiatorRun{}, fmt.Errorf("load zone %s: %w", name, err)
		}
		local := storyInstant.In(location)
		zones = append(zones, demos.I18nZoneRow{
			Zone:         name,
			Abbreviation: local.Format("MST"),
			Offset:       local.Format("-07:00"),
			ClockEN:      domi18n.FormatClock(english, local),
			ClockDE:      domi18n.FormatClock(german, local),
		})
	}
	_, honestyErr := time.LoadLocation(honestyZone)

	composed := make([]demos.I18nMessageRow, 0, 2)
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return demos.I18nNegotiatorRun{}, fmt.Errorf("load zone Europe/Berlin: %w", err)
	}
	berlinLocal := storyInstant.In(berlin)
	for _, tag := range []string{"en", "de"} {
		locale := c.translator.LocaleFor(tag)
		translation := c.translator.Translate(tag, "demo.published_at", 1, map[string]string{
			"date": domi18n.FormatDate(locale, berlinLocal),
			"time": domi18n.FormatClock(locale, berlinLocal),
			"zone": "Europe/Berlin",
		})
		composed = append(composed, demos.I18nMessageRow{
			Tag: tag, Text: translation.Text, Served: translation.Served, Found: translation.Found,
		})
	}

	fallback := domi18n.Negotiate(demos.I18nUnsupportedHeader, supported, DefaultTag)
	fallbackChain := make([]demos.I18nChainRow, 0, len(fallback.Chain))
	for _, step := range fallback.Chain {
		fallbackChain = append(fallbackChain, demos.I18nChainRow{
			Range: step.Range, Quality: step.Quality, Outcome: step.Outcome, Matched: step.Matched,
		})
	}

	// The missing-key beat runs through a story-scoped translator so
	// its notice can be SHOWN rather than inferred from the server log.
	var notices []string
	storyTranslator, err := domi18n.NewTranslator(c.catalog, domi18n.SupportedLocales(), DefaultTag, func(tag, key string) {
		notices = append(notices, fmt.Sprintf("missing i18n catalog key %q for %q — serving the key itself", key, tag))
	})
	if err != nil {
		return demos.I18nNegotiatorRun{}, fmt.Errorf("story translator: %w", err)
	}
	missing := storyTranslator.Translate("de", missingKeyDemo, 1, nil)
	logLine := "(no notice emitted)"
	if len(notices) > 0 {
		logLine = notices[0]
	}

	return demos.I18nNegotiatorRun{
		Header:       header,
		HeaderSource: headerSource,
		Candidates:   candidates,
		Selected:     negotiation.Selected,
		Via:          negotiation.Via,
		Chain:        chain,
		Greetings:    greetings,
		Plurals:      plurals,
		Formats:      formats,
		Instant:      storyInstant.Format(time.RFC3339),
		Zones:        zones,
		ZoneHonesty:  demos.I18nZoneHonestyRow{Zone: honestyZone, Error: honestyErr.Error()},
		Composed:     composed,
		Fallback: demos.I18nFallbackRow{
			Header: demos.I18nUnsupportedHeader, Chain: fallbackChain, Selected: fallback.Selected, Via: fallback.Via,
		},
		MissingKey: demos.I18nMissingKeyRow{Key: missingKeyDemo, Text: missing.Text, LogLine: logLine},
	}, nil
}
