package files

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

// RFC 7233 range math as a table: every header shape a client can send,
// with the RFC's own verdicts — 206 slices, clamped over-asks, suffix
// ranges, ignored malformed headers, and honest 416s.

func TestParseRangeServesTheCanonicalShapes(t *testing.T) {
	const size = int64(1024)
	cases := []struct {
		header      string
		wantSpec    RangeSpec
		wantUnsat   bool
		wantPresent bool
	}{
		{"bytes=0-0", RangeSpec{0, 0}, false, true},
		{"bytes=0-99", RangeSpec{0, 99}, false, true},
		{"bytes=500-999", RangeSpec{500, 999}, false, true},
		{"bytes=500-", RangeSpec{500, 1023}, false, true},
		{"bytes=0-1023", RangeSpec{0, 1023}, false, true},
		{"bytes=0-9999", RangeSpec{0, 1023}, false, true},   // over-ask clamps
		{"bytes=1023-", RangeSpec{1023, 1023}, false, true}, // the last byte
		{"bytes=-1", RangeSpec{1023, 1023}, false, true},
		{"bytes=-64", RangeSpec{960, 1023}, false, true},
		{"bytes=-1024", RangeSpec{0, 1023}, false, true}, // whole object by suffix
		{"bytes=-4096", RangeSpec{0, 1023}, false, true}, // suffix longer than the object
	}
	for _, tc := range cases {
		spec, unsat, present := ParseRange(tc.header, size)
		if spec != tc.wantSpec || unsat != tc.wantUnsat || present != tc.wantPresent {
			t.Errorf("ParseRange(%q) = %+v, unsat=%t, present=%t; want %+v, %t, %t",
				tc.header, spec, unsat, present, tc.wantSpec, tc.wantUnsat, tc.wantPresent)
		}
		if present && !unsat && spec.End < spec.Start {
			t.Errorf("ParseRange(%q) produced End < Start", tc.header)
		}
		if present && !unsat && (spec.Start < 0 || spec.End >= size) {
			t.Errorf("ParseRange(%q) produced bytes outside the object", tc.header)
		}
	}
}

func TestParseRangeIgnoresMalformedHeaders(t *testing.T) {
	for _, header := range []string{
		"",                   // no header at all
		"chunks=0-99",        // wrong unit
		"bytes=",             // no directive
		"bytes=abc-def",      // not numbers
		"bytes=99-0",         // last < first: invalid per RFC, ignore
		"bytes=-12-34",       // two dashes
		"bytes=5",            // missing dash
		"bytes=0-99,200-299", // multi-range: answered in full, never half
	} {
		_, unsat, present := ParseRange(header, 1024)
		if present || unsat {
			t.Errorf("ParseRange(%q) = present=%t unsat=%t; want ignored (200 full body)", header, present, unsat)
		}
	}
}

func TestParseRangeRefusesTheUnsatisfiableWith416Semantics(t *testing.T) {
	for _, header := range []string{
		"bytes=1024-", // first byte is past the end
		"bytes=2000-", // far past the end
		"bytes=1024-1024",
		"bytes=5000-6000",
		"bytes=-0", // a suffix of zero bytes asks for nothing
	} {
		_, unsat, present := ParseRange(header, 1024)
		if !present || !unsat {
			t.Errorf("ParseRange(%q) = present=%t unsat=%t; want the 416 verdict", header, present, unsat)
		}
	}
}

func TestParseRangeOnAnEmptyObjectIsAlwaysUnsatisfiable(t *testing.T) {
	for _, header := range []string{"bytes=0-", "bytes=0-0", "bytes=-1", "bytes=-100"} {
		_, unsat, present := ParseRange(header, 0)
		if !present || !unsat {
			t.Errorf("ParseRange(%q, 0) = present=%t unsat=%t; want unsatisfiable", header, present, unsat)
		}
	}
}

func TestRangeLengthCountsInclusively(t *testing.T) {
	if got := (RangeSpec{Start: 0, End: 99}).Length(); got != 100 {
		t.Errorf("Length(0-99) = %d, want 100 (both ends inclusive)", got)
	}
}

func TestContentRangeRendersBothVerdicts(t *testing.T) {
	if got := ContentRange(RangeSpec{Start: 0, End: 99}, 1024); got != "bytes 0-99/1024" {
		t.Errorf("ContentRange = %q", got)
	}
	if got := UnsatisfiableContentRange(1024); got != "bytes */1024" {
		t.Errorf("UnsatisfiableContentRange = %q", got)
	}
}

func TestHonorsIfRangeGatesTheRangeOnValidators(t *testing.T) {
	modTime := time.Date(2026, 10, 27, 12, 0, 1, int(900*time.Millisecond), time.UTC)
	etag := `"abc123"`
	cases := []struct {
		name    string
		ifRange string
		want    bool
	}{
		{"no If-Range serves the range", "", true},
		{"an exact strong etag match serves the range", etag, true},
		{"a stale etag serves the whole object", `"other"`, false},
		{"a weak etag never authorizes a range", `W/"abc123"`, false},
		// HTTP-dates are IMF-fixdate in GMT (RFC 7231) — the only date
		// spellings http.ParseTime accepts on this Go line, matching
		// what real If-Range senders emit.
		{"an exact date serves the range", modTime.UTC().Format(http.TimeFormat), true},
		{"a stale date serves the whole object", modTime.Add(-time.Hour).UTC().Format(http.TimeFormat), false},
		{"an unparseable value serves the whole object", "not a validator", false},
	}
	for _, tc := range cases {
		if got := HonorsIfRange(tc.ifRange, etag, modTime); got != tc.want {
			t.Errorf("%s: HonorsIfRange(%q) = %t", tc.name, tc.ifRange, got)
		}
	}
}

func TestObjectETagIsStrongAndOpaque(t *testing.T) {
	first := Object{Name: "a", Size: 10, ModTime: time.Unix(1, 2).UTC()}
	second := Object{Name: "a", Size: 10, ModTime: time.Unix(1, 3).UTC()}
	etag := ObjectETag(first)
	if etag == "" || etag[0] != '"' || etag[len(etag)-1] != '"' {
		t.Errorf("etag %q must be a quoted strong validator", etag)
	}
	if etag == ObjectETag(second) {
		t.Error("a changed ModTime must change the etag")
	}
	if !errors.Is(ErrRangeUnsatisfiable, ErrRangeUnsatisfiable) {
		t.Error("sentinel must stay errors.Is-able")
	}
}
