package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Snippet-source verification: is a teaching snippet quoted from a real
// file? The matcher answers one question: after language-aware
// normalization, is the snippet an ORDERED SUBSEQUENCE of the source's
// lines? Normalization strips blank lines and comments (full-line comments
// from both sides; trailing teaching comments from code lines), so snippets
// may legitimately add prose comments and "// …" / "// ..." elision lines —
// an elision normalizes to empty and therefore marks a GAP: source lines
// may be skipped between matches, but every remaining snippet line must
// equal a source line exactly (after normalization + trim), in order.
//
// Comment syntax is language-aware and quote-aware (a comment token inside
// a string literal is not a comment — url := "http://x" survives "//"
// stripping in go/ts). "--" (sql) and "#" (yaml/toml) only start a comment
// at line start or after whitespace, so i-- and #field never get mangled.
// JSON has no comments — blank-strip only. The scan is a single
// left-to-right pass per line (two-pointer match over the normalized
// lists): linear time, no regex backtracking.

// maxReasonLine caps a line quoted in a failure reason so it stays readable.
const maxReasonLine = 80

// commentStyle describes how comments are written in one language family.
type commentStyle struct {
	// lineTokens start a comment anywhere on the line (trailing included).
	lineTokens []string
	// startOnlyTokens start a comment only at line start or after whitespace.
	startOnlyTokens []string
	// blockStart/blockEnd delimit block comments when both are set.
	blockStart string
	blockEnd   string
	// quoteAware tracks string literals so comment tokens inside them are
	// content, not comments.
	quoteAware bool
}

// plainStyle is the zero style: unknown languages and json get no comment
// syntax, blank-strip only.
var plainStyle = commentStyle{}

// commentStyles maps a language (lowercase) to its comment syntax — the
// family table from the fastapi guide, extended with the Go guide's
// languages: go/mod carry // line comments plus /* */ blocks; xml shares
// html's <!-- --> block comments.
var commentStyles = map[string]commentStyle{
	"go":    {lineTokens: []string{"//"}, blockStart: "/*", blockEnd: "*/", quoteAware: true},
	"mod":   {lineTokens: []string{"//"}, blockStart: "/*", blockEnd: "*/", quoteAware: true},
	"ts":    {lineTokens: []string{"//"}, quoteAware: true},
	"tsx":   {lineTokens: []string{"//"}, quoteAware: true},
	"js":    {lineTokens: []string{"//"}, quoteAware: true},
	"proto": {lineTokens: []string{"//"}, blockStart: "/*", blockEnd: "*/", quoteAware: true},
	"css":   {lineTokens: []string{"//"}, blockStart: "/*", blockEnd: "*/", quoteAware: true},
	"toml":  {startOnlyTokens: []string{"#"}},
	"yaml":  {startOnlyTokens: []string{"#"}},
	"yml":   {startOnlyTokens: []string{"#"}},
	"sql":   {startOnlyTokens: []string{"--"}},
	"html":  {blockStart: "<!--", blockEnd: "-->"},
	"xml":   {blockStart: "<!--", blockEnd: "-->"},
}

const (
	quoteChars = "'\"`"
	whitespace = " \t"
)

// atTokenBoundary reports whether a start-only comment token sits at line
// start or after whitespace.
func atTokenBoundary(codeSoFar string) bool {
	return codeSoFar == "" || strings.ContainsRune(whitespace, rune(codeSoFar[len(codeSoFar)-1]))
}

// hasPrefixAt reports whether text starts with token at index i.
func hasPrefixAt(text, token string, i int) bool {
	return strings.HasPrefix(text[i:], token)
}

