package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"guide/internal/content"
	"guide/internal/core"
)

// Content API handlers — thin by family invariant: resolve the injected
// registry, shape the response, done. The registry was validated once at
// startup (main.go fails fast otherwise); an unknown topic id is the 404
// envelope via the central error writer. No business logic lives here.

// ContentListHandler serves GET /api/content: the whole topic index —
// groups and topic summaries, each ordered — plus the content API version.
type ContentListHandler struct {
	registry content.Registry
}

// NewContentListHandler wires the validated content registry.
func NewContentListHandler(registry content.Registry) *ContentListHandler {
	return &ContentListHandler{registry: registry}
}

// ServeHTTP implements http.Handler for the content index.
func (h *ContentListHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, content.ContentListResponse{
		Version: content.ContentAPIVersion,
		Groups:  h.registry.Groups(),
		Topics:  h.registry.Summaries(),
	})
}

// ContentTopicHandler serves GET /api/content/{topicId}: one topic's full
// payload (sections and blocks) plus the content API version.
type ContentTopicHandler struct {
	registry content.Registry
	logger   *slog.Logger
}

// NewContentTopicHandler wires the validated content registry and the
// process logger (the central error writer logs with it).
func NewContentTopicHandler(registry content.Registry, logger *slog.Logger) *ContentTopicHandler {
	return &ContentTopicHandler{registry: registry, logger: logger}
}

// ServeHTTP implements http.Handler for one topic's detail.
func (h *ContentTopicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	topicID := r.PathValue("topicId")
	topic, ok := h.registry.TopicByID(topicID)
	if !ok {
		core.WriteError(w, r, core.Newf(core.KindNotFound, "Topic %q not found", topicID), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, content.ContentDetail{
		Version: content.ContentAPIVersion,
		Topic:   topic,
	})
}

// writeJSON is the handlers' shared success writer: one Content-Type, one
// status, one encoded body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// An Encode failure means the peer vanished mid-write; there is no
	// recovery action, and the status line already went out.
	_ = json.NewEncoder(w).Encode(v)
}
