// The spec's own YAML reader — a deliberately small parser for the
// controlled subset server/api/openapi.yaml is written in, so the whole
// spec-first toolchain (walker, drift tests, demo facts) reads the
// document with ZERO third-party dependencies: the guide server stays
// stdlib-only, and the parser's strictness is a feature (a spec that
// wanders outside the subset fails loudly instead of parsing into
// something subtly wrong).
//
// The subset, all of it visible in the guide's spec:
//
//   - two-space indentation; nested blocks are always deeper than their
//     parent key, sequence continuation keys sit at dash-indent + 2;
//   - block mappings (key: value) and block sequences (- item), including
//     sequences of mappings written as "- key: value" with continuation;
//   - flow sequences of scalars ([a, b, c]);
//   - block scalars (key: |) for multi-line description text;
//   - quoted keys ("200":) and quoted scalar values (escapes \" and \\);
//   - plain scalars typed as bool, int, or string.
//
// Not supported (by design): anchors, aliases, tags, multi-document
// streams, flow mappings, complex keys, line folding beyond the block
// forms above. The parser errors on tabs in indentation and on a key
// line that cannot be split — the two authoring mistakes a spec editor
// actually makes.
package apispec

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSpecYAML parses one OpenAPI document written in the supported
// subset into the same generic shapes the walker walks: map[string]any,
// []any, string, bool, int.
func ParseSpecYAML(raw []byte) (map[string]any, error) {
	lines, err := scanLines(raw)
	if err != nil {
		return nil, err
	}
	pos := 0
	value, err := parseBlock(lines, &pos, 0)
	if err != nil {
		return nil, err
	}
	document, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("spec document must be a YAML mapping, got %T", value)
	}
	return document, nil
}

// yamlLine is one raw source line with its indentation measured; blank
// and comment lines survive scanning so block scalars keep their shape,
// and are skipped only where the grammar says so.
type yamlLine struct {
	indent int
	text   string
	number int
}

// scanLines measures indentation and rejects tabs; nothing else.
func scanLines(raw []byte) ([]yamlLine, error) {
	var lines []yamlLine
	for i, rawLine := range strings.Split(string(raw), "\n") {
		rawLine = strings.TrimSuffix(rawLine, "\r")
		indent := 0
		for indent < len(rawLine) && rawLine[indent] == ' ' {
			indent++
		}
		if indent < len(rawLine) && rawLine[indent] == '\t' {
			return nil, fmt.Errorf("line %d: tab in indentation (the spec uses spaces)", i+1)
		}
		lines = append(lines, yamlLine{indent: indent, text: rawLine[indent:], number: i + 1})
	}
	return lines, nil
}

// skippable reports whether a line is blank or a full-line comment — the
// two line kinds the mapping and sequence grammars ignore.
func skippable(line yamlLine) bool {
	trimmed := strings.TrimSpace(line.text)
	return trimmed == "" || strings.HasPrefix(trimmed, "#")
}

// skip advances past skippable lines.
func skip(lines []yamlLine, pos *int) {
	for *pos < len(lines) && skippable(lines[*pos]) {
		*pos++
	}
}

// parseBlock parses the node starting at the next non-skippable line,
// which must be indented at least minIndent. An absent block is nil.
func parseBlock(lines []yamlLine, pos *int, minIndent int) (any, error) {
	skip(lines, pos)
	if *pos >= len(lines) || lines[*pos].indent < minIndent {
		return nil, nil
	}
	indent := lines[*pos].indent
	if isSequenceEntry(lines[*pos].text) {
		return parseSequence(lines, pos, indent)
	}
	return parseMapping(lines, pos, indent)
}

// isSequenceEntry reports whether a line opens a sequence item.
func isSequenceEntry(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

// parseMapping consumes "key: value" lines at exactly indent. A value is
// inline (scalar or flow sequence), a nested block on following lines,
// or a block scalar; deeper unexpected indentation is an error, not a
// guess.
func parseMapping(lines []yamlLine, pos *int, indent int) (map[string]any, error) {
	mapping := map[string]any{}
	for {
		skip(lines, pos)
		if *pos >= len(lines) || lines[*pos].indent < indent {
			return mapping, nil
		}
		line := lines[*pos]
		if line.indent > indent {
			return nil, fmt.Errorf("line %d: unexpected indent %d under mapping at %d", line.number, line.indent, indent)
		}
		if isSequenceEntry(line.text) {
			return nil, fmt.Errorf("line %d: sequence item where a mapping key was expected", line.number)
		}
		key, rest, err := splitKey(line)
		if err != nil {
			return nil, err
		}
		*pos++

		switch {
		case rest == "":
			value, err := parseBlock(lines, pos, indent+1)
			if err != nil {
				return nil, err
			}
			mapping[key] = value
		case rest == "|" || rest == "|-" || rest == ">":
			mapping[key] = readBlockScalar(lines, pos, indent)
		case strings.HasPrefix(rest, "["):
			value, err := parseFlowSequence(rest, line.number)
			if err != nil {
				return nil, err
			}
			mapping[key] = value
		case strings.HasPrefix(rest, "\""):
			value, err := unquoteQuotedScalar(rest, line.number)
			if err != nil {
				return nil, err
			}
			mapping[key] = value
		default:
			mapping[key] = decodeScalar(rest)
		}
	}
}

// parseSequence consumes "- …" items at exactly indent. An item is a
// scalar, a nested block, or an inline mapping whose continuation keys
// sit at dash-indent + 2 — the only continuation style the spec uses.
func parseSequence(lines []yamlLine, pos *int, indent int) ([]any, error) {
	var items []any
	for {
		skip(lines, pos)
		if *pos >= len(lines) || lines[*pos].indent < indent {
			return items, nil
		}
		line := lines[*pos]
		if line.indent > indent {
			return nil, fmt.Errorf("line %d: unexpected indent %d under sequence at %d", line.number, line.indent, indent)
		}
		if !isSequenceEntry(line.text) {
			return nil, fmt.Errorf("line %d: mapping key where a sequence item was expected", line.number)
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line.text, "-"))
		if rest == "" {
			// A bare dash: the item is the block that follows.
			*pos++
			value, err := parseBlock(lines, pos, indent+1)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
			continue
		}
		if _, _, err := splitKey(yamlLine{indent: indent, text: rest, number: line.number}); err == nil {
			// "- key: value" — rewrite the line as its first mapping line
			// at the continuation column and let parseMapping do the rest.
			lines[*pos] = yamlLine{indent: indent + 2, text: rest, number: line.number}
			value, err := parseMapping(lines, pos, indent+2)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
			continue
		}
		if strings.HasPrefix(rest, "\"") {
			value, err := unquoteQuotedScalar(rest, line.number)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
			*pos++
			continue
		}
		items = append(items, decodeScalar(rest))
		*pos++
	}
}

