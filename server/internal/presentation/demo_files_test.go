package presentation

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/data/files"
	"guide/internal/domain/demos"
)

// The topic 26 endpoints over the REAL local store (the handlers hold
// only the Store port; the adapter is the honest way to exercise the
// full download and thumbnail paths). The workbench story runner is
// faked here — the REAL console story has its own suite in
// internal/data/files — and every response shape is pinned: the upload
// answer with its sniffed content type, the download headers, the exact
// 206 slice, the 416 census, the typed envelopes for refused names.

// ── fixtures ─────────────────────────────────────────────────────────────

// solidPNG encodes a width×height solid-color PNG the thumb route can
// decode — deterministic bytes, real codec.
func solidPNG(t *testing.T, width, height int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	out := bytes.Buffer{}
	if err := png.Encode(&out, img); err != nil {
		t.Fatalf("encode fixture png: %v", err)
	}
	return out.Bytes()
}

// bombPNG crafts a valid PNG header claiming width×height — the decode
// bomb shape, refused by the pixel budget before any allocation.
func bombPNG(width, height int) []byte {
	out := bytes.NewBuffer(nil)
	out.Write([]byte("\x89PNG\r\n\x1a\n"))
	chunk := bytes.NewBuffer(nil)
	chunk.Write([]byte("IHDR"))
	binary.Write(chunk, binary.BigEndian, uint32(width))
	binary.Write(chunk, binary.BigEndian, uint32(height))
	chunk.Write([]byte{8, 2, 0, 0, 0})
	binary.Write(out, binary.BigEndian, uint32(13))
	out.Write(chunk.Bytes())
	binary.Write(out, binary.BigEndian, crc32.ChecksumIEEE(chunk.Bytes()))
	return out.Bytes()
}

// multipartUploadBody builds a real multipart body carrying one file
// part under field "file", returning the body and its content type.
func multipartUploadBody(t *testing.T, filename string, payload []byte) (string, *bytes.Buffer) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return writer.FormDataContentType(), body
}

