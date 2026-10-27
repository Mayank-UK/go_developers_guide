package i18n

import (
	"fmt"
	"strings"
)

// The catalog half of the guide's i18n engine (topic 27): the port a
// message store satisfies, the plural categories the guide's four demo
// locales need (selected by hand from CLDR's cardinal rules — the full
// six-category machinery of zero/one/two/few/many/other is exactly the
// job golang.org/x/text/message does, and the topic's prose says so),
// {placeholder} interpolation, and the Translator that ties catalog,
// locales, and fallback policy together. The engine never panics on a
// missing key: it serves the key itself and says so to a log callback —
// a user seeing an untranslated message is a bug worth a log line, not
// a 500.

// PluralCategory is a CLDR cardinal category. The guide's locales need
// two of the six CLDR defines — enough to show that "plural" is a
// per-locale rule, not a count.
type PluralCategory string

// The two categories the demo locales use. English and German pluralize
// around exactly 1; French includes 0; Japanese does not distinguish.
const (
	PluralOne   PluralCategory = "one"
	PluralOther PluralCategory = "other"
)

// pluralOneForOne is the English/German rule: one thing is singular,
// everything else — including zero — is plural.
func pluralOneForOne(count int) PluralCategory {
	if count == 1 {
		return PluralOne
	}
	return PluralOther
}

// pluralOneForZeroAndOne is the French rule: zero AND one are singular
// ("0 nouvelle publication" is correct French).
func pluralOneForZeroAndOne(count int) PluralCategory {
	if count == 0 || count == 1 {
		return PluralOne
	}
	return PluralOther
}

// pluralAlwaysOther is the Japanese rule: no grammatical plural, one
// form for every count.
func pluralAlwaysOther(int) PluralCategory { return PluralOther }

// Message is one catalog entry: a key and its forms keyed by plural
// category. Every message carries "other" (the universal form); "one"
// exists where the locale's rule can select it. Form text may hold
// {name} placeholders for Interpolate.
type Message struct {
	Key   string
	Forms map[PluralCategory]string
}

// Catalog is the message-store port: look up one key under one language
// tag, and list the tags the store carries. Implementations hold
// translated text and nothing else — no formatting policy, no fallback
// chain (the Translator owns both, which is why an in-memory map and an
// embed.FS-backed store are interchangeable behind it).
type Catalog interface {
	// Message answers the key's entry for the tag, or false when the
	// tag or the key is not carried.
	Message(tag, key string) (Message, bool)
	// Tags lists the carried tags in preference order — the negotiator's
	// supported list, default first.
	Tags() []string
}

// MissingKeyFunc receives missing-key notices (tag, key). Wire it to a
// slog line in production; wire it to a counter in the demo. A nil
// callback is legal and notices are dropped.
type MissingKeyFunc func(tag, key string)

// Translation is one rendered message plus where it actually came from.
type Translation struct {
	Text     string
	Served   string // the tag whose catalog answered (may be the default)
	Fallback bool   // true when the requested tag fell back (unknown locale, or key missing locally)
	Found    bool   // false when NO catalog carried the key — Text is the key itself
}

// Translator composes a Catalog, the Locale profiles, and the fallback
// policy into the one call handlers make. Constructed once at the
// composition root; safe for concurrent use (read-only after build).
type Translator struct {
	catalog    Catalog
	locales    map[string]Locale
	tags       []string
	defaultTag string
	onMissing  MissingKeyFunc
}

// NewTranslator validates the wiring — every catalog tag must have a
// locale profile, and the default tag must be carried by both — then
// builds the translator. A bad wiring fails startup, not a request.
func NewTranslator(catalog Catalog, locales []Locale, defaultTag string, onMissing MissingKeyFunc) (*Translator, error) {
	if catalog == nil {
		return nil, fmt.Errorf("i18n translator: catalog is nil")
	}
	if len(locales) == 0 {
		return nil, fmt.Errorf("i18n translator: no locale profiles")
	}
	byTag := make(map[string]Locale, len(locales))
	for _, locale := range locales {
		if locale.Tag == "" {
			return nil, fmt.Errorf("i18n translator: locale profile with empty tag")
		}
		if _, duplicate := byTag[locale.Tag]; duplicate {
			return nil, fmt.Errorf("i18n translator: duplicate locale profile %q", locale.Tag)
		}
		byTag[locale.Tag] = locale
	}
	tags := catalog.Tags()
	if len(tags) == 0 {
		return nil, fmt.Errorf("i18n translator: catalog carries no tags")
	}
	for _, tag := range tags {
		if _, ok := byTag[tag]; !ok {
			return nil, fmt.Errorf("i18n translator: catalog tag %q has no locale profile", tag)
		}
	}
	if _, ok := byTag[defaultTag]; !ok {
		return nil, fmt.Errorf("i18n translator: default tag %q has no locale profile", defaultTag)
	}
	if !carriesTag(tags, defaultTag) {
		return nil, fmt.Errorf("i18n translator: default tag %q is not carried by the catalog", defaultTag)
	}
	return &Translator{
		catalog:    catalog,
		locales:    byTag,
		tags:       tags,
		defaultTag: defaultTag,
		onMissing:  onMissing,
	}, nil
}

func carriesTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}

