package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	domfiles "guide/internal/domain/files"
)

// The local store against a real filesystem: streamed saves land
// atomically, limits refuse mid-stream with nothing left behind,
// cancellation stops the copy, opens sniff their content type from the
// bytes, and concurrent saves of one name never interleave.

func newTestStore(t *testing.T) *LocalStore {
	t.Helper()
	store, err := NewLocalStore(filepath.Join(t.TempDir(), "store"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return store
}

// patternReader streams the deterministic object's first n bytes — the
// expected bytes are computable from the formula, so equality checks
// are proofs, not echoes.
type patternReader struct {
	total     int64
	remaining int64
}

func patternOf(n int64) io.Reader {
	return &patternReader{total: n, remaining: n}
}

func (p *patternReader) Read(buf []byte) (int, error) {
	if p.remaining <= 0 {
		return 0, io.EOF
	}
	size := min(int64(len(buf)), p.remaining)
	offset := p.total - p.remaining
	for i := int64(0); i < size; i++ {
		buf[i] = expectedByte(offset + i)
	}
	p.remaining -= size
	return int(size), nil
}

func TestSaveStreamLandsExactlyTheBytes(t *testing.T) {
	store := newTestStore(t)
	object, err := store.SaveStream(context.Background(), "notes.bin", patternOf(4096), 1<<20)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if object.Size != 4096 {
		t.Errorf("size = %d, want 4096", object.Size)
	}
	if object.ContentType != "application/octet-stream" {
		t.Errorf("content type = %q, want sniffed application/octet-stream", object.ContentType)
	}
	raw, err := os.ReadFile(filepath.Join(store.Root(), "notes.bin"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(raw, expectedSlice(0, 4096)) {
		t.Error("stored bytes differ from the deterministic pattern")
	}
}

func TestSaveStreamRefusesOverLimitAndCleansUp(t *testing.T) {
	store := newTestStore(t)
	_, err := store.SaveStream(context.Background(), "big.bin", patternOf(1<<20+512), 1<<20)
	if !errors.Is(err, domfiles.ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if _, err := store.Stat(context.Background(), "big.bin"); !errors.Is(err, domfiles.ErrNotFound) {
		t.Errorf("refused upload must not exist: stat err = %v", err)
	}
	entries, readErr := os.ReadDir(store.Root())
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".incoming-") {
			t.Errorf("staged temp %q left behind after refusal", entry.Name())
		}
	}
}

func TestSaveStreamAcceptsExactlyTheLimit(t *testing.T) {
	store := newTestStore(t)
	const limit = 4096
	object, err := store.SaveStream(context.Background(), "exact.bin", patternOf(limit), limit)
	if err != nil {
		t.Fatalf("a stream of exactly the limit must be accepted: %v", err)
	}
	if object.Size != limit {
		t.Errorf("size = %d, want %d", object.Size, limit)
	}
}

func TestSaveStreamRejectsNonPositiveLimits(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.SaveStream(context.Background(), "x.bin", strings.NewReader("x"), 0); err == nil {
		t.Error("limit 0 must be a programmer error")
	}
}

// cancelAfterReader cancels the context once enough bytes have flowed,
// then keeps answering reads — the ctxReader inside SaveStream must
// notice the cancellation and stop the copy.
type cancelAfterReader struct {
	inner   io.Reader
	flowed  int64
	onBytes func()
}

func (r *cancelAfterReader) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	r.flowed += int64(n)
	if r.onBytes != nil && r.flowed >= 64<<10 {
		r.onBytes()
		r.onBytes = nil
	}
	return n, err
}

func TestSaveStreamStopsOnContextCancellation(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slow := &cancelAfterReader{inner: patternOf(4 << 20)}
	slow.onBytes = cancel
	_, err := store.SaveStream(ctx, "cancelled.bin", slow, 1<<20)
	if err == nil {
		t.Fatal("a cancelled upload must fail, never complete")
	}
	if ctx.Err() == nil {
		t.Error("the context should have been cancelled mid-stream")
	}
	if _, statErr := store.Stat(context.Background(), "cancelled.bin"); !errors.Is(statErr, domfiles.ErrNotFound) {
		t.Errorf("cancelled upload must leave nothing: stat err = %v", statErr)
	}
	entries, _ := os.ReadDir(store.Root())
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".incoming-") {
			t.Errorf("staged temp %q left behind after cancellation", entry.Name())
		}
	}
}

func TestOpenSniffsContentTypesFromTheBytes(t *testing.T) {
	store := newTestStore(t)
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'} // the magic, then EOF
	if _, err := store.SaveStream(context.Background(), "tiny.png", bytes.NewReader(png), 1<<20); err != nil {
		t.Fatal(err)
	}
	object, content, err := store.Open(context.Background(), "tiny.png")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer content.Close()
	if object.ContentType != "image/png" {
		t.Errorf("content type = %q, want image/png sniffed from the bytes", object.ContentType)
	}
	head := make([]byte, len(png))
	if _, err := io.ReadFull(content, head); err != nil {
		t.Fatalf("read after open: %v", err)
	}
	if !bytes.Equal(head, png) {
		t.Error("Open must hand the seeker positioned at byte zero")
	}
}

func TestStatAndOpenMapAbsenceToTypedNotFound(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Stat(context.Background(), "absent.txt"); !errors.Is(err, domfiles.ErrNotFound) {
		t.Errorf("stat err = %v, want ErrNotFound", err)
	}
	if _, _, err := store.Open(context.Background(), "absent.txt"); !errors.Is(err, domfiles.ErrNotFound) {
		t.Errorf("open err = %v, want ErrNotFound", err)
	}
}

func TestConcurrentSavesOfOneNameNeverInterleave(t *testing.T) {
	store := newTestStore(t)
	const writers = 8
	const size = 256 << 10
	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each writer streams its own marker byte, repeated.
			marker := byte(i + 1)
			_, err := store.SaveStream(context.Background(), "shared.bin",
				bytes.NewReader(bytes.Repeat([]byte{marker}, size)), size)
			errs[i] = err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: %v", i, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(store.Root(), "shared.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(raw)) != size {
		t.Fatalf("object size = %d, want %d", len(raw), size)
	}
	first := raw[0]
	for i, b := range raw {
		if b != first {
			t.Fatalf("byte %d differs (%d vs %d): writers interleaved", i, b, first)
		}
	}
	if first == 0 {
		t.Fatal("marker bytes start at 1; a zero means no writer landed")
	}
}

func TestDefaultRootLivesUnderTempDir(t *testing.T) {
	if root := DefaultRoot(); !strings.HasPrefix(root, os.TempDir()+string(filepath.Separator)) {
		t.Errorf("DefaultRoot = %q, want under %q", root, os.TempDir())
	}
	if limit := DefaultUploadLimit; limit <= 0 {
		t.Errorf("DefaultUploadLimit = %d, want positive", limit)
	}
}
