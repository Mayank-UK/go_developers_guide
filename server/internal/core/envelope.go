package core

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// genericInternalMessage is the only body a client sees for an unexpected
// failure — internals never leak.
const genericInternalMessage = "Internal server error."

// errorBody is the "error" object of the envelope; wire keys are camelCase
// and match the client's error pipeline (code, message, requestId, then
// optional details). Details is omitted from the wire when absent, never
// null.
type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
	Details   any    `json:"details,omitempty"`
}

// errorEnvelope is the whole error body: {"error": {...}}.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// WriteError is the one central error-response writer: it maps err onto
// the family envelope {"error":{code,message,requestId,details?}} with the
// kind's HTTP status and the x-request-id header stamped from the request
// context.
//
// A *Error with a known kind gets its own status, code, message, and
// details, logged at warn. Anything else — plain errors, nil-kind errors,
// or an Error with an unrecognized kind — is a taxonomy miss: the response
// is a generic 500 internal while the underlying error is logged at error
// with the request id, so nothing is ever swallowed.
func WriteError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	requestID := RequestIDFromContext(r.Context())

	var appErr *Error
	if errors.As(err, &appErr) && knownKinds[appErr.Kind] {
		status := HTTPStatusFor(appErr.Kind)
		if logger != nil {
			logger.Warn("error response",
				"kind", string(appErr.Kind),
				"message", appErr.Message,
				"status", status,
				"request_id", requestID,
				"error", err.Error(),
			)
		}
		writeEnvelope(w, status, appErr.Kind, appErr.Message, requestID, appErr.Details)
		return
	}

	if logger != nil {
		logger.Error("unhandled error",
			"request_id", requestID,
			"error", err.Error(),
		)
	}
	writeEnvelope(w, http.StatusInternalServerError, KindInternal, genericInternalMessage, requestID, nil)
}

// WriteErrorStatus writes the envelope with an explicit HTTP status,
// taking only the code bucket from err's kind. It exists for statuses the
// taxonomy does not name — the family rule keeps the truthful status and
// takes the closest kind bucket (e.g. a 405 method mismatch is a 405
// response carrying the not_found code).
func WriteErrorStatus(w http.ResponseWriter, r *http.Request, status int, err *Error, logger *slog.Logger) {
	requestID := RequestIDFromContext(r.Context())
	if err == nil || !knownKinds[err.Kind] {
		err = Internal(genericInternalMessage)
		status = http.StatusInternalServerError
	}
	if logger != nil {
		logger.Warn("error response",
			"kind", string(err.Kind),
			"message", err.Message,
			"status", status,
			"request_id", requestID,
			"error", err.Error(),
		)
	}
	writeEnvelope(w, status, err.Kind, err.Message, requestID, err.Details)
}

// writeEnvelope stamps the shared headers and writes the JSON envelope.
func writeEnvelope(w http.ResponseWriter, status int, kind Kind, message, requestID string, details any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(RequestIDHeader, requestID)
	w.WriteHeader(status)
	body := errorEnvelope{Error: errorBody{
		Code:      string(kind),
		Message:   message,
		RequestID: requestID,
		Details:   details,
	}}
	// An Encode failure means the peer vanished mid-write; there is no
	// recovery action, and the status line already went out.
	_ = json.NewEncoder(w).Encode(body)
}