// Supported lists the negotiator's supported tags (the catalog's tags,
// preference order preserved).
func (t *Translator) Supported() []string {
	return append([]string(nil), t.tags...)
}

// Default answers the default tag — the chain's last link.
func (t *Translator) Default() string { return t.defaultTag }

// LocaleFor resolves a locale profile for any tag: exact, then base
// language ("fr-CA" → "fr"), then the default. The second return is the
// profile actually chosen; callers who need "did it fall back" compare
// against their own expectation.
func (t *Translator) LocaleFor(tag string) Locale {
	if locale, ok := t.localeFor(tag); ok {
		return locale
	}
	return t.locales[t.defaultTag]
}

func (t *Translator) localeFor(tag string) (Locale, bool) {
	if locale, ok := t.locales[strings.ToLower(strings.TrimSpace(tag))]; ok {
		return locale, true
	}
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	locale, ok := t.locales[base]
	return locale, ok
}

// Translate renders one key under one tag for one count. The fallback
// ladder, each rung visible in the returned Translation: resolve the
// LOCALE (exact → base → default); find the key in that locale's
// catalog, else in the DEFAULT catalog (the untranslated-locale
// fallback); select the plural form the locale's rule picks, falling to
// "other" when the exact category is absent; interpolate the
// placeholders. A key no catalog carries serves ITSELF as the text —
// visible, honest, logged — because a missing translation must never
// take the page down.
func (t *Translator) Translate(tag, key string, count int, vars map[string]string) Translation {
	locale, resolved := t.localeFor(tag)
	fellBack := false
	if !resolved {
		locale = t.locales[t.defaultTag]
		fellBack = true
	}
	served := locale.Tag

	message, found := t.catalog.Message(served, key)
	if !found {
		message, found = t.catalog.Message(t.defaultTag, key)
		served = t.defaultTag
		fellBack = true
	}
	if !found {
		if t.onMissing != nil {
			t.onMissing(strings.ToLower(strings.TrimSpace(tag)), key)
		}
		return Translation{Text: key, Served: served, Fallback: true, Found: false}
	}

	form, ok := message.Forms[locale.Plural(count)]
	if !ok || form == "" {
		form = message.Forms[PluralOther]
	}
	return Translation{
		Text:     Interpolate(form, vars),
		Served:   served,
		Fallback: fellBack || served != locale.Tag,
		Found:    true,
	}
}

// Interpolate replaces {name} placeholders with their values, rune-wise.
// An unknown placeholder is left verbatim on purpose — "{name}" on screen
// points at the code that forgot the variable, which beats silently
// dropping it. Placeholders nest nowhere and repeat fine.
func Interpolate(text string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(text, "{") {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '{' {
			out.WriteRune(runes[i])
			continue
		}
		closing := -1
		for j := i + 1; j < len(runes); j++ {
			if runes[j] == '}' {
				closing = j
				break
			}
			if runes[j] == '{' {
				break
			}
		}
		if closing == -1 {
			out.WriteRune(runes[i])
			continue
		}
		name := strings.TrimSpace(string(runes[i+1 : closing]))
		if value, ok := vars[name]; ok {
			out.WriteString(value)
		} else {
			out.WriteString(string(runes[i : closing+1]))
		}
		i = closing
	}
	return out.String()
}

// mapCatalog is the in-memory Catalog — Go source as the catalog store.
// It is the whole adapter for a small service, and the shape an
// embed.FS loader builds into (the guide's data layer does exactly
// that, from checked-in JSON).
type mapCatalog struct {
	messages map[string]map[string]Message
	tags     []string
}

// NewMapCatalog builds a Catalog from per-tag message maps. Tags keep
// the given order (default first is the convention the negotiator's
// supported list expects); duplicate tags are refused. Every message
// must carry a non-empty "other" form — the universal fallback the
// translator assumes exists.
func NewMapCatalog(messages map[string]map[string]Message, tags []string) (Catalog, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("map catalog: no tags")
	}
	ordered := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if seen[tag] {
			return nil, fmt.Errorf("map catalog: duplicate tag %q", tag)
		}
		seen[tag] = true
		ordered = append(ordered, tag)
	}
	store := make(map[string]map[string]Message, len(messages))
	for tag, entries := range messages {
		if !seen[tag] {
			return nil, fmt.Errorf("map catalog: messages for unlisted tag %q", tag)
		}
		byKey := make(map[string]Message, len(entries))
		for key, message := range entries {
			if key == "" {
				return nil, fmt.Errorf("map catalog: empty key under %q", tag)
			}
			if message.Key == "" {
				message.Key = key
			}
			if message.Forms[PluralOther] == "" {
				return nil, fmt.Errorf("map catalog: key %q under %q lacks an %q form", key, tag, PluralOther)
			}
			byKey[key] = message
		}
		store[tag] = byKey
	}
	return &mapCatalog{messages: store, tags: append([]string(nil), ordered...)}, nil
}

// Message implements Catalog.
func (c *mapCatalog) Message(tag, key string) (Message, bool) {
	entries, ok := c.messages[strings.ToLower(tag)]
	if !ok {
		return Message{}, false
	}
	message, ok := entries[key]
	return message, ok
}

// Tags implements Catalog.
func (c *mapCatalog) Tags() []string { return append([]string(nil), c.tags...) }
