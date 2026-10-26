// The drift walker: validates one decoded JSON value against the spec's
// schemas, parsed straight from the OpenAPI YAML — no schema library, no
// dependencies beyond the YAML decoder. This is the engine behind the
// drift tests the api-documentation topic teaches: a response that
// disagrees with the spec (a renamed key, a missing required field, a
// wrong type, a value outside an enum or bound, a timestamp that is not
// RFC 3339) produces a violation naming the JSON path, so a spec that
// lies about the live wire FAILS the suite instead of misleading clients.
package apispec

import (
	"fmt"
	"sort"
	"time"
)

// Walker holds the spec's resolved schemas: components/schemas from the
// OpenAPI document, indexed by name so $ref pointers resolve locally.
type Walker struct {
	schemas map[string]any
}

// NewWalker parses the raw OpenAPI YAML (through the package's own
// zero-dependency subset parser) and indexes its named schemas. The
// spec must carry a components/schemas map; an unparseable document is an
// error, not a silent empty walker.
func NewWalker(raw []byte) (*Walker, error) {
	document, err := ParseSpecYAML(raw)
	if err != nil {
		return nil, fmt.Errorf("parse OpenAPI yaml: %w", err)
	}
	components, _ := document["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if len(schemas) == 0 {
		return nil, fmt.Errorf("OpenAPI document carries no components/schemas")
	}
	return &Walker{schemas: schemas}, nil
}

// Schemas exposes the schema names the walker resolved, sorted — the
// inventory the docs-link demo renders and the tests pin.
func (w *Walker) Schemas() []string {
	names := make([]string, 0, len(w.schemas))
	for name := range w.schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Validate walks value against the named schema and returns every
// violation as "path: what disagreed". An empty result means the value
// conforms — the spec and the wire agree at every field this schema
// describes.
func (w *Walker) Validate(value any, schemaName string) []string {
	schema, ok := w.schemas[schemaName]
	if !ok {
		return []string{fmt.Sprintf("$: unknown schema %q", schemaName)}
	}
	return w.walk(value, schema, "$")
}

// walk is the recursive heart: one JSON value against one OpenAPI 3.1
// schema (decoded from YAML as maps, slices, and scalars). Every branch
// returns human-readable violations anchored at the JSON path.
func (w *Walker) walk(value any, schema any, path string) []string {
	node, ok := schema.(map[string]any)
	if !ok {
		return nil // a bare schema (e.g. {}) constrains nothing
	}
	if ref, ok := node["$ref"].(string); ok {
		resolved, err := w.resolveRef(ref)
		if err != nil {
			return []string{fmt.Sprintf("%s: %v", path, err)}
		}
		return w.walk(value, resolved, path)
	}
	if branches, ok := node["oneOf"].([]any); ok {
		return w.walkOneOf(value, branches, path)
	}

	var violations []string
	if enum, ok := node["enum"].([]any); ok && len(enum) > 0 {
		if !enumContains(enum, value) {
			violations = append(violations, fmt.Sprintf("%s: %v outside enum %v", path, shorthand(value), enum))
		}
	}
	declared, typed := node["type"]
	if !typed {
		// No explicit type: only enum (and format-free) constraints apply.
		return violations
	}
	switch declared {
	case "object":
		violations = append(violations, w.walkObject(value, node, path)...)
	case "array":
		violations = append(violations, w.walkArray(value, node, path)...)
	case "string":
		text, ok := value.(string)
		if !ok {
			return append(violations, fmt.Sprintf("%s: want string, got %s", path, typeOf(value)))
		}
		if format, _ := node["format"].(string); format == "date-time" {
			if _, err := time.Parse(time.RFC3339, text); err != nil {
				violations = append(violations, fmt.Sprintf("%s: %q is not an RFC 3339 date-time", path, text))
			}
		}
	case "integer":
		if !isJSONInteger(value) {
			return append(violations, fmt.Sprintf("%s: want integer, got %s", path, typeOf(value)))
		}
		violations = append(violations, bounds(value.(float64), node, path)...)
	case "boolean":
		if _, ok := value.(bool); !ok {
			violations = append(violations, fmt.Sprintf("%s: want boolean, got %s", path, typeOf(value)))
		}
	case "number":
		if !isJSONNumber(value) {
			return append(violations, fmt.Sprintf("%s: want number, got %s", path, typeOf(value)))
		}
		violations = append(violations, bounds(value.(float64), node, path)...)
	default:
		// An unknown type name is a spec bug, not a wire bug — report it.
		violations = append(violations, fmt.Sprintf("%s: schema declares unknown type %q", path, declared))
	}
	return violations
}

// walkObject checks object shape: presence of every required property,
// refusal of unknown keys when the schema closes the object
// (additionalProperties: false — the wire discipline this API keeps), and
// recursion into every declared property the value carries.
func (w *Walker) walkObject(value any, node map[string]any, path string) []string {
	object, ok := value.(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("%s: want object, got %s", path, typeOf(value))}
	}
	var violations []string
	for _, required := range stringSlice(node["required"]) {
		if _, present := object[required]; !present {
			violations = append(violations, fmt.Sprintf("%s: required property %q missing", path, required))
		}
	}
	properties, _ := node["properties"].(map[string]any)
	var valueSchema any // set when additionalProperties is a schema, not false
	closed := false
	switch additional := node["additionalProperties"].(type) {
	case bool:
		closed = !additional
	case map[string]any:
		valueSchema = additional
	}
	for key, child := range object {
		if property, documented := properties[key]; documented {
			violations = append(violations, w.walk(child, property, path+"."+key)...)
			continue
		}
		if closed {
			violations = append(violations, fmt.Sprintf("%s.%s: not in schema (additionalProperties: false)", path, key))
			continue
		}
		if valueSchema != nil {
			violations = append(violations, w.walk(child, valueSchema, path+"."+key)...)
		}
	}
	return violations
}