func newFilesDemoServer(t *testing.T, runner demos.FilesWorkbenchRunner) (http.Handler, *files.LocalStore) {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	store, err := files.NewLocalStore(filepath.Join(t.TempDir(), "store"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("POST /api/demo/files/upload", NewFilesUploadHandler(store, files.DefaultUploadLimit, logger))
	mux.Handle("GET /api/demo/files/download/{name...}", NewFilesDownloadHandler(store, logger))
	mux.Handle("GET /api/demo/files/thumb/{name...}", NewFilesThumbHandler(store, logger))
	mux.Handle("GET /api/demo/files/workbench", NewFilesWorkbenchHandler(runner, logger))
	return Chain(RequestID, RequestLogger(logger), Recover(logger), EnvelopeMuxErrors(logger))(mux), store
}

func doFilesRequest(server http.Handler, method, path, contentType string, body io.Reader, header map[string]string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}
	server.ServeHTTP(rec, req)
	return rec
}

// seedObject streams payload into the store under name — the honest way
// to place an object the download/thumb routes serve.
func seedObject(t *testing.T, store *files.LocalStore, name string, payload []byte) {
	t.Helper()
	if _, err := store.SaveStream(context.Background(), name, bytes.NewReader(payload), files.DefaultUploadLimit); err != nil {
		t.Fatalf("seed %s: %v", name, err)
	}
}

// ── the upload route ─────────────────────────────────────────────────────

func TestFilesUploadStreamsAndAnswersSniffedMetadata(t *testing.T) {
	server, _ := newFilesDemoServer(t, nil)
	// Deterministic binary bytes (control and high bytes, no magic):
	// the sniffer's honest answer is application/octet-stream.
	payload := make([]byte, 300_000)
	for i := range payload {
		payload[i] = byte(i*251 + 7)
	}
	contentType, body := multipartUploadBody(t, "sample.bin", payload)
	rec := doFilesRequest(server, http.MethodPost, "/api/demo/files/upload", contentType, body, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var answer struct {
		Name        string `json:"name"`
		Bytes       int64  `json:"bytes"`
		ContentType string `json:"contentType"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("answer shape: %v", err)
	}
	if answer.Name != "sample.bin" || answer.Bytes != int64(len(payload)) {
		t.Errorf("answer = %+v", answer)
	}
	if answer.ContentType != "application/octet-stream" {
		t.Errorf("content type = %q, want sniffed from the stored bytes", answer.ContentType)
	}

	// A real PNG under a .png name: the answer must come from the magic
	// bytes, not from the filename or the client's claim.
	pngFixture := solidPNG(t, 4, 4, color.NRGBA{A: 0xFF})
	contentType, body = multipartUploadBody(t, "photo.png", pngFixture)
	rec = doFilesRequest(server, http.MethodPost, "/api/demo/files/upload", contentType, body, nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil || rec.Code != http.StatusCreated {
		t.Fatalf("png upload: %d %v", rec.Code, err)
	}
	if answer.ContentType != "image/png" {
		t.Errorf("png content type = %q, want the sniffed image/png", answer.ContentType)
	}
}

func TestFilesUploadBasenamesHostileFilenamesBeforeTheStore(t *testing.T) {
	// Go's own multipart stack defangs structure first: Part.FileName()
	// returns the path BASE, so "../escape.txt" arrives at the store as
	// "escape.txt" and is legitimately accepted. The route's
	// ErrInvalidName mapping stays as defense in depth (the sanitizer
	// still guards every direct SaveStream caller); this test pins the
	// layering so a future Go change cannot pass silently.
	server, _ := newFilesDemoServer(t, nil)
	for _, tc := range []struct{ raw, base string }{
		{"../escape.txt", "escape.txt"},
		{"/etc/passwd", "passwd"},
		{"nested/dir.txt", "dir.txt"},
	} {
		contentType, body := multipartUploadBody(t, tc.raw, []byte("x"))
		rec := doFilesRequest(server, http.MethodPost, "/api/demo/files/upload", contentType, body, nil)
		if rec.Code != http.StatusCreated {
			t.Errorf("filename %q: status = %d, want 201 under the basename: %s",
				tc.raw, rec.Code, rec.Body.String())
			continue
		}
		var answer struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
			t.Fatal(err)
		}
		if answer.Name != tc.base {
			t.Errorf("filename %q stored as %q, want the base %q", tc.raw, answer.Name, tc.base)
		}
	}
}

func TestFilesUploadRefusesMissingFilePartsAndBrokenContentTypes(t *testing.T) {
	server, _ := newFilesDemoServer(t, nil)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if field, err := writer.CreateFormField("note"); err == nil {
		field.Write([]byte("no file here"))
	}
	writer.Close()
	rec := doFilesRequest(server, http.MethodPost, "/api/demo/files/upload", writer.FormDataContentType(), body, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("text-only body: status = %d, want 422", rec.Code)
	}
	rec = doFilesRequest(server, http.MethodPost, "/api/demo/files/upload", "application/json", strings.NewReader("{}"), nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("json body: status = %d, want 422", rec.Code)
	}
}

func TestFilesUploadRefusesOverLimitObjects(t *testing.T) {
	// A store-level limit far below the route's: proves the store's own
	// mid-stream refusal maps to the typed 422 just like the edge's.
	small, err := files.NewLocalStore(filepath.Join(t.TempDir(), "small"))
	if err != nil {
		t.Fatal(err)
	}
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("POST /api/demo/files/upload", NewFilesUploadHandler(small, 1024, logger))
	contentType, body := multipartUploadBody(t, "big.bin", bytes.Repeat([]byte{0x41}, 8192))
	rec := doFilesRequest(mux, http.MethodPost, "/api/demo/files/upload", contentType, body, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != string(core.KindValidationFailed) {
		t.Errorf("code = %q", envelope.Error.Code)
	}
	if limit, ok := envelope.Error.Details["limitBytes"].(float64); !ok || limit != 1024 {
		t.Errorf("details.limitBytes = %v, want 1024", envelope.Error.Details["limitBytes"])
	}
}

// ── the download route ───────────────────────────────────────────────────

func TestFilesDownloadServesTheWholeObjectWithHonestHeaders(t *testing.T) {
	server, store := newFilesDemoServer(t, nil)
	payload := []byte("0123456789abcdef")
	seedObject(t, store, "digits.bin", payload)
	rec := doFilesRequest(server, http.MethodGet, "/api/demo/files/download/digits.bin", "", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), payload) {
		t.Errorf("body drifted: %q", rec.Body.String())
	}
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges = %q", got)
	}
	disposition := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, `filename="digits.bin"`) ||
		!strings.Contains(disposition, `filename*=UTF-8''digits.bin`) {
		t.Errorf("Content-Disposition = %q, want both clauses", disposition)
	}
	if got := rec.Header().Get("ETag"); !strings.HasPrefix(got, `"`) {
		t.Errorf("ETag = %q, want a quoted strong validator", got)
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(payload)) {
		t.Errorf("Content-Length = %q", got)
	}
}

func TestFilesDownloadServesExactRangeSlices(t *testing.T) {
	server, store := newFilesDemoServer(t, nil)
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = byte(i & 0xFF)
	}
	seedObject(t, store, "ranged.bin", payload)

	// A mid-file slice: 206 with exactly those bytes.
	rec := doFilesRequest(server, http.MethodGet, "/api/demo/files/download/ranged.bin", "", nil,
		map[string]string{"Range": "bytes=100-199"})
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("slice status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 100-199/1024" {
		t.Errorf("Content-Range = %q", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), payload[100:200]) {
		t.Error("the 206 body must be EXACTLY the requested slice")
	}

	// A suffix range: the last 32 bytes.
	rec = doFilesRequest(server, http.MethodGet, "/api/demo/files/download/ranged.bin", "", nil,
		map[string]string{"Range": "bytes=-32"})
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("suffix status = %d", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), payload[992:]) {
		t.Error("the suffix body must be the object's tail")
	}

	// Past the end: 416 with the honest census.
	rec = doFilesRequest(server, http.MethodGet, "/api/demo/files/download/ranged.bin", "", nil,
		map[string]string{"Range": "bytes=5000-"})
	if rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("past-the-end status = %d, want 416", rec.Code)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes */1024" {
		t.Errorf("416 Content-Range = %q", got)
	}

	// Malformed and multi-range headers are IGNORED per RFC 7233: the
	// whole object, never half an answer.
	for _, header := range []string{"bytes=99-0", "chunks=0-9", "bytes=0-99,200-299"} {
		rec = doFilesRequest(server, http.MethodGet, "/api/demo/files/download/ranged.bin", "", nil,
			map[string]string{"Range": header})
		if rec.Code != http.StatusOK || rec.Body.Len() != 1024 {
			t.Errorf("Range %q: status = %d, body = %d bytes; want 200 with the whole object",
				header, rec.Code, rec.Body.Len())
		}
	}
}

