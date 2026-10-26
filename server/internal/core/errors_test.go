package core

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestKindStatusMapping(t *testing.T) {
	cases := []struct {
		kind   Kind
		status int
	}{
		{KindValidationFailed, http.StatusUnprocessableEntity},
		{KindUnauthorized, http.StatusUnauthorized},
		{KindForbidden, http.StatusForbidden},
		{KindNotFound, http.StatusNotFound},
		{KindConflict, http.StatusConflict},
		{KindRateLimited, http.StatusTooManyRequests},
		{KindServiceUnavailable, http.StatusServiceUnavailable},
		{KindInternal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if got := HTTPStatusFor(tc.kind); got != tc.status {
			t.Errorf("HTTPStatusFor(%s) = %d, want %d", tc.kind, got, tc.status)
		}
		if got := KindForHTTPStatus(tc.status); got != tc.kind {
			t.Errorf("KindForHTTPStatus(%d) = %s, want %s", tc.status, got, tc.kind)
		}
	}
}

func TestKindForUnnamedStatuses(t *testing.T) {
	// Unnamed 4xx → not_found (closest client-error kind, e.g. 405);
	// unnamed 5xx → internal. The truthful status is kept by the caller.
	if got := KindForHTTPStatus(http.StatusMethodNotAllowed); got != KindNotFound {
		t.Errorf("KindForHTTPStatus(405) = %s, want not_found", got)
	}
	if got := KindForHTTPStatus(http.StatusTeapot); got != KindNotFound {
		t.Errorf("KindForHTTPStatus(418) = %s, want not_found", got)
	}
	if got := KindForHTTPStatus(http.StatusNotImplemented); got != KindInternal {
		t.Errorf("KindForHTTPStatus(501) = %s, want internal", got)
	}
}

func TestHTTPStatusForUnknownKindDegradesTo500(t *testing.T) {
	if got := HTTPStatusFor(Kind("made_up")); got != http.StatusInternalServerError {
		t.Errorf("HTTPStatusFor(unknown) = %d, want 500", got)
	}
}

func TestErrorStringAndUnwrap(t *testing.T) {
	cause := errors.New("connection refused")
	wrapped := Wrap(cause, KindInternal, "store unavailable")
	if got, want := wrapped.Error(), "[internal] store unavailable"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(wrapped, cause) {
		t.Error("wrapped error must expose its cause to errors.Is")
	}
	// %w chaining through fmt preserves the taxonomy for errors.As.
	chained := fmt.Errorf("demo call: %w", Newf(KindNotFound, "topic %q not found", "x"))
	var appErr *Error
	if !errors.As(chained, &appErr) || appErr.Kind != KindNotFound {
		t.Errorf("errors.As through fmt %%w = %+v, want a not_found *Error", chained)
	}
}

func TestSentinelKindMatching(t *testing.T) {
	if !errors.Is(NotFound("nope"), ErrNotFound) {
		t.Error("NotFound must match the ErrNotFound sentinel by kind")
	}
	if !errors.Is(Wrap(errors.New("boom"), KindValidationFailed, "bad input"), ErrValidationFailed) {
		t.Error("wrapped errors must match their kind's sentinel")
	}
	if errors.Is(NotFound("nope"), ErrConflict) {
		t.Error("a not_found error must not match the conflict sentinel")
	}
	if errors.Is(NotFound("nope"), errors.New("unrelated")) {
		t.Error("a typed error must not match an unrelated plain error")
	}
}

func TestConstructorsBuildRightKinds(t *testing.T) {
	cases := []struct {
		err  *Error
		kind Kind
	}{
		{ValidationFailed("bad", map[string]string{"field": "dataMode"}), KindValidationFailed},
		{NotFound("gone"), KindNotFound},
		{Unauthorized("who?"), KindUnauthorized},
		{Forbidden("no"), KindForbidden},
		{Conflict("clash"), KindConflict},
		{RateLimited("slow down"), KindRateLimited},
		{ServiceUnavailable("later"), KindServiceUnavailable},
		{Internal("oops"), KindInternal},
		{Newf(KindConflict, "version %d", 7), KindConflict},
	}
	for _, tc := range cases {
		if tc.err.Kind != tc.kind {
			t.Errorf("%v: kind = %s, want %s", tc.err, tc.err.Kind, tc.kind)
		}
	}
	withDetails := ValidationFailed("bad", []string{"dataMode"})
	if withDetails.Details == nil {
		t.Error("ValidationFailed must carry the details it was given")
	}
}