// unquoteQuotedScalar validates that text is exactly one quoted scalar
// and decodes it.
func unquoteQuotedScalar(text string, lineNo int) (string, error) {
	end := findClosingQuote(text, 0)
	if end != len(text)-1 {
		return "", fmt.Errorf("line %d: trailing content after quoted scalar %q", lineNo, text)
	}
	value, err := unquote(text[1:end])
	if err != nil {
		return "", fmt.Errorf("line %d: %w", lineNo, err)
	}
	return value, nil
}

// splitKey splits "key: rest", honoring a quoted key. The colon must be
// followed by a space or end of line — the YAML rule that keeps URLs and
// timestamps inside values.
func splitKey(line yamlLine) (key, rest string, err error) {
	text := line.text
	if strings.HasPrefix(text, "\"") {
		end := findClosingQuote(text, 0)
		if end < 0 || end+1 >= len(text) || text[end+1] != ':' {
			return "", "", fmt.Errorf("line %d: quoted key lacks a colon", line.number)
		}
		key, err = unquote(text[1:end])
		if err != nil {
			return "", "", fmt.Errorf("line %d: %w", line.number, err)
		}
		return key, strings.TrimSpace(text[end+2:]), nil
	}
	for i := 0; i < len(text); i++ {
		if text[i] == ':' && (i+1 == len(text) || text[i+1] == ' ') {
			return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:]), nil
		}
	}
	return "", "", fmt.Errorf("line %d: no key colon in %q", line.number, text)
}

// readBlockScalar consumes the indented block after "key: |" and returns
// its text. Blank lines inside the block are preserved; trailing blanks
// are dropped. The subset treats block scalars as PROSE: since scanning
// measures and strips each line's indentation, relative extra indentation
// inside a block is not preserved — descriptions do not need it.
func readBlockScalar(lines []yamlLine, pos *int, indent int) string {
	var content []string
	for *pos < len(lines) {
		line := lines[*pos]
		if strings.TrimSpace(line.text) == "" {
			content = append(content, "")
			*pos++
			continue
		}
		if line.indent <= indent {
			break
		}
		content = append(content, strings.TrimRight(line.text, " "))
		*pos++
	}
	for len(content) > 0 && content[len(content)-1] == "" {
		content = content[:len(content)-1]
	}
	if len(content) == 0 {
		return ""
	}
	return strings.Join(content, "\n") + "\n"
}

// parseFlowSequence parses "[a, b, c]" — scalars only, quoted or plain.
func parseFlowSequence(text string, lineNo int) ([]any, error) {
	if !strings.HasSuffix(text, "]") {
		return nil, fmt.Errorf("line %d: unterminated flow sequence %q", lineNo, text)
	}
	inner := strings.TrimSpace(text[1 : len(text)-1])
	items := []any{}
	if inner == "" {
		return items, nil
	}
	for _, part := range splitFlowItems(inner) {
		trimmed := strings.TrimSpace(part)
		if strings.HasPrefix(trimmed, "\"") {
			value, err := unquote(trimmed[1 : len(trimmed)-1])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			items = append(items, value)
			continue
		}
		items = append(items, decodeScalar(trimmed))
	}
	return items, nil
}

// splitFlowItems splits on commas that are not inside quotes.
func splitFlowItems(inner string) []string {
	var parts []string
	var current strings.Builder
	quoted := false
	for i := 0; i < len(inner); i++ {
		switch {
		case inner[i] == '\\' && quoted && i+1 < len(inner):
			current.WriteByte(inner[i])
			current.WriteByte(inner[i+1])
			i++
		case inner[i] == '"':
			quoted = !quoted
			current.WriteByte(inner[i])
		case inner[i] == ',' && !quoted:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteByte(inner[i])
		}
	}
	return append(parts, current.String())
}

// decodeScalar types one plain scalar: bool, int, or string.
func decodeScalar(text string) any {
	switch text {
	case "true":
		return true
	case "false":
		return false
	case "null", "~":
		return nil
	}
	if n, err := strconv.Atoi(text); err == nil {
		return n
	}
	return text
}

// findClosingQuote returns the index of the closing double quote starting
// the scan at start (which must be the opening quote).
func findClosingQuote(text string, start int) int {
	for i := start + 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}

// unquote decodes the escapes the subset allows.
func unquote(text string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '\\' {
			out.WriteByte(text[i])
			continue
		}
		if i+1 >= len(text) {
			return "", fmt.Errorf("dangling escape in %q", text)
		}
		i++
		switch text[i] {
		case '"', '\\':
			out.WriteByte(text[i])
		default:
			return "", fmt.Errorf("unsupported escape \\%c in %q", text[i], text)
		}
	}
	return out.String(), nil
}
