package obs

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The metrics registry's tests pin the three properties the observability
// topic teaches: the text-exposition output shape, deterministic sorted
// rendering, and the bounded-label discipline (raw hostile values can
// never reach a label — the middleware test proves that side; here the
// escaping proves the renderer cannot be broken by what it is handed).

func TestCounterRendersTheTextExpositionFormat(t *testing.T) {
	reg := NewMetricsRegistry()
	requests := reg.MustCounter("guide_http_requests_total",
		"HTTP requests processed, labeled by method, route pattern, and status class.",
		[]string{"method", "path", "status"})
	requests.Inc("GET", "/api/content", "2xx")
	requests.Inc("GET", "/api/content", "2xx")
	requests.Inc("GET", "/api/posts/{id}", "4xx")

	var out strings.Builder
	if err := reg.Write(&out); err != nil {
		t.Fatalf("write: %v", err)
	}
	want := strings.Join([]string{
		`# HELP guide_http_requests_total HTTP requests processed, labeled by method, route pattern, and status class.`,
		`# TYPE guide_http_requests_total counter`,
		`guide_http_requests_total{method="GET",path="/api/content",status="2xx"} 2`,
		`guide_http_requests_total{method="GET",path="/api/posts/{id}",status="4xx"} 1`,
		``,
	}, "\n")
	if out.String() != want {
		t.Errorf("exposition =\n%s\nwant\n%s", out.String(), want)
	}
	if got := requests.Value("GET", "/api/content", "2xx"); got != 2 {
		t.Errorf("Value = %d, want 2", got)
	}
}

func TestHistogramRendersBucketsSumAndCount(t *testing.T) {
	reg := NewMetricsRegistry()
	durations := reg.MustHistogramSeconds("guide_http_request_duration_seconds",
		"Request durations in seconds, labeled by method and route pattern.",
		[]string{"method"}, []float64{0.01, 0.1, 1})
	durations.Observe(0.004, "GET")
	durations.Observe(0.05, "GET")
	durations.Observe(2, "GET")

	var out strings.Builder
	if err := reg.Write(&out); err != nil {
		t.Fatalf("write: %v", err)
	}
	body := out.String()
	for _, want := range []string{
		`# TYPE guide_http_request_duration_seconds histogram`,
		`guide_http_request_duration_seconds_bucket{method="GET",le="0.01"} 1`,
		`guide_http_request_duration_seconds_bucket{method="GET",le="0.1"} 2`,
		`guide_http_request_duration_seconds_bucket{method="GET",le="1"} 2`,
		`guide_http_request_duration_seconds_bucket{method="GET",le="+Inf"} 3`,
		`guide_http_request_duration_seconds_count{method="GET"} 3`,
	} {
		if !strings.Contains(body, want+"\n") {
			t.Errorf("exposition missing line %q:\n%s", want, body)
		}
	}
	if !strings.Contains(body, `guide_http_request_duration_seconds_sum{method="GET"} 2.054`) {
		t.Errorf("sum line drifted:\n%s", body)
	}
}

func TestRegistryRendersFamiliesInRegistrationOrderAndSeriesSorted(t *testing.T) {
	reg := NewMetricsRegistry()
	letters := reg.MustCounter("guide_a_total", "A.", []string{"k"})
	other := reg.MustCounter("guide_b_total", "B.", []string{"k"})
	letters.Inc("zeta")
	letters.Inc("alpha")
	letters.Inc("mid")
	other.Inc("one")

	var out strings.Builder
	if err := reg.Write(&out); err != nil {
		t.Fatalf("write: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	var samples []string
	for _, line := range lines {
		if strings.HasPrefix(line, "guide_") {
			samples = append(samples, line)
		}
	}
	got := make([]string, 0, len(samples))
	for _, sample := range samples {
		got = append(got, strings.SplitN(sample, "{", 2)[0]+strings.SplitN(sample, "{", 2)[1][:5])
	}
	sorted := append([]string(nil), got...)
	sort.Strings(sorted)
	for i := range got {
		if got[i] != sorted[i] {
			t.Errorf("series not rendered sorted: %v", samples)
			break
		}
	}
	if strings.Index(out.String(), "guide_a_total{") > strings.Index(out.String(), "guide_b_total{") {
		t.Error("families must render in registration order")
	}
}

func TestLabelAndHelpEscaping(t *testing.T) {
	reg := NewMetricsRegistry()
	weird := reg.MustCounter("guide_weird_total", `back\slash and
newline`, []string{"path"})
	weird.Inc(`/a"b\c
d`)
	var out strings.Builder
	if err := reg.Write(&out); err != nil {
		t.Fatalf("write: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, `# HELP guide_weird_total back\\slash and\nnewline`) {
		t.Errorf("help escaping drifted:\n%s", body)
	}
	if !strings.Contains(body, `path="/a\"b\\c\nd"`) {
		t.Errorf("label escaping drifted:\n%s", body)
	}
}

func TestRegistryRejectsBadRegistration(t *testing.T) {
	reg := NewMetricsRegistry()
	reg.MustCounter("guide_ok_total", "ok.", nil)
	dup := func() { reg.MustCounter("guide_ok_total", "again.", nil) }
	if !panics(dup) {
		t.Error("duplicate registration must panic")
	}
	badName := func() { reg.MustCounter("1-bad", "no.", nil) }
	if !panics(badName) {
		t.Error("malformed metric name must panic")
	}
	badBuckets := func() { reg.MustHistogramSeconds("guide_h", "h.", nil, []float64{1, 0.5}) }
	if !panics(badBuckets) {
		t.Error("non-ascending buckets must panic")
	}
	badArity := func() {
		counter := reg.MustCounter("guide_arity_total", "a.", []string{"one"})
		counter.Inc("a", "b")
	}
	if !panics(badArity) {
		t.Error("label arity mismatch must panic")
	}
}

func TestConcurrentIncrementsAreRaceCleanAndExact(t *testing.T) {
	reg := NewMetricsRegistry()
	counter := reg.MustCounter("guide_race_total", "race.", []string{"w"})
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				counter.Inc(fmt.Sprintf("w%d", worker))
			}
		}(worker)
	}
	wg.Wait()
	for worker := 0; worker < 8; worker++ {
		if got := counter.Value(fmt.Sprintf("w%d", worker)); got != 100 {
			t.Errorf("worker %d counted %d, want 100 (lost update)", worker, got)
		}
	}
}

func panics(fn func()) (caught bool) {
	defer func() {
		if recover() != nil {
			caught = true
		}
	}()
	fn()
	return false
}
