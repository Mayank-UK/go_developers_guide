// Demo API handlers for topic 26 files-media — four thin handlers over
// the files domain: the streaming upload route (multipart parts copied
// straight to the store, never buffered), the download route
// (Content-Disposition plus RFC 7233 range serving), the thumbnail
// route (bounded decode, box downscale, re-encode), and the workbench
// story endpoint that runs the whole topic against a fresh store. The
// handlers hold only the Store port, the runner port, and a logger:
// parse and validate the edge, call the domain, shape the response.
package presentation

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"guide/internal/core"
	"guide/internal/domain/demos"
	domfiles "guide/internal/domain/files"
)

// Compile-time proofs the four handlers satisfy http.Handler.
var (
	_ http.Handler = (*FilesUploadHandler)(nil)
	_ http.Handler = (*FilesDownloadHandler)(nil)
	_ http.Handler = (*FilesThumbHandler)(nil)
	_ http.Handler = (*FilesWorkbenchHandler)(nil)
)

// filesUploadResponse is the upload route's success body: what landed,
// how big, and what the stored BYTES say it is (the content type is
// sniffed server-side, never trusted from the request).
type filesUploadResponse struct {
	Name        string `json:"name"`
	Bytes       int64  `json:"bytes"`
	ContentType string `json:"contentType"`
}

// FilesUploadHandler serves POST /api/demo/files/upload: the raw
// multipart body streams into the store part by part. The transport
// itself is bounded by http.MaxBytesReader (the object limit plus 1 MiB
// of framing headroom) so an oversized REQUEST dies at the edge, while
// the object limit is enforced mid-stream by the store.
type FilesUploadHandler struct {
	store  domfiles.Store
	limit  int64
	logger *slog.Logger
}

// NewFilesUploadHandler wires the upload route: the store port, the
// per-object byte limit, and the process logger.
func NewFilesUploadHandler(store domfiles.Store, limit int64, logger *slog.Logger) *FilesUploadHandler {
	return &FilesUploadHandler{store: store, limit: limit, logger: logger}
}

// ServeHTTP implements http.Handler for the streaming upload.
func (h *FilesUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.limit+(1<<20))
	object, err := domfiles.SaveMultipart(r.Context(), h.store, r.Header.Get("Content-Type"), r.Body, h.limit)
	if err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.Is(err, domfiles.ErrTooLarge):
			core.WriteError(w, r, core.ValidationFailed(
				"The upload exceeds the per-object size limit; the stream was refused mid-copy.",
				map[string]any{"limitBytes": h.limit}), h.logger)
		case errors.As(err, &tooLarge):
			core.WriteError(w, r, core.ValidationFailed(
				"The request body exceeds the transport bound; send the object in a smaller request.",
				map[string]any{"limitBytes": h.limit}), h.logger)
		case errors.Is(err, domfiles.ErrInvalidName):
			core.WriteError(w, r, core.ValidationFailed(
				"The upload's filename was refused: names are a single component — no separators, no '..', no absolute paths.",
				nil), h.logger)
		case errors.Is(err, domfiles.ErrNoFilePart):
			core.WriteError(w, r, core.ValidationFailed(
				"The multipart body carries no file part; send the object under a file field.",
				nil), h.logger)
		default:
			core.WriteError(w, r, core.Wrap(err, core.KindInternal,
				"The upload could not be streamed to the store."), h.logger)
		}
		return
	}
	writeJSON(w, http.StatusCreated, filesUploadResponse{
		Name:        object.Name,
		Bytes:       object.Size,
		ContentType: object.ContentType,
	})
}

// FilesDownloadHandler serves GET /api/demo/files/download/{name...}:
// the whole object with Content-Disposition, or — when the client sends
// a Range the validator allows — exactly the requested slice as 206,
// with 416 for ranges past the end.
type FilesDownloadHandler struct {
	store  domfiles.Store
	logger *slog.Logger
}

// NewFilesDownloadHandler wires the download route.
func NewFilesDownloadHandler(store domfiles.Store, logger *slog.Logger) *FilesDownloadHandler {
	return &FilesDownloadHandler{store: store, logger: logger}
}

