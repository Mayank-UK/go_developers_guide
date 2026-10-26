package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// RequestIDHeader is the one header the request-id contract rides on,
// both inbound (honored when the client supplies one) and outbound
// (echoed on every response).
const RequestIDHeader = "x-request-id"

// MaxInboundRequestIDLength caps a client-supplied id so hostile headers
// cannot bloat every log line and response.
const MaxInboundRequestIDLength = 200

// requestIDKey is the unexported context key type: no other package can
// collide with or extract the value by accident.
type requestIDKey struct{}

// WithRequestID returns a context carrying the request id. The
// presentation middleware calls this once per request; WriteError and any
// handler read it back with RequestIDFromContext.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the ambient request id, or "" outside a
// request (or when the middleware did not run).
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// NewRequestID mints a fresh id: 16 random bytes, hex-encoded — the same
// shape as a uuid4 hex.
func NewRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand never fails on the supported platforms; if it ever
		// does, a constant id is worse than a panic-free fallback but
		// keeps the process serving. This branch is unreachable in
		// practice and exists only to satisfy the error contract.
		return "unavailable"
	}
	return hex.EncodeToString(buf[:])
}
