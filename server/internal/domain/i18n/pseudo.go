package i18n

import (
	"strings"
)

// Pseudo-locale testing (topic 27's translation-workflow section): a
// mechanical transform that turns a working English string into a
// visibly WRONG-but-structured one, so a UI can be exercised before any
// translator has touched it. Two defects it surfaces that nothing else
// catches early: hardcoded strings (a pseudo-ized catalog still reads
// plain English wherever the code glued text together) and layout
// breakage (real translations run ~30% longer than English; padding
// makes every fixed-width box, truncation, and overflow visible). This
// is the by-hand version of what CLDR's en-XA pseudo-locale encodes.

// pseudoAccent maps ASCII vowels to accented look-alikes — enough
// weirdness to expose encoding bugs (a pipeline that mangles UTF-8 will
// mangle these) without making the string unreadable.
var pseudoAccent = map[rune]rune{
	'a': 'á', 'e': 'é', 'i': 'í', 'o': 'ó', 'u': 'ú', 'y': 'ý',
	'A': 'Á', 'E': 'É', 'I': 'Í', 'O': 'Ó', 'U': 'Ú', 'Y': 'Ý',
}

// pseudoPadFraction is how much longer than the source the pseudo
// string gets: 30%, the number localization teams quote for real
// translations out of English.
const pseudoPadFraction = 0.3

// PseudoLocalize transforms message text for pseudo-locale testing:
// bracketed so it is unmistakably not a real translation, accented so
// encoding bugs surface, and padded with '×' so length-sensitive layout
// breaks before a single translator is hired. Placeholders ({name}) are
// preserved verbatim — the pseudo string must still interpolate, or the
// test itself would be lying.
func PseudoLocalize(text string) string {
	var out strings.Builder
	out.WriteString("[")
	out.WriteString(pseudoBody(text))
	padding := int(float64(countRunes(text))*pseudoPadFraction) + 1
	for i := 0; i < padding; i++ {
		out.WriteRune('×')
	}
	out.WriteString("]")
	return out.String()
}

// pseudoBody accents the text while leaving {placeholders} untouched.
func pseudoBody(text string) string {
	var out strings.Builder
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '{' {
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
			if closing != -1 {
				out.WriteString(string(runes[i : closing+1]))
				i = closing
				continue
			}
		}
		if accent, ok := pseudoAccent[runes[i]]; ok {
			out.WriteRune(accent)
			continue
		}
		out.WriteRune(runes[i])
	}
	return out.String()
}

// countRunes counts text's runes — the honest length of a string,
// distinct from len(text), which counts bytes (the global-by-default
// section's whole point in one call).
func countRunes(text string) int {
	count := 0
	for range text {
		count++
	}
	return count
}