// ServeHTTP implements http.Handler for downloads and ranges.
func (h *FilesDownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	object, content, err := h.store.Open(r.Context(), r.PathValue("name"))
	if err != nil {
		writeStoreError(w, r, err, h.logger)
		return
	}
	defer content.Close()

	etag := domfiles.ObjectETag(object)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", domfiles.ContentDisposition(object.Name))
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", object.ContentType)

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" &&
		domfiles.HonorsIfRange(r.Header.Get("If-Range"), etag, object.ModTime) {
		spec, unsatisfiable, present := domfiles.ParseRange(rangeHeader, object.Size)
		if present && unsatisfiable {
			w.Header().Set("Content-Range", domfiles.UnsatisfiableContentRange(object.Size))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if present {
			// Seek BEFORE the status is written: after WriteHeader a
			// failure could only be logged, not re-shaped.
			if _, err := content.Seek(spec.Start, io.SeekStart); err != nil {
				core.WriteError(w, r, core.Wrap(err, core.KindInternal,
					"The object could not be positioned for the range request."), h.logger)
				return
			}
			w.Header().Set("Content-Range", domfiles.ContentRange(spec, object.Size))
			w.Header().Set("Content-Length", strconv.FormatInt(spec.Length(), 10))
			w.WriteHeader(http.StatusPartialContent)
			if _, err := io.Copy(w, io.LimitReader(content, spec.Length())); err != nil {
				h.logger.Warn("files: range copy failed", slog.Any("error", err))
			}
			return
		}
		// Malformed or multi-range: fall through to the whole object.
	}

	w.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, content); err != nil {
		h.logger.Warn("files: download copy failed", slog.Any("error", err))
	}
}

// The thumbnail route's bounds: the query default, and the clamp that
// keeps a thumbnail a thumbnail.
const (
	thumbDefaultMax = 160
	thumbMinDim     = 16
	thumbMaxDim     = 1024
)

// FilesThumbHandler serves GET /api/demo/files/thumb/{name...}?max=N:
// the stored image decodes through the pixel budget, downscales by box
// sampling to the requested longest edge, and re-encodes in its source
// format. Thumbnails are small by construction, so encoding into a
// bounded buffer and writing once is honest — this is the one place in
// the topic where holding the artifact in memory is the point.
type FilesThumbHandler struct {
	store  domfiles.Store
	logger *slog.Logger
}

// NewFilesThumbHandler wires the thumbnail route.
func NewFilesThumbHandler(store domfiles.Store, logger *slog.Logger) *FilesThumbHandler {
	return &FilesThumbHandler{store: store, logger: logger}
}

// ServeHTTP implements http.Handler for the thumbnail transform.
func (h *FilesThumbHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	maxDim := thumbDefaultMax
	if raw := r.URL.Query().Get("max"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < thumbMinDim || parsed > thumbMaxDim {
			core.WriteError(w, r, core.ValidationFailed(
				"The thumbnail's max edge must be an integer between 16 and 1024.",
				map[string]any{"expected": "16..1024"}), h.logger)
			return
		}
		maxDim = parsed
	}
	object, content, err := h.store.Open(r.Context(), r.PathValue("name"))
	if err != nil {
		writeStoreError(w, r, err, h.logger)
		return
	}
	defer content.Close()

	image, format, err := domfiles.DecodeBounded(content)
	if err != nil {
		switch {
		case errors.Is(err, domfiles.ErrImageTooLarge):
			core.WriteError(w, r, core.ValidationFailed(
				"The image's declared dimensions exceed the decode budget; refusing before any pixel is allocated.",
				map[string]any{"maxPixels": domfiles.MaxPixels}), h.logger)
		default:
			core.WriteError(w, r, core.ValidationFailed(
				"The stored object is not a decodable png or jpeg image.", nil), h.logger)
		}
		return
	}
	thumb := domfiles.Thumbnail(image, maxDim)
	encoded := bytes.Buffer{}
	if err := domfiles.EncodeThumbnail(&encoded, thumb, format); err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The thumbnail could not be encoded."), h.logger)
		return
	}
	contentType := "image/png"
	if format == "jpeg" {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", domfiles.ContentDisposition("thumb-"+object.Name))
	w.Header().Set("Content-Length", strconv.Itoa(encoded.Len()))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(encoded.Bytes()); err != nil {
		h.logger.Warn("files: thumbnail write failed", slog.Any("error", err))
	}
}

// FilesWorkbenchHandler serves GET /api/demo/files/workbench: the
// topic's story, run fresh against a private store by the console.
type FilesWorkbenchHandler struct {
	runner demos.FilesWorkbenchRunner
	logger *slog.Logger
}

// NewFilesWorkbenchHandler wires the workbench story endpoint.
func NewFilesWorkbenchHandler(runner demos.FilesWorkbenchRunner, logger *slog.Logger) *FilesWorkbenchHandler {
	return &FilesWorkbenchHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the workbench story.
func (h *FilesWorkbenchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	run, err := h.runner.RunFilesWorkbenchStory(r.Context())
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The files workbench demo could not run its story against the store."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildFilesWorkbenchPayload(run))
}

// writeStoreError maps the domain's typed store errors to their honest
// envelopes: unknown objects are 404, refused names are 422, and
// anything else is an internal that gets logged with its request id.
func writeStoreError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	switch {
	case errors.Is(err, domfiles.ErrNotFound):
		core.WriteError(w, r, core.NotFound("No stored object under that name."), logger)
	case errors.Is(err, domfiles.ErrInvalidName):
		core.WriteError(w, r, core.ValidationFailed(
			"The object name was refused: names are a single component inside the store — no separators, no '..', no absolute paths, no symlinks out.",
			nil), logger)
	default:
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The file store could not serve that request."), logger)
	}
}
