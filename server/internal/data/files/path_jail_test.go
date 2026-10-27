package files

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domfiles "guide/internal/domain/files"
)

// The traversal PROOF: hostile names are refused at every entry point
// (save, open, stat), and the string jail's blind spot — a symlink
// planted inside the root pointing outside — is caught by the
// EvalSymlinks re-jail.

func TestTraversalNamesAreRefusedAtEveryEntryPoint(t *testing.T) {
	store := newTestStore(t)
	hostile := []string{
		"../escape.txt",
		"../../deeper/escape.txt",
		"/etc/passwd",
		`..\windows.txt`,
		"nested/dir.txt",
		"nested\\dir.txt",
		"nul\x00.txt",
		"..",
		".",
	}
	ctx := context.Background()
	for _, name := range hostile {
		if _, err := store.SaveStream(ctx, name, nil, 1<<20); !errors.Is(err, domfiles.ErrInvalidName) {
			t.Errorf("SaveStream(%q) err = %v, want ErrInvalidName", name, err)
		}
		if _, err := store.Stat(ctx, name); !errors.Is(err, domfiles.ErrInvalidName) {
			t.Errorf("Stat(%q) err = %v, want ErrInvalidName", name, err)
		}
		if _, _, err := store.Open(ctx, name); !errors.Is(err, domfiles.ErrInvalidName) {
			t.Errorf("Open(%q) err = %v, want ErrInvalidName", name, err)
		}
	}
	entries, err := os.ReadDir(store.Root())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("hostile saves must create nothing; root holds %d entries", len(entries))
	}
}

func TestSymlinkEscapeIsRefusedByEvalSymlinks(t *testing.T) {
	store := newTestStore(t)
	outside := t.TempDir() // a DIFFERENT tree from the store's
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret — must never be served"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(store.Root(), "leak.txt")); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"Open", "Stat"} {
		var err error
		if op == "Open" {
			_, _, err = store.Open(context.Background(), "leak.txt")
		} else {
			_, err = store.Stat(context.Background(), "leak.txt")
		}
		if !errors.Is(err, domfiles.ErrInvalidName) {
			t.Errorf("%s through the symlink: err = %v, want ErrInvalidName (the string jail passes; EvalSymlinks must not)", op, err)
		}
	}
	// Saving THROUGH the jailed name (replacing the symlink by rename)
	// is a normal save — the jail refuses reads that escape, not writes
	// that land inside.
	if _, err := store.SaveStream(context.Background(), "leak.txt", strings.NewReader("replacement bytes"), 1<<20); err != nil {
		t.Errorf("SaveStream over a symlink must rename-replace it: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(store.Root(), "leak.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "replacement bytes" {
		t.Errorf("replaced object = %q", raw)
	}
}

func TestSymlinkedDirectoryEscapeIsRefused(t *testing.T) {
	store := newTestStore(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "elsewhere.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A directory symlink INSIDE the root: names under it would resolve
	// outside. The sanitizer refuses separators, so the escape would
	// need "dir/elsewhere.txt" — already refused by name; the direct
	// probe here is Stat on the symlink itself, which resolves outside.
	if err := os.Symlink(outside, filepath.Join(store.Root(), "dirlink")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(context.Background(), "dirlink"); !errors.Is(err, domfiles.ErrInvalidName) {
		t.Errorf("Stat on a directory symlink out of the root: err = %v, want ErrInvalidName", err)
	}
}
