// Topic 26's demo console. The composition root wires ONE console: its
// Store is the route store the curl tour uploads into. The workbench
// STORY deliberately runs against a FRESH store under a private temp
// dir (removed on exit) so every request observes the same beats —
// uploads land, limits refuse, ranges answer, the jail holds — request
// over request, regardless of what previous requests left in the route
// store.
package files

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"guide/internal/domain/demos"
	domfiles "guide/internal/domain/files"
)

// Console is the topic's substrate: the persistent route store plus the
// upload limit its routes enforce.
type Console struct {
	// Store serves the upload/download/thumb routes.
	Store *LocalStore
	// UploadLimit is the per-object byte ceiling those routes enforce.
	UploadLimit int64
}

// NewConsole wires the substrate under DefaultRoot().
func NewConsole() (*Console, error) {
	store, err := NewLocalStore(DefaultRoot())
	if err != nil {
		return nil, fmt.Errorf("files console: %w", err)
	}
	return &Console{Store: store, UploadLimit: DefaultUploadLimit}, nil
}

// Compile-time proof the console satisfies the demo runner port.
var _ demos.FilesWorkbenchRunner = (*Console)(nil)

// The story's deterministic fixtures: one 4 MiB object assembled from a
// repeating 64 KiB pattern, one mid-file slice, one suffix, one
// past-the-end range, one thumbnail geometry.
const (
	storyUploadName  = "workbench.bin"
	storyUploadBytes = 4 << 20
	storyPatternSize = 64 << 10
	storySliceHeader = "bytes=1048576-1048639"
	storySuffixRange = "bytes=-64"
	storyBeyondRange = "bytes=9999999-"
	storyStaleETag   = `"stale-never-matches"`
	storyOverLimit   = 1 << 20
	storyOverAttempt = storyOverLimit + 512<<10
	storyThumbMaxDim = 64
)

