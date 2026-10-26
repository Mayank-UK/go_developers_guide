package core

import (
	"fmt"
	"net/http"
)

// Kind is the failure bucket every error in the app maps onto — the eight
// kinds the family guides share. Wire value is the snake_case code the
// error envelope carries.
type Kind string

// The eight failure buckets. Nothing outside this taxonomy invents status
// codes or error codes.
const (
	KindValidationFailed   Kind = "validation_failed"   // 422 (family convention; the express sibling uses 400)
	KindUnauthorized       Kind = "unauthorized"        // 401
	KindForbidden          Kind = "forbidden"           // 403
	KindNotFound           Kind = "not_found"           // 404
	KindConflict           Kind = "conflict"            // 409
	KindRateLimited        Kind = "rate_limited"        // 429
	KindServiceUnavailable Kind = "service_unavailable" // 503
	KindInternal           Kind = "internal"            // 500
)

// knownKinds is the closed set — an Error carrying anything else is a
// programmer error and degrades to the internal bucket.
var knownKinds = map[Kind]bool{
	KindValidationFailed:   true,
	KindUnauthorized:       true,
	KindForbidden:          true,
	KindNotFound:           true,
	KindConflict:           true,
	KindRateLimited:        true,
	KindServiceUnavailable: true,
	KindInternal:           true,
}

var httpStatusByKind = map[Kind]int{
	KindValidationFailed:   http.StatusUnprocessableEntity,
	KindUnauthorized:       http.StatusUnauthorized,
	KindForbidden:          http.StatusForbidden,
	KindNotFound:           http.StatusNotFound,
	KindConflict:           http.StatusConflict,
	KindRateLimited:        http.StatusTooManyRequests,
	KindServiceUnavailable: http.StatusServiceUnavailable,
	KindInternal:           http.StatusInternalServerError,
}

var kindByHTTPStatus = func() map[int]Kind {
	byStatus := make(map[int]Kind, len(httpStatusByKind))
	for kind, status := range httpStatusByKind {
		byStatus[status] = kind
	}
	return byStatus
}()

// HTTPStatusFor maps a failure kind to its HTTP status. Unknown kinds take
// 500 — the same degradation WriteError applies.
func HTTPStatusFor(kind Kind) int {
	if status, ok := httpStatusByKind[kind]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// KindForHTTPStatus maps an HTTP status to the closest failure kind:
// statuses the taxonomy names map exactly; unnamed 4xx statuses take the
// not_found bucket (closest client-error kind — e.g. 405 method not
// allowed) and unnamed 5xx statuses take internal. The response keeps the
// truthful status either way.
func KindForHTTPStatus(status int) Kind {
	if kind, ok := kindByHTTPStatus[status]; ok {
		return kind
	}
	if status < 500 {
		return KindNotFound
	}
	return KindInternal
}

// Error is the typed application error: kind + message + optional details
// + optional wrapped cause. Domain and data layers return (never panic
// with) *Error; WriteError translates it into the one HTTP error envelope.
// Details carries structured context for clients (e.g. which fields failed
// validation). The cause is preserved through Unwrap, so errors.Is/As and
// fmt's %w chaining work through the whole stack.
type Error struct {
	Kind    Kind
	Message string
	Details any
	cause   error
}

// Error implements the error interface: "[kind] message".
func (e *Error) Error() string {
	return fmt.Sprintf("[%s] %s", e.Kind, e.Message)
}

// Unwrap returns the wrapped cause, if any (%w-compatible).
func (e *Error) Unwrap() error { return e.cause }

// Is makes errors.Is compare by kind: any error whose chain holds an *Error
// matches the sentinel for its kind, so callers can test
// errors.Is(err, core.ErrNotFound) without knowing where the error was
// wrapped.
func (e *Error) Is(target error) bool {
	targetErr, ok := target.(*Error)
	return ok && targetErr.Kind == e.Kind
}

// Package-level kind sentinels for errors.Is comparisons. Their messages
// are never served — the concrete error's message is.
var (
	ErrValidationFailed   = &Error{Kind: KindValidationFailed, Message: "Validation failed."}
	ErrUnauthorized       = &Error{Kind: KindUnauthorized, Message: "Authentication required."}
	ErrForbidden          = &Error{Kind: KindForbidden, Message: "Access forbidden."}
	ErrNotFound           = &Error{Kind: KindNotFound, Message: "Resource not found."}
	ErrConflict           = &Error{Kind: KindConflict, Message: "Resource conflict."}
	ErrRateLimited        = &Error{Kind: KindRateLimited, Message: "Too many requests."}
	ErrServiceUnavailable = &Error{Kind: KindServiceUnavailable, Message: "Service unavailable."}
	ErrInternal           = &Error{Kind: KindInternal, Message: "Internal server error."}
)

// New builds a typed error with an explicit kind.
func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

// Newf builds a typed error with a formatted message.
func Newf(kind Kind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// Wrap builds a typed error that carries cause (%w-style) for the
// error's own bookkeeping while presenting message to callers.
func Wrap(cause error, kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message, cause: cause}
}

// Wrapf is Wrap with a formatted message.
func Wrapf(cause error, kind Kind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...), cause: cause}
}

// ValidationFailed builds the 422 kind with structured details naming what
// failed (details may be nil).
func ValidationFailed(message string, details any) *Error {
	return &Error{Kind: KindValidationFailed, Message: message, Details: details}
}

// NotFound builds the 404 kind.
func NotFound(message string) *Error { return &Error{Kind: KindNotFound, Message: message} }

// Unauthorized builds the 401 kind.
func Unauthorized(message string) *Error { return &Error{Kind: KindUnauthorized, Message: message} }

// Forbidden builds the 403 kind.
func Forbidden(message string) *Error { return &Error{Kind: KindForbidden, Message: message} }

// Conflict builds the 409 kind.
func Conflict(message string) *Error { return &Error{Kind: KindConflict, Message: message} }

// RateLimited builds the 429 kind.
func RateLimited(message string) *Error { return &Error{Kind: KindRateLimited, Message: message} }

// ServiceUnavailable builds the 503 kind.
func ServiceUnavailable(message string) *Error {
	return &Error{Kind: KindServiceUnavailable, Message: message}
}

// Internal builds the 500 kind.
func Internal(message string) *Error { return &Error{Kind: KindInternal, Message: message} }
