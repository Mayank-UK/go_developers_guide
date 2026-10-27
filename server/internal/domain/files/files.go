// Package files is topic 26's binary-cargo domain: the streaming store
// port (SaveStream bounded by a limit, Open returning a seeker for range
// serving), the filename sanitizer and root jail that make path
// traversal a REFUSED request instead of an incident, RFC 7233 range
// math as pure functions, and the Content-Disposition encoder that
// keeps filenames honest on the wire. Everything here is pure or
// interface: the local-filesystem adapter lives in internal/data/files.
package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// Object is one stored binary's metadata — everything a response needs
// except the bytes themselves, which Open streams.
type Object struct {
	// Name is the store's sanitized single-component filename.
	Name string
	// Size is the object's byte length.
	Size int64
	// ContentType is sniffed from the stored bytes (the first 512), not
	// trusted from the upload request.
	ContentType string
	// ModTime is when the object last landed under its final name.
	ModTime time.Time
}

// The store's typed error vocabulary — errors.Is-able end to end, so the
// presentation layer maps each to its honest envelope (not_found 404,
// validation_failed 422) without string matching.
var (
	// ErrNotFound means the named object is not in the store.
	ErrNotFound = errors.New("files: object not found")
	// ErrInvalidName means the name was refused by the sanitizer or the
	// root jail: separators, "..", absolute paths, NUL bytes, symlink
	// escapes.
	ErrInvalidName = errors.New("files: name refused")
	// ErrTooLarge means the stream exceeded its stated limit and was
	// refused mid-copy (the partial write is cleaned up by the adapter).
	ErrTooLarge = errors.New("files: object exceeds the upload limit")
)

// Store is the streaming binary-store port: uploads arrive as readers,
// downloads leave as seekers. Two rules are part of the contract, not
// the implementations — a SaveStream call never materializes the whole
// object in memory (the limit exists so io.LimitReader can bound the
// copy), and Open returns an io.ReadSeekCloser so range requests can
// seek instead of re-reading.
type Store interface {
	// SaveStream reads body (already an untrusted stream) under name,
	// refusing with ErrTooLarge once more than limit bytes have been
	// read. The adapter streams to a sibling temp file and renames, so
	// a refused or crashed upload never leaves a partial object behind.
	SaveStream(ctx context.Context, name string, body io.Reader, limit int64) (Object, error)
	// Open returns the object's metadata plus a seeker over its bytes.
	// ErrNotFound when absent; ErrInvalidName when the name refuses or
	// escapes the root (including via symlink).
	Open(ctx context.Context, name string) (Object, io.ReadSeekCloser, error)
	// Stat returns the metadata without opening the bytes.
	Stat(ctx context.Context, name string) (Object, error)
}

// SanitizeFilename accepts exactly one filesystem component. The rule is
// an allowlist posture on structure: separators of both families are
// refused (a filename never needs one), NUL bytes are refused, "." and
// ".." are refused, and any embedded ".." is refused outright — a
// conservative sweep, because the legitimate spellings a download route
// needs are plain names and the hostile spellings all smuggle structure.
func SanitizeFilename(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("%w: %q is empty", ErrInvalidName, raw)
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("%w: %q is a directory reference", ErrInvalidName, raw)
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("%w: %q carries a path separator", ErrInvalidName, raw)
	}
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: %q carries a NUL byte", ErrInvalidName, raw)
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("%w: %q carries '..'", ErrInvalidName, raw)
	}
	return name, nil
}

// Confine resolves a sanitized name under root and proves the result
// cannot leave root: filepath.Clean the join, then require the root as a
// strict path-prefix (root itself is not an object). This is the second
// lock on the door — SanitizeFilename refuses structure, Confine proves
// the surviving string stays inside — and the adapter adds the third
// (EvalSymlinks) because cleaning confines the STRING, not the
// filesystem it names.
func Confine(root, name string) (string, error) {
	cleanRoot := filepath.Clean(root)
	resolved := filepath.Clean(filepath.Join(cleanRoot, name))
	if !strings.HasPrefix(resolved, cleanRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q escapes the store root", ErrInvalidName, name)
	}
	return resolved, nil
}

// ContentDisposition renders the download header for filename:
//
//	attachment; filename="ascii-fallback"; filename*=UTF-8''percent-encoded
//
// Both clauses per RFC 6266/5987: the quoted fallback is ASCII-only
// (quotes and backslashes escaped, everything non-ASCII replaced with
// "_"), and the ext-value carries the real name percent-encoded UTF-8.
// Serving the raw name in the quoted clause is the classic header
// injection bug — a newline or quote in a filename would rewrite the
// response's header shape.
func ContentDisposition(filename string) string {
	var fallback strings.Builder
	for _, r := range filename {
		switch {
		case r == '"' || r == '\\':
			fallback.WriteByte('\\')
			fallback.WriteRune(r)
		case r > 0x20 && r < 0x7F:
			fallback.WriteRune(r)
		default:
			fallback.WriteByte('_')
		}
	}
	return fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s",
		fallback.String(), url.PathEscape(filename))
}