// RunFilesWorkbenchStory walks the topic's five beats against a fresh
// private store: a streamed multipart upload measured for honesty, the
// mid-stream size refusal, the range math over the stored object, the
// traversal sweep with its symlink escape, and the bounded thumbnail
// with a decode bomb refused from its header.
func (c *Console) RunFilesWorkbenchStory(ctx context.Context) (demos.FilesWorkbenchRun, error) {
	run := demos.FilesWorkbenchRun{}

	storyRoot, err := os.MkdirTemp("", "guide-files-story-*")
	if err != nil {
		return run, fmt.Errorf("files story: stage store: %w", err)
	}
	defer os.RemoveAll(storyRoot)
	store, err := NewLocalStore(storyRoot)
	if err != nil {
		return run, fmt.Errorf("files story: %w", err)
	}
	pattern := workbenchPattern()

	// ── beat 1: the streamed upload, measured ─────────────────────
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	bodyReader, bodyWriter := io.Pipe()
	contentType, written := streamWorkbenchUpload(bodyWriter, storyUploadName, pattern, storyUploadBytes)
	started := time.Now()
	object, err := domfiles.SaveMultipart(ctx, store, contentType, bodyReader, c.UploadLimit)
	wall := time.Since(started)
	// The store stops reading the moment its part ends, so the
	// generator's trailing multipart epilogue needs the read side
	// closed to unblock — expected, not an error.
	bodyReader.Close()
	if writerErr := <-written; err == nil && writerErr != nil {
		return run, fmt.Errorf("files story: generate upload: %w", writerErr)
	}
	runtime.ReadMemStats(&memAfter)
	if err != nil {
		return run, fmt.Errorf("files story upload: %w", err)
	}
	run.Upload = demos.FilesUploadRow{
		Name:       storyUploadName,
		Bytes:      object.Size,
		LimitBytes: c.UploadLimit,
		WallMs:     fmt.Sprintf("%.1f ms", float64(wall.Microseconds())/1000),
		AllocBytes: int64(memAfter.TotalAlloc - memBefore.TotalAlloc),
		Stored:     object.Size == storyUploadBytes,
	}

	// ── beat 2: the size bound refuses mid-stream ─────────────────
	overReader, overWriter := io.Pipe()
	overType, overWritten := streamWorkbenchUpload(overWriter, "toolarge.bin", pattern, storyOverAttempt)
	_, overErr := domfiles.SaveMultipart(ctx, store, overType, overReader, storyOverLimit)
	overReader.Close()
	<-overWritten
	leftovers, err := countStagingFiles(storyRoot)
	if err != nil {
		return run, fmt.Errorf("files story: staging census: %w", err)
	}
	run.OverLimit = demos.FilesOverLimitRow{
		Name:              "toolarge.bin",
		AttemptedBytes:    storyOverAttempt,
		LimitBytes:        storyOverLimit,
		Refused:           errors.Is(overErr, domfiles.ErrTooLarge),
		LeftoverTempFiles: leftovers,
	}

	// ── beat 3: the range math over the stored object ─────────────
	stored, bytesReader, err := store.Open(ctx, storyUploadName)
	if err != nil {
		return run, fmt.Errorf("files story: open stored object: %w", err)
	}
	defer bytesReader.Close()

	spec, unsatisfiable, present := domfiles.ParseRange(storySliceHeader, stored.Size)
	slice, err := readSliceAt(bytesReader, spec.Start, spec.Length())
	if err != nil {
		return run, fmt.Errorf("files story: read slice: %w", err)
	}
	want := expectedSlice(spec.Start, spec.Length())
	sliceMatched := present && !unsatisfiable && bytes.Equal(slice, want)

	suffixSpec, _, suffixPresent := domfiles.ParseRange(storySuffixRange, stored.Size)
	suffix, err := readSliceAt(bytesReader, suffixSpec.Start, suffixSpec.Length())
	if err != nil {
		return run, fmt.Errorf("files story: read suffix: %w", err)
	}
	suffixMatched := suffixPresent && bytes.Equal(suffix, expectedSlice(suffixSpec.Start, suffixSpec.Length()))

	_, beyondUnsatisfiable, beyondPresent := domfiles.ParseRange(storyBeyondRange, stored.Size)

	etag := domfiles.ObjectETag(stored)
	honorsStale := domfiles.HonorsIfRange(storyStaleETag, etag, stored.ModTime)
	if _, err := bytesReader.Seek(0, io.SeekStart); err != nil {
		return run, fmt.Errorf("files story: rewind: %w", err)
	}
	fullServed, err := verifyFullServe(bytesReader, pattern, stored.Size)
	if err != nil {
		return run, fmt.Errorf("files story: full serve check: %w", err)
	}
	run.Range = demos.FilesRangeRow{
		Object: stored.Name,
		Size:   stored.Size,

		Header:       storySliceHeader,
		ContentRange: domfiles.ContentRange(spec, stored.Size),
		SliceHex:     hexHead(slice),
		SliceMatched: sliceMatched,

		SuffixHeader:  storySuffixRange,
		SuffixRange:   domfiles.ContentRange(suffixSpec, stored.Size),
		SuffixMatched: suffixMatched,

		BeyondHeader:       storyBeyondRange,
		BeyondContentRange: domfiles.UnsatisfiableContentRange(stored.Size),
		BeyondRefused416:   beyondPresent && beyondUnsatisfiable,

		IfRangeHeader:     storyStaleETag,
		IfRangeServedFull: !honorsStale && fullServed == stored.Size,
		FullBytesServed:   fullServed,
	}

	// ── beat 4: the traversal sweep + the symlink escape ──────────
	attempts := []string{
		"../escape.txt",
		"/etc/passwd",
		`..\windows.txt`,
		"nested/dir.txt",
		"nul\x00.txt",
		"..",
	}
	rows := make([]demos.FilesTraversalRow, 0, len(attempts))
	for _, attempt := range attempts {
		_, statErr := store.Stat(ctx, attempt)
		rows = append(rows, demos.FilesTraversalRow{
			Attempt: attempt,
			Refused: errors.Is(statErr, domfiles.ErrInvalidName),
		})
	}
	// The symlink: planted INSIDE the root, pointing OUTSIDE it — the
	// string jail cannot see it, EvalSymlinks can.
	secret := filepath.Join(filepath.Dir(storyRoot), "workbench-secret-outside.txt")
	if err := os.WriteFile(secret, []byte("top secret — must never be served"), 0o600); err != nil {
		return run, fmt.Errorf("files story: stage secret: %w", err)
	}
	defer os.Remove(secret)
	if err := os.Symlink(secret, filepath.Join(storyRoot, "leak.txt")); err != nil {
		return run, fmt.Errorf("files story: stage symlink: %w", err)
	}
	_, _, leakErr := store.Open(ctx, "leak.txt")
	_, cleanErr := store.Stat(ctx, storyUploadName)
	run.Jail = demos.FilesJailRow{
		Root:             storyRoot,
		Attempts:         rows,
		SymlinkAttempt:   "leak.txt → (symlink outside the root)",
		SymlinkRefused:   errors.Is(leakErr, domfiles.ErrInvalidName),
		CleanStillServed: cleanErr == nil,
	}

	// ── beat 5: the bounded thumbnail + the decode bomb ───────────
	const sourceW, sourceH = 320, 200
	source := workbenchImage(sourceW, sourceH)
	pngBuf := bytes.Buffer{}
	if err := png.Encode(&pngBuf, source); err != nil {
		return run, fmt.Errorf("files story: encode source png: %w", err)
	}
	decoded, format, err := domfiles.DecodeBounded(bytes.NewReader(pngBuf.Bytes()))
	if err != nil {
		return run, fmt.Errorf("files story: decode source: %w", err)
	}
	thumb := domfiles.Thumbnail(decoded, storyThumbMaxDim)
	thumbPNG := bytes.Buffer{}
	if err := domfiles.EncodeThumbnail(&thumbPNG, thumb, "png"); err != nil {
		return run, fmt.Errorf("files story: encode png thumb: %w", err)
	}
	thumbJPEG := bytes.Buffer{}
	if err := domfiles.EncodeThumbnail(&thumbJPEG, thumb, "jpeg"); err != nil {
		return run, fmt.Errorf("files story: encode jpeg thumb: %w", err)
	}
	jpegDecoded, err := jpeg.Decode(bytes.NewReader(thumbJPEG.Bytes()))
	if err != nil {
		return run, fmt.Errorf("files story: decode jpeg thumb: %w", err)
	}
	bomb := hugePNGHeader(30000, 30000)
	_, _, bombErr := domfiles.DecodeBounded(bytes.NewReader(bomb))
	bounds := thumb.Bounds()
	run.Thumb = demos.FilesThumbRow{
		SourceLabel:  fmt.Sprintf("in-process PNG fixture (decoded as %s)", format),
		SourceW:      sourceW,
		SourceH:      sourceH,
		MaxDim:       storyThumbMaxDim,
		ThumbW:       bounds.Dx(),
		ThumbH:       bounds.Dy(),
		PngBytes:     thumbPNG.Len(),
		JpegBytes:    thumbJPEG.Len(),
		JpegDecodedW: jpegDecoded.Bounds().Dx(),
		JpegDecodedH: jpegDecoded.Bounds().Dy(),
		BombLabel:    "30000×30000",
		BombBytes:    len(bomb),
		BombRefused:  errors.Is(bombErr, domfiles.ErrImageTooLarge),
	}
	return run, nil
}

