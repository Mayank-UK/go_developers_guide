// The local-filesystem adapter for the files domain's Store port: every
// upload streams through a sibling temp file and lands under its final
// name by atomic rename (a refused, cancelled, or crashed upload never
// leaves a partial object), every download opens through the three-lock
// name jail (sanitizer, Clean+prefix, EvalSymlinks), and the content
// type is sniffed from the stored bytes — never trusted from the
// request that delivered them.
package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	domfiles "guide/internal/domain/files"
)

// DefaultRoot is the composition root's store location: a directory
// under os.TempDir(). Deliberately NOT environment-configured — the
// committed core config stays untouched for this teaching substrate,
// and a production deployment injects its own root (a volume, a bucket
// adapter) through the same Store port. The choice is documented here,
// where the wiring reads it.
func DefaultRoot() string {
	return filepath.Join(os.TempDir(), "guide-files-store")
}

// DefaultUploadLimit is the demo routes' upload ceiling: 8 MiB per
// object. The store enforces it with io.LimitReader mid-stream; the
// HTTP edge wraps the body in http.MaxBytesReader with headroom for the
// multipart framing, so the transport dies before the disk does.
const DefaultUploadLimit int64 = 8 << 20

// LocalStore is the Store port over one directory tree.
type LocalStore struct {
	root string
}

// Compile-time proof the adapter satisfies the domain port.
var _ domfiles.Store = (*LocalStore)(nil)

// NewLocalStore prepares (and creates) the store rooted at dir.
func NewLocalStore(dir string) (*LocalStore, error) {
	root := filepath.Clean(dir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("files: create store root %s: %w", root, err)
	}
	return &LocalStore{root: root}, nil
}

// Root exposes the store's root (the demo story names it in its rows).
func (s *LocalStore) Root() string { return s.root }

// SaveStream copies body into a sibling temp file bounded by limit,
// then renames it under the sanitized name. Because the rename is
// atomic, two concurrent uploads of the same name both succeed and the
// object is always exactly ONE of them — never an interleave.
func (s *LocalStore) SaveStream(ctx context.Context, name string, body io.Reader, limit int64) (domfiles.Object, error) {
	if limit <= 0 {
		return domfiles.Object{}, fmt.Errorf("files: upload limit %d must be positive", limit)
	}
	clean, err := domfiles.SanitizeFilename(name)
	if err != nil {
		return domfiles.Object{}, err
	}
	target, err := domfiles.Confine(s.root, clean)
	if err != nil {
		return domfiles.Object{}, err
	}

	tmp, err := os.CreateTemp(s.root, ".incoming-*")
	if err != nil {
		return domfiles.Object{}, fmt.Errorf("files: stage upload: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func(why error) (domfiles.Object, error) {
		tmp.Close()
		os.Remove(tmpName)
		return domfiles.Object{}, why
	}

	// limit+1 lets one extra byte through: reading it PROVES the stream
	// exceeded the limit without ever trusting a Content-Length header.
	written, err := io.Copy(tmp, io.LimitReader(&ctxReader{ctx: ctx, r: body}, limit+1))
	if err != nil {
		return cleanup(fmt.Errorf("files: stream upload: %w", err))
	}
	if written > limit {
		return cleanup(fmt.Errorf("%w: %d bytes against a %d-byte limit",
			domfiles.ErrTooLarge, written, limit))
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return domfiles.Object{}, fmt.Errorf("files: close staged upload: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return domfiles.Object{}, fmt.Errorf("files: land upload: %w", err)
	}
	return s.Stat(ctx, clean)
}

// Open returns the object's metadata plus a seek handle over its bytes.
func (s *LocalStore) Open(ctx context.Context, name string) (domfiles.Object, io.ReadSeekCloser, error) {
	path, err := s.pathFor(name)
	if err != nil {
		return domfiles.Object{}, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return domfiles.Object{}, nil, mapPathError(err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return domfiles.Object{}, nil, fmt.Errorf("files: stat object: %w", err)
	}
	if info.IsDir() {
		file.Close()
		return domfiles.Object{}, nil, fmt.Errorf("%w: %q is a directory", domfiles.ErrNotFound, name)
	}

	// Sniff from the bytes, then rewind: the caller's Range math starts
	// at offset zero no matter how far DetectContentType read.
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return domfiles.Object{}, nil, fmt.Errorf("files: rewind object: %w", err)
	}
	return domfiles.Object{
		Name:        filepath.Base(path),
		Size:        info.Size(),
		ContentType: http.DetectContentType(head[:n]),
		ModTime:     info.ModTime().UTC(),
	}, file, nil
}

// Stat returns the metadata without opening the bytes.
func (s *LocalStore) Stat(ctx context.Context, name string) (domfiles.Object, error) {
	path, err := s.pathFor(name)
	if err != nil {
		return domfiles.Object{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return domfiles.Object{}, mapPathError(err)
	}
	if info.IsDir() {
		return domfiles.Object{}, fmt.Errorf("%w: %q is a directory", domfiles.ErrNotFound, name)
	}
	head := make([]byte, 512)
	file, err := os.Open(path)
	if err != nil {
		return domfiles.Object{}, mapPathError(err)
	}
	defer file.Close()
	n, _ := io.ReadFull(file, head)
	return domfiles.Object{
		Name:        filepath.Base(path),
		Size:        info.Size(),
		ContentType: http.DetectContentType(head[:n]),
		ModTime:     info.ModTime().UTC(),
	}, nil
}

// pathFor runs the name through all three locks: the sanitizer refuses
// structure, Confine proves the cleaned path stays under the root, and
// EvalSymlinks re-proves the prefix over the REAL filesystem — because
// a symlink planted inside the root can point anywhere, and the string
// jail cannot see it. A not-yet-existing target resolves its parent, so
// a fresh upload's path is checked the same way as an old object's.
func (s *LocalStore) pathFor(name string) (string, error) {
	clean, err := domfiles.SanitizeFilename(name)
	if err != nil {
		return "", err
	}
	resolved, err := domfiles.Confine(s.root, clean)
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return "", fmt.Errorf("files: resolve store root: %w", err)
	}
	real, err := filepath.EvalSymlinks(resolved)
	if errors.Is(err, fs.ErrNotExist) {
		// The object may not exist yet (SaveStream's rename target) —
		// jail the parent directory instead, then re-append the name.
		realParent, parentErr := filepath.EvalSymlinks(filepath.Dir(resolved))
		if parentErr != nil {
			return "", mapPathError(parentErr)
		}
		real = filepath.Join(realParent, filepath.Base(resolved))
	} else if err != nil {
		return "", fmt.Errorf("files: resolve object path: %w", err)
	}
	if !strings.HasPrefix(real, realRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q resolves outside the store root", domfiles.ErrInvalidName, name)
	}
	return real, nil
}

// mapPathError converts filesystem absences into the domain's typed
// ErrNotFound so callers never string-match.
func mapPathError(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %w", domfiles.ErrNotFound, err)
	}
	return fmt.Errorf("files: store i/o: %w", err)
}

// ctxReader makes io.Copy cancellation-aware: each Read first checks
// the context, so a cancelled upload stops the copy (and the producer
// behind it, through backpressure) instead of draining to completion.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

// Read implements io.Reader with a context guard.
func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, fmt.Errorf("files: upload cancelled: %w", err)
	}
	n, err := c.r.Read(p)
	if err != nil && err != io.EOF && c.ctx.Err() != nil {
		return n, fmt.Errorf("files: upload cancelled: %w", c.ctx.Err())
	}
	return n, err
}