// stripLine strips comments from one line; it returns the code fragment and
// whether a block comment is still open. The single left-to-right pass
// tracks string literals (with backslash escapes), full and inline block
// comments, and line-comment tokens — no regex, no backtracking.
func stripLine(line string, style commentStyle, inBlock bool) (string, bool) {
	text := line
	if inBlock {
		if style.blockEnd == "" {
			// Unreachable by construction: every style with a block start
			// also sets its end.
			return strings.TrimSpace(text), false
		}
		end := strings.Index(text, style.blockEnd)
		if end == -1 {
			return "", true
		}
		text = text[end+len(style.blockEnd):]
	}

	var out strings.Builder
	var quote byte
	for i := 0; i < len(text); i++ {
		ch := text[i]
		next := byte(0)
		if i+1 < len(text) {
			next = text[i+1]
		}
		if quote != 0 {
			out.WriteByte(ch)
			if ch == '\\' && next != 0 {
				out.WriteByte(next)
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if style.quoteAware && strings.IndexByte(quoteChars, ch) != -1 {
			quote = ch
			out.WriteByte(ch)
			continue
		}
		if style.blockStart != "" && style.blockEnd != "" && hasPrefixAt(text, style.blockStart, i) {
			end := strings.Index(text[i+len(style.blockStart):], style.blockEnd)
			if end == -1 {
				return strings.TrimRight(out.String(), whitespace), true
			}
			i = i + len(style.blockStart) + end + len(style.blockEnd) - 1
			continue
		}
		for _, token := range style.lineTokens {
			if hasPrefixAt(text, token, i) {
				return strings.TrimRight(out.String(), whitespace), false
			}
		}
		for _, token := range style.startOnlyTokens {
			if hasPrefixAt(text, token, i) && atTokenBoundary(out.String()) {
				return strings.TrimRight(out.String(), whitespace), false
			}
		}
		out.WriteByte(ch)
	}
	return strings.TrimRight(out.String(), whitespace), false
}

// normalize reduces text to its meaningful lines: comments and blanks
// stripped, trailing carriage returns dropped, code trimmed. Full-line
// comments — including elision lines (comment token + "…" or "...") —
// normalize to empty and drop out, which is what makes gaps legal during
// matching.
func normalize(text string, style commentStyle) []string {
	var normalized []string
	inBlock := false
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimSuffix(raw, "\r")
		code, stillInBlock := stripLine(raw, style, inBlock)
		inBlock = stillInBlock
		code = strings.TrimSpace(code)
		if code != "" {
			normalized = append(normalized, code)
		}
	}
	return normalized
}

// short caps a line for failure reasons so they stay readable.
func short(line string) string {
	if len(line) <= maxReasonLine {
		return line
	}
	return line[:maxReasonLine] + "…"
}

// MatchSnippet verifies the snippet is an ordered subsequence of the
// source, per language. Unknown languages (and json) get blank-strip-only
// normalization. It returns ok=true on success, or ok=false with a short
// reason naming the first snippet line that could not be matched in order.
func MatchSnippet(source, snippet, language string) (ok bool, reason string) {
	style, known := commentStyles[strings.ToLower(language)]
	if !known {
		style = plainStyle
	}
	sourceLines := normalize(source, style)
	snippetLines := normalize(snippet, style)

	if len(snippetLines) == 0 {
		return false, "snippet is empty after normalization"
	}

	position := 0
	for number, line := range snippetLines {
		for position < len(sourceLines) && sourceLines[position] != line {
			position++
		}
		if position == len(sourceLines) {
			return false, fmt.Sprintf(
				"snippet line %d not found in source (in order): %q", number+1, short(line),
			)
		}
		position++
	}
	return true, ""
}

// RepoRoot locates the repository root for snippet-source resolution:
// sources are repo-root-relative by contract (e.g.
// "server/internal/core/config.go"), while tests run with their package
// directory as the working directory. It walks up from the working
// directory to the first directory containing go.mod — the server module —
// and returns that directory's parent. The walk stops at the first go.mod,
// so a stray module higher up can never shadow the real root.
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("find repo root: %w", err)
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			root := filepath.Dir(dir)
			if root == dir {
				// The module dir is the filesystem root — no parent exists.
				return "", fmt.Errorf("find repo root: module at %q has no parent", dir)
			}
			return root, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("find repo root: no go.mod above %s", start)
		}
		dir = parent
	}
}