// workbenchPattern is the story's deterministic 64 KiB block: byte j is
// (j*7+13) mod 256, chosen so the block repeats to any 64 KiB-aligned
// length and the expected byte at ANY offset is computable on the spot.
func workbenchPattern() []byte {
	pattern := make([]byte, storyPatternSize)
	for j := range pattern {
		pattern[j] = byte((j*7 + 13) & 0xFF)
	}
	return pattern
}

// expectedByte is the deterministic object's byte at offset i.
func expectedByte(i int64) byte { return byte((i*7 + 13) & 0xFF) }

// expectedSlice renders length bytes of the deterministic object at
// offset start — computed from the formula, never from the stored file,
// so equality is a real proof.
func expectedSlice(start, length int64) []byte {
	out := make([]byte, length)
	for i := range out {
		out[i] = expectedByte(start + int64(i))
	}
	return out
}

// streamWorkbenchUpload generates a real multipart body on pw: exactly
// total bytes of the pattern under one file part, written chunk by
// chunk through the unbuffered pipe. It returns the body's content type
// (available before any byte flows) and a channel carrying the
// generator's own verdict. A write failing with ErrClosedPipe after the
// payload transferred is EXPECTED — the store stops reading at its
// part's end — and reported as success.
func streamWorkbenchUpload(pw *io.PipeWriter, name string, pattern []byte, total int64) (string, <-chan error) {
	writer := multipart.NewWriter(pw)
	contentType := writer.FormDataContentType()
	done := make(chan error, 1)
	go func() {
		part, err := writer.CreateFormFile("file", name)
		if err != nil {
			done <- fmt.Errorf("create form file: %w", err)
			pw.CloseWithError(err)
			return
		}
		var written int64
		for written < total {
			chunk := pattern
			if remain := total - written; remain < int64(len(chunk)) {
				chunk = chunk[:remain]
			}
			n, err := part.Write(chunk)
			written += int64(n)
			if err != nil {
				if errors.Is(err, io.ErrClosedPipe) {
					done <- nil // the store stopped reading: expected
					return
				}
				done <- fmt.Errorf("generate pattern: %w", err)
				return
			}
		}
		writer.Close()
		pw.Close()
		done <- nil
	}()
	return contentType, done
}

