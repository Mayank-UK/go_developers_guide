// Multipart uploads, streamed: multipart.NewReader over the raw body —
// NOT ParseMultipartForm, which buffers the whole request (to memory or
// temp files) before handing you anything. Reading parts as they arrive
// means an 8 MiB upload costs kilobytes of resident memory, the store's
// limit refuses mid-stream, and backpressure from the disk slows the
// socket through the pipeline's bounded buffers. This orchestration is
// domain logic — it composes the Store port and never touches a
// filesystem or an *http.Request — so both the HTTP route and the demo
// story run the same code.
package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
)

// ErrNoFilePart reports a multipart body that never carried a file part
// — a validation failure, not an internal one.
var ErrNoFilePart = errors.New("files: multipart body carries no file part")

// SaveMultipart streams the FIRST file part of a multipart body into
// the store. contentType is the request's Content-Type (the boundary
// lives there); body is the raw stream. Non-file parts are drained and
// skipped — a form's text fields are not this route's business, and
// draining keeps the part reader in sync. The filename arrives from the
// client and goes straight into the sanitizer: SaveStream refuses it if
// it carries any structure at all.
func SaveMultipart(ctx context.Context, store Store, contentType string, body io.Reader, limit int64) (Object, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return Object{}, fmt.Errorf("files: parse multipart content type: %w", err)
	}
	if mediaType != "multipart/form-data" || params["boundary"] == "" {
		return Object{}, fmt.Errorf("%w: content type %q is not multipart/form-data with a boundary",
			ErrNoFilePart, contentType)
	}

	reader := multipart.NewReader(body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return Object{}, ErrNoFilePart
		}
		if err != nil {
			return Object{}, fmt.Errorf("files: read multipart part: %w", err)
		}
		if part.FileName() == "" {
			// A non-file field: drain it and keep looking for the file.
			// The drain is bounded by the same limit as the upload — the
			// part's framing is structure, not a promise, and a text
			// field larger than the object limit is hostile input.
			skipped, err := io.Copy(io.Discard, io.LimitReader(part, limit+1))
			if err != nil {
				part.Close()
				return Object{}, fmt.Errorf("files: skip non-file part: %w", err)
			}
			part.Close()
			if skipped > limit {
				return Object{}, fmt.Errorf("%w: a non-file field of %d bytes against a %d-byte limit",
					ErrTooLarge, skipped, limit)
			}
			continue
		}
		object, err := store.SaveStream(ctx, part.FileName(), part, limit)
		part.Close()
		return object, err
	}
}
