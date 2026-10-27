package obs

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// MetricsRegistry is a bounded-label metrics store that renders itself in
// the Prometheus text exposition format — the plain-text protocol every
// scraper speaks ("# HELP", "# TYPE", one sample per line). Counters and
// histograms are registered once at the composition root with FIXED label
// NAMES; the bounded-label discipline is that label VALUES come only from
// closed server-side vocabularies (route patterns and status classes, not
// raw paths; ids never), so the series count is bounded by the route
// table, not by traffic. Metric and label names are validated at
// registration — a malformed name is a programmer bug and panics loudly,
// before any request can be counted.
type MetricsRegistry struct {
	mu         sync.Mutex
	counters   map[string]*Counter
	histograms map[string]*Histogram
	names      []string
}

// NewMetricsRegistry builds an empty registry.
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		counters:   make(map[string]*Counter),
		histograms: make(map[string]*Histogram),
	}
}

// MustCounter registers (once) and returns a counter with the given name,
// help text, and label names. Duplicate or malformed names panic at
// registration time — wiring errors, not runtime ones.
func (r *MetricsRegistry) MustCounter(name, help string, labelNames []string) *Counter {
	counter := &Counter{
		name:       name,
		help:       help,
		labelNames: append([]string(nil), labelNames...),
		series:     make(map[string][]string),
		values:     make(map[string]uint64),
	}
	r.mustRegister(name, counter, nil)
	return counter
}

// MustHistogramSeconds registers (once) and returns a duration histogram
// with the given name, help, label names, and bucket upper bounds in
// ascending SECONDS (the exposition format's canonical unit; Prometheus
// convention spells it in the metric name suffix). Buckets are validated
// ascending and finite; the +Inf bucket is rendered but never stored.
func (r *MetricsRegistry) MustHistogramSeconds(name, help string, labelNames []string, buckets []float64) *Histogram {
	for i, bound := range buckets {
		if bound < 0 || bound != bound || bound > 1e18 {
			panic(fmt.Sprintf("metrics: histogram %q bucket %d (%v) is not a sane finite bound", name, i, bound))
		}
		if i > 0 && buckets[i-1] >= bound {
			panic(fmt.Sprintf("metrics: histogram %q buckets must ascend: %v", name, buckets))
		}
	}
	histogram := &Histogram{
		name:       name,
		help:       help,
		labelNames: append([]string(nil), labelNames...),
		buckets:    append([]float64(nil), buckets...),
		series:     make(map[string][]string),
		rows:       make(map[string]*histogramRow),
	}
	r.mustRegister(name, nil, histogram)
	return histogram
}

// mustRegister owns the registration invariants shared by both metric
// kinds: valid Prometheus name, no duplicate, deterministic render order.
func (r *MetricsRegistry) mustRegister(name string, counter *Counter, histogram *Histogram) {
	if !validMetricName(name) {
		panic(fmt.Sprintf("metrics: %q is not a valid Prometheus metric name", name))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, clash := r.counters[name]; clash {
		panic(fmt.Sprintf("metrics: counter %q registered twice", name))
	}
	if _, clash := r.histograms[name]; clash {
		panic(fmt.Sprintf("metrics: histogram %q registered twice", name))
	}
	r.names = append(r.names, name)
	if counter != nil {
		r.counters[name] = counter
	} else {
		r.histograms[name] = histogram
	}
}

// validMetricName matches the exposition format's name grammar:
// [a-zA-Z_:][a-zA-Z0-9_:]*.
func validMetricName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		ok := c == '_' || c == ':' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if i == 0 && c >= '0' && c <= '9' {
			return false
		}
		if !ok {
			return false
		}
	}
	return true
}

// Counter is a monotonically increasing metric split into series by its
// registered label names. Arity mismatches on Inc are programmer bugs
// (labels are server-chosen, never client-echoed) and panic loudly.
type Counter struct {
	name       string
	help       string
	labelNames []string

	mu     sync.Mutex
	series map[string][]string
	values map[string]uint64
}

// Inc increments the series selected by labelValues (one per registered
// label name, in registration order).
func (c *Counter) Inc(labelValues ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.checkedKey(labelValues)
	if _, seen := c.series[key]; !seen {
		c.series[key] = append([]string(nil), labelValues...)
	}
	c.values[key]++
}

// Value returns the current count of one series — the test surface; the
// wire surface is the registry's Write.
func (c *Counter) Value(labelValues ...string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.values[c.checkedKey(labelValues)]
}

// checkedKey validates arity and builds the series key.
func (c *Counter) checkedKey(labelValues []string) string {
	if len(labelValues) != len(c.labelNames) {
		panic(fmt.Sprintf("metrics: counter %q wants %d label value(s) (%v), got %d",
			c.name, len(c.labelNames), c.labelNames, len(labelValues)))
	}
	return strings.Join(labelValues, "\xff")
}

// Histogram is a cumulative distribution: observations fall into fixed
// upper-bound buckets (plus the implicit +Inf), with sum and count kept
// alongside. Rendered as the classic three-line _bucket/_sum/_count
// family of the exposition format.
type Histogram struct {
	name       string
	help       string
	labelNames []string
	buckets    []float64

	mu     sync.Mutex
	series map[string][]string
	rows   map[string]*histogramRow
}

// histogramRow is one labeled series' accumulations. Bucket counts are
// stored NON-cumulatively (the observation's bucket only) and rendered
// cumulatively, which is what the format demands and what makes deletes
// impossible to get wrong later.
type histogramRow struct {
	bucketCounts []uint64
	sum          float64
	count        uint64
}