func TestFilesDownloadHonorsIfRangeValidators(t *testing.T) {
	server, store := newFilesDemoServer(t, nil)
	payload := bytes.Repeat([]byte{0xC3}, 2048)
	seedObject(t, store, "validator.bin", payload)

	whole := doFilesRequest(server, http.MethodGet, "/api/demo/files/download/validator.bin", "", nil, nil)
	etag := whole.Header().Get("ETag")

	// The matching validator authorizes the range: 206 slice.
	rec := doFilesRequest(server, http.MethodGet, "/api/demo/files/download/validator.bin", "", nil,
		map[string]string{"Range": "bytes=0-99", "If-Range": etag})
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 {
		t.Fatalf("matching If-Range: status = %d, body = %d; want the 206 slice", rec.Code, rec.Body.Len())
	}

	// A stale validator ignores the range: the whole object, 200.
	rec = doFilesRequest(server, http.MethodGet, "/api/demo/files/download/validator.bin", "", nil,
		map[string]string{"Range": "bytes=0-99", "If-Range": `"never-matches"`})
	if rec.Code != http.StatusOK || rec.Body.Len() != 2048 {
		t.Fatalf("stale If-Range: status = %d, body = %d; want 200 with the whole object", rec.Code, rec.Body.Len())
	}
}