// walkArray checks array shape and recurses into items.
func (w *Walker) walkArray(value any, node map[string]any, path string) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{fmt.Sprintf("%s: want array, got %s", path, typeOf(value))}
	}
	var violations []string
	if min, ok := node["minItems"].(int); ok && len(items) < min {
		violations = append(violations, fmt.Sprintf("%s: %d items, want at least %d", path, len(items), min))
	}
	itemSchema, hasItems := node["items"]
	if !hasItems {
		return violations
	}
	for i, item := range items {
		violations = append(violations, w.walk(item, itemSchema, fmt.Sprintf("%s[%d]", path, i))...)
	}
	return violations
}

// walkOneOf accepts a value that fully matches ANY branch — the union
// discipline with no partial credit: a branch with even one violation
// does not count, and only when every branch fails do the violations of
// the first branch surface (enough to name the disagreement without
// drowning the reader in every branch's version of it).
func (w *Walker) walkOneOf(value any, branches []any, path string) []string {
	var first []string
	for _, branch := range branches {
		violations := w.walk(value, branch, path)
		if len(violations) == 0 {
			return nil
		}
		if first == nil {
			first = violations
		}
	}
	if first == nil {
		first = []string{fmt.Sprintf("%s: oneOf has no branches", path)}
	}
	return first
}

// bounds checks numeric minimum/maximum (YAML decodes them as int; JSON
// numbers arrive as float64).
func bounds(number float64, node map[string]any, path string) []string {
	var violations []string
	if minimum, ok := node["minimum"].(int); ok && number < float64(minimum) {
		violations = append(violations, fmt.Sprintf("%s: %v below minimum %d", path, number, minimum))
	}
	if maximum, ok := node["maximum"].(int); ok && number > float64(maximum) {
		violations = append(violations, fmt.Sprintf("%s: %v above maximum %d", path, number, maximum))
	}
	return violations
}

// resolveRef resolves "#/components/schemas/Name" into the schema map.
func (w *Walker) resolveRef(ref string) (any, error) {
	const prefix = "#/components/schemas/"
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return nil, fmt.Errorf("unsupported $ref %q (only local schema refs resolve)", ref)
	}
	schema, ok := w.schemas[ref[len(prefix):]]
	if !ok {
		return nil, fmt.Errorf("$ref %q names an unknown schema", ref)
	}
	return schema, nil
}

// enumContains compares scalars with Go's == after normalizing the YAML
// side: YAML integers decode as int, JSON as float64 — compare by value
// across both.
func enumContains(enum []any, value any) bool {
	for _, candidate := range enum {
		if candidate == value {
			return true
		}
		if number, ok := value.(float64); ok && isWhole(number) {
			if asInt, ok := candidate.(int); ok && int(number) == asInt {
				return true
			}
		}
	}
	return false
}

// isWhole reports whether a JSON-decoded number is integral.
func isWhole(number float64) bool { return number == float64(int(number)) }

// isJSONInteger reports whether a decoded JSON value is an integral number.
func isJSONInteger(value any) bool {
	number, ok := value.(float64)
	return ok && isWhole(number)
}

// isJSONNumber reports whether the value decoded as a JSON number.
func isJSONNumber(value any) bool {
	_, ok := value.(float64)
	return ok
}

// typeOf names a decoded JSON value's kind for violation messages.
func typeOf(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// shorthand renders a scalar for violation messages without dumping whole
// documents into the failure text.
func shorthand(value any) string {
	text := fmt.Sprintf("%v", value)
	if len(text) > 40 {
		return text[:40] + "…"
	}
	return text
}

// stringSlice coerces a YAML list of strings.
func stringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