// readSliceAt seeks the seeker and reads exactly length bytes — the
// exact-slice proof for the range beats.
func readSliceAt(rs io.ReadSeeker, start, length int64) ([]byte, error) {
	if _, err := rs.Seek(start, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek %d: %w", start, err)
	}
	out := make([]byte, length)
	if _, err := io.ReadFull(rs, out); err != nil {
		return nil, fmt.Errorf("read %d at %d: %w", length, start, err)
	}
	return out, nil
}

// verifyFullServe streams n bytes from rs and proves every one equals
// the deterministic object: each 64 KiB-aligned block (and any partial
// tail) must equal the pattern block — the full-serve proof, streamed
// through one reused buffer rather than materialized.
func verifyFullServe(rs io.Reader, pattern []byte, n int64) (int64, error) {
	block := make([]byte, storyPatternSize)
	var read int64
	for read < n {
		want := int(min(int64(storyPatternSize), n-read))
		if _, err := io.ReadFull(rs, block[:want]); err != nil {
			return read, fmt.Errorf("read at %d: %w", read, err)
		}
		if !bytes.Equal(block[:want], pattern[:want]) {
			return read, fmt.Errorf("bytes at %d do not match the pattern", read)
		}
		read += int64(want)
	}
	return read, nil
}

// countStagingFiles counts leftover ".incoming-" temp files — the
// partial-write cleanup proof.
func countStagingFiles(root string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".incoming-") {
			count++
		}
	}
	return count, nil
}

// workbenchImage paints a deterministic source image: a two-tone
// quadrant field with a gradient stripe, so the thumbnail's box
// sampling has real edges to average.
func workbenchImage(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var c color.NRGBA
			switch {
			case y < height/4: // top band: horizontal gradient
				c = color.NRGBA{R: uint8(x * 255 / width), G: 0x40, B: 0x80, A: 0xFF}
			case x < width/2: // left quadrant: solid
				c = color.NRGBA{R: 0x20, G: 0x90, B: 0x50, A: 0xFF}
			default: // right quadrant: vertical gradient
				c = color.NRGBA{R: 0xF0, G: uint8(y * 255 / height), B: 0x30, A: 0xFF}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// hugePNGHeader crafts a valid PNG header (signature + IHDR) whose
// dimensions claim width×height — the shape of a real decode bomb: a
// few dozen bytes that would allocate gigabytes if decoded. CRCs are
// computed properly so the header parses.
func hugePNGHeader(width, height int) []byte {
	out := bytes.NewBuffer(nil)
	out.Write([]byte("\x89PNG\r\n\x1a\n"))
	chunk := bytes.NewBuffer(nil)
	chunk.Write([]byte("IHDR"))
	binary.Write(chunk, binary.BigEndian, uint32(width))
	binary.Write(chunk, binary.BigEndian, uint32(height))
	chunk.Write([]byte{8, 2, 0, 0, 0}) // 8-bit truecolor
	binary.Write(out, binary.BigEndian, uint32(13))
	out.Write(chunk.Bytes())
	binary.Write(out, binary.BigEndian, crc32.ChecksumIEEE(chunk.Bytes()))
	return out.Bytes()
}

// hexHead renders the first bytes of a slice for the story's facts.
func hexHead(b []byte) string {
	const show = 8
	if len(b) > show {
		b = b[:show]
	}
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*3)
	for i, c := range b {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, hexDigits[c>>4], hexDigits[c&0xF])
	}
	return string(out)
}