func TestFilesDownloadAnswersAbsenceAndHostileNames(t *testing.T) {
	server, _ := newFilesDemoServer(t, nil)
	rec := doFilesRequest(server, http.MethodGet, "/api/demo/files/download/absent.bin", "", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("absent object status = %d, want 404", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &envelope)
	if envelope.Error.Code != string(core.KindNotFound) {
		t.Errorf("absent object code = %q", envelope.Error.Code)
	}

	// An escaped separator smuggled through the wildcard: the route
	// reaches the handler, and the jail refuses the name.
	rec = doFilesRequest(server, http.MethodGet, "/api/demo/files/download/smuggled%2F..%2Fetc", "", nil, nil)
	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusNotFound {
		t.Fatalf("smuggled name status = %d, want the typed refusal", rec.Code)
	}
	if rec.Code == http.StatusUnprocessableEntity {
		json.Unmarshal(rec.Body.Bytes(), &envelope)
		if envelope.Error.Code != string(core.KindValidationFailed) {
			t.Errorf("smuggled name code = %q", envelope.Error.Code)
		}
	}
}

// ── the thumbnail route ──────────────────────────────────────────────────

func TestFilesThumbTransformsStoredImages(t *testing.T) {
	server, store := newFilesDemoServer(t, nil)
	seedObject(t, store, "photo.png", solidPNG(t, 320, 200, color.NRGBA{R: 0x20, G: 0x90, B: 0x50, A: 0xFF}))
	rec := doFilesRequest(server, http.MethodGet, "/api/demo/files/thumb/photo.png?max=64", "", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q", got)
	}
	decoded, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("thumb body must decode: %v", err)
	}
	if decoded.Bounds().Dx() != 64 || decoded.Bounds().Dy() != 40 {
		t.Errorf("thumb = %d×%d, want 64×40", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func TestFilesThumbValidatesItsBoundsAndInputs(t *testing.T) {
	server, store := newFilesDemoServer(t, nil)
	seedObject(t, store, "photo.png", solidPNG(t, 64, 64, color.NRGBA{A: 0xFF}))
	seedObject(t, store, "not-an-image.bin", []byte("just text, no pixels"))
	seedObject(t, store, "bomb.png", bombPNG(30000, 30000))

	for _, tc := range []struct {
		path string
		want int
	}{
		{"/api/demo/files/thumb/photo.png?max=8", http.StatusUnprocessableEntity},    // below the clamp
		{"/api/demo/files/thumb/photo.png?max=4096", http.StatusUnprocessableEntity}, // above the clamp
		{"/api/demo/files/thumb/photo.png?max=zero", http.StatusUnprocessableEntity}, // not a number
		{"/api/demo/files/thumb/not-an-image.bin", http.StatusUnprocessableEntity},   // undecodable
		{"/api/demo/files/thumb/bomb.png", http.StatusUnprocessableEntity},           // decode bomb
		{"/api/demo/files/thumb/absent.png", http.StatusNotFound},                    // unknown object
	} {
		rec := doFilesRequest(server, http.MethodGet, tc.path, "", nil, nil)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d: %s", tc.path, rec.Code, tc.want, rec.Body.String())
		}
	}
}

// ── the workbench story endpoint ─────────────────────────────────────────

type fakeFilesWorkbenchRunner struct {
	err error
}

func (f fakeFilesWorkbenchRunner) RunFilesWorkbenchStory(context.Context) (demos.FilesWorkbenchRun, error) {
	return demos.FilesWorkbenchRun{
		Upload:    demos.FilesUploadRow{Name: "x", Bytes: 1, Stored: true},
		OverLimit: demos.FilesOverLimitRow{Refused: true},
		Range:     demos.FilesRangeRow{SliceMatched: true},
		Jail:      demos.FilesJailRow{Attempts: []demos.FilesTraversalRow{{Attempt: "x", Refused: true}}},
		Thumb:     demos.FilesThumbRow{BombRefused: true},
	}, f.err
}

func TestFilesWorkbenchServesTheStoryShape(t *testing.T) {
	server, _ := newFilesDemoServer(t, fakeFilesWorkbenchRunner{})
	rec := doFilesRequest(server, http.MethodGet, "/api/demo/files/workbench", "", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Mechanism string `json:"mechanism"`
		Beats     []struct {
			Beat    string `json:"beat"`
			Verdict string `json:"verdict"`
			Facts   []any  `json:"facts"`
		} `json:"beats"`
		Proofs    []any  `json:"proofs"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("story shape: %v", err)
	}
	if payload.Mechanism != "files-workbench" || len(payload.Beats) != 5 {
		t.Errorf("mechanism = %q, beats = %d", payload.Mechanism, len(payload.Beats))
	}
	if payload.Beats[0].Facts == nil || len(payload.Proofs) == 0 || payload.Reasoning == "" {
		t.Error("payload must carry non-nil facts, proofs, and the reasoning paragraph")
	}
}

func TestFilesWorkbenchMapsRunnerFailureToInternal(t *testing.T) {
	server, _ := newFilesDemoServer(t, fakeFilesWorkbenchRunner{err: errors.New("boom")})
	rec := doFilesRequest(server, http.MethodGet, "/api/demo/files/workbench", "", nil, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != string(core.KindInternal) || envelope.Error.RequestID == "" {
		t.Errorf("envelope = %+v", envelope.Error)
	}
}
