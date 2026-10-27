// Range-request math as pure functions (RFC 7233): ParseRange turns one
// Range header into either a satisfiable inclusive slice, an explicit
// unsatisfiable verdict, or "ignore the header" — the three answers a
// download route owes. Formatting helpers render Content-Range both
// ways (the served slice and the 416 census), and If-Range decides
// whether a range may apply at all. No I/O anywhere: the same functions
// answer for the HTTP route and for the demo story's proofs.
package files

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrRangeUnsatisfiable reports a syntactically valid Range whose bytes
// do not exist in the object (first-byte-pos at or past the end, or a
// zero-byte suffix). The honest answer is 416 with a Content-Range of
// "bytes */size" — not 404, and not a silent 200.
var ErrRangeUnsatisfiable = errors.New("files: range not satisfiable")

// RangeSpec is one inclusive byte slice: Start and End both name bytes
// that exist in the object, End >= Start, and Length is End-Start+1.
type RangeSpec struct {
	Start int64
	End   int64
}

// Length is the slice's byte count.
func (s RangeSpec) Length() int64 { return s.End - s.Start + 1 }

// ParseRange reads one Range header against an object of the given
// size. Three outcomes, all honest:
//
//	present=false             — no usable range: serve the whole object
//	                             (no header, malformed syntax, or a
//	                             multi-range request this store
//	                             deliberately answers in full)
//	present, unsatisfiable    — valid syntax, absent bytes: serve 416
//	present, satisfiable spec — serve 206 with exactly spec's slice
//
// Per RFC 7233 a malformed Range MUST be ignored (never refused), and
// last-byte-pos < first-byte-pos is malformed; over-asking past the end
// is legal and clamped.
func ParseRange(header string, size int64) (spec RangeSpec, unsatisfiable bool, present bool) {
	const unit = "bytes="
	if !strings.HasPrefix(header, unit) {
		return RangeSpec{}, false, false
	}
	directive := strings.TrimPrefix(header, unit)
	if strings.Contains(directive, ",") {
		// Multi-range: refused as a range, honored as a whole-object
		// request — never silently answered with the FIRST range.
		return RangeSpec{}, false, false
	}
	startText, endText, hasDash := strings.Cut(directive, "-")
	if !hasDash {
		return RangeSpec{}, false, false
	}

	// "bytes=-N": the LAST N bytes. N >= size means the whole object;
	// N == 0 asks for nothing that exists.
	if startText == "" {
		suffix, err := strconv.ParseInt(endText, 10, 64)
		if err != nil || suffix < 0 {
			return RangeSpec{}, false, false
		}
		if suffix == 0 {
			return RangeSpec{}, true, true
		}
		if size == 0 {
			return RangeSpec{}, true, true
		}
		if suffix >= size {
			return RangeSpec{Start: 0, End: size - 1}, false, true
		}
		return RangeSpec{Start: size - suffix, End: size - 1}, false, true
	}

	start, err := strconv.ParseInt(startText, 10, 64)
	if err != nil || start < 0 {
		return RangeSpec{}, false, false
	}
	var end int64
	if endText == "" {
		end = size - 1 // open-ended: through the last byte
	} else {
		end, err = strconv.ParseInt(endText, 10, 64)
		if err != nil || end < 0 {
			return RangeSpec{}, false, false
		}
		if end < start {
			// last-byte-pos < first-byte-pos is an invalid range the
			// RFC says to ignore, not to refuse.
			return RangeSpec{}, false, false
		}
	}
	if size == 0 || start >= size {
		return RangeSpec{}, true, true
	}
	if end >= size {
		end = size - 1 // a client may legitimately over-ask; clamp
	}
	return RangeSpec{Start: start, End: end}, false, true
}

// ContentRange renders the 206 response's Content-Range for a served
// slice: "bytes 0-99/1024".
func ContentRange(spec RangeSpec, size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", spec.Start, spec.End, size)
}

// UnsatisfiableContentRange renders the 416's census: "bytes */1024" —
// the only Content-Range a 416 may carry, telling the client the real
// size so it can re-ask sensibly.
func UnsatisfiableContentRange(size int64) string {
	return fmt.Sprintf("bytes */%d", size)
}

// ObjectETag mints the strong validator If-Range and Range clients
// compare against: size and modification time folded into an opaque
// quoted string. Weak validators (W/) never match — If-Range demands
// strong comparison so a changed object can never serve a stale slice.
func ObjectETag(obj Object) string {
	return fmt.Sprintf(`"%x-%x"`, obj.Size, obj.ModTime.UnixNano())
}

// HonorsIfRange decides whether a Range header may shape the response
// given an If-Range. No If-Range: always honored. An exact ETag match:
// honored. An exact HTTP-date match (to the second): honored. Anything
// else — a stale validator, a weak tag, an unparseable value — means
// the object may have changed since the client saw it, so the whole
// object is served and the range ignored, per RFC 7233.
func HonorsIfRange(ifRange, etag string, modTime time.Time) bool {
	if ifRange == "" {
		return true
	}
	if strings.HasPrefix(ifRange, "W/") {
		return false // weak validators never authorize a range
	}
	if ifRange == etag {
		return true
	}
	if when, err := http.ParseTime(ifRange); err == nil {
		return when.Equal(modTime.Truncate(time.Second))
	}
	return false
}