// Observe records one value into the series selected by labelValues.
func (h *Histogram) Observe(value float64, labelValues ...string) {
	if len(labelValues) != len(h.labelNames) {
		panic(fmt.Sprintf("metrics: histogram %q wants %d label value(s) (%v), got %d",
			h.name, len(h.labelNames), h.labelNames, len(labelValues)))
	}
	key := strings.Join(labelValues, "\xff")
	h.mu.Lock()
	defer h.mu.Unlock()
	row, ok := h.rows[key]
	if !ok {
		row = &histogramRow{bucketCounts: make([]uint64, len(h.buckets))}
		h.rows[key] = row
		h.series[key] = append([]string(nil), labelValues...)
	}
	for i, bound := range h.buckets {
		if value <= bound {
			row.bucketCounts[i]++
			break
		}
	}
	row.sum += value
	row.count++
}

// ObservedCount returns one series' observation count — the test
// surface; the wire surface is the registry's Write.
func (h *Histogram) ObservedCount(labelValues ...string) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if row, ok := h.rows[strings.Join(labelValues, "\xff")]; ok {
		return row.count
	}
	return 0
}

// DefaultDurationBuckets is the canonical seconds bucket set the
// Prometheus client library ships: fine-grained where web requests live,
// coarse where pathological ones do, +Inf implied by the renderer.
func DefaultDurationBuckets() []float64 {
	return []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
}

// Write renders the whole registry in the Prometheus text exposition
// format, metric families in registration order and series sorted by
// label values so scrapes are byte-stable. The format is deliberately
// plain: HELP/TYPE comment lines, then one "name{label="value"} value"
// sample per line — the protocol this registry exists to prove is
// teachable without a client library.
func (r *MetricsRegistry) Write(w io.Writer) error {
	r.mu.Lock()
	names := append([]string(nil), r.names...)
	counters := make([]string, 0, len(r.counters))
	histograms := make([]string, 0, len(r.histograms))
	for name := range r.counters {
		counters = append(counters, name)
	}
	for name := range r.histograms {
		histograms = append(histograms, name)
	}
	r.mu.Unlock()

	var buf strings.Builder
	for _, name := range names {
		if counter, ok := r.counters[name]; ok {
			counter.write(&buf)
		} else if histogram, ok := r.histograms[name]; ok {
			histogram.write(&buf)
		}
	}
	_, err := io.WriteString(w, buf.String())
	return err
}

// write renders one counter family.
func (c *Counter) write(buf *strings.Builder) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fmt.Fprintf(buf, "# HELP %s %s\n", c.name, escapeHelp(c.help))
	fmt.Fprintf(buf, "# TYPE %s counter\n", c.name)
	for _, key := range sortedKeys(c.series) {
		values := c.series[key]
		fmt.Fprintf(buf, "%s%s %d\n", c.name, formatLabels(c.labelNames, values), c.values[key])
	}
}

// write renders one histogram family: cumulative _bucket lines with the
// le bound (+"+Inf"), then _sum and _count.
func (h *Histogram) write(buf *strings.Builder) {
	h.mu.Lock()
	defer h.mu.Unlock()

	fmt.Fprintf(buf, "# HELP %s %s\n", h.name, escapeHelp(h.help))
	fmt.Fprintf(buf, "# TYPE %s histogram\n", h.name)
	for _, key := range sortedKeys(h.series) {
		values := h.series[key]
		row := h.rows[key]
		cumulative := uint64(0)
		for i, bound := range h.buckets {
			cumulative += row.bucketCounts[i]
			fmt.Fprintf(buf, "%s_bucket%s %d\n",
				h.name, formatLabels(h.labelNames, values, "le", formatFloat(bound)), cumulative)
		}
		fmt.Fprintf(buf, "%s_bucket%s %d\n",
			h.name, formatLabels(h.labelNames, values, "le", "+Inf"), row.count)
		fmt.Fprintf(buf, "%s_sum%s %s\n", h.name, formatLabels(h.labelNames, values), formatFloat(row.sum))
		fmt.Fprintf(buf, "%s_count%s %d\n", h.name, formatLabels(h.labelNames, values), row.count)
	}
}

// sortedKeys renders series deterministically.
func sortedKeys(series map[string][]string) []string {
	keys := make([]string, 0, len(series))
	for key := range series {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// formatLabels renders the {k="v",…} matcher set, escaping the three
// characters the format reserves inside quoted values. Extra pairs ride
// along after the registered labels (the histogram's le bound).
func formatLabels(labelNames, values []string, extra ...string) string {
	if len(labelNames) == 0 && len(extra) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labelNames)+len(extra)/2)
	for i, name := range labelNames {
		parts = append(parts, name+`="`+escapeLabelValue(values[i])+`"`)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		parts = append(parts, extra[i]+`="`+escapeLabelValue(extra[i+1])+`"`)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// escapeLabelValue escapes backslash, double quote, and newline — the
// three escapes the exposition format defines.
func escapeLabelValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return replacer.Replace(value)
}

// escapeHelp escapes backslash and newline in HELP text (quotes are free
// there, but the others would break the line structure).
func escapeHelp(help string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`)
	return replacer.Replace(help)
}

// formatFloat renders a sample value without exponent noise where possible.
func formatFloat(v float64) string {
	// Fixed precision then trailing-zero trim: 2.054 stays "2.054", 3
	// becomes "3" (a bare integer, which the format accepts).
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.9f", v), "0"), ".")
}
