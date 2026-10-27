package files

import (
	"errors"
	"strings"
	"testing"
)

// The name gate: SanitizeFilename accepts exactly one component and
// refuses every spelling of "leave this directory" — separators of both
// families, absolute paths, NUL bytes, dot segments, and any embedded
// "..". Confine then proves the surviving string stays under the root
// after Clean — belt and braces, because encodings survive naive checks.

func TestSanitizeFilenameAcceptsPlainComponents(t *testing.T) {
	for _, name := range []string{"report.pdf", "a", "notes 2.txt", "üñïçødé.png", ".hidden", "dots.dot.txt"} {
		got, err := SanitizeFilename(name)
		if err != nil || got != name {
			t.Errorf("SanitizeFilename(%q) = %q, %v; want it accepted verbatim", name, got, err)
		}
	}
}

func TestSanitizeFilenameRefusesHostileStructure(t *testing.T) {
	hostile := []string{
		"",
		"   ",
		".",
		"..",
		"../escape.txt",
		"..\\windows.txt",
		"/etc/passwd",
		"nested/dir.txt",
		"nul\x00.txt",
		"trailing..dots",
		"  ../padded.txt",
	}
	for _, name := range hostile {
		if _, err := SanitizeFilename(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("SanitizeFilename(%q) err = %v, want ErrInvalidName", name, err)
		}
	}
}

func TestConfineKeepsNamesUnderTheRoot(t *testing.T) {
	resolved, err := Confine("/store/root", "report.pdf")
	if err != nil {
		t.Fatalf("clean name refused: %v", err)
	}
	if resolved != "/store/root/report.pdf" {
		t.Errorf("resolved = %q", resolved)
	}
	// The root itself is not an object.
	if _, err := Confine("/store/root", "."); !errors.Is(err, ErrInvalidName) {
		t.Errorf("root itself must be refused, got %v", err)
	}
}

func TestConfineRefusesEveryEscape(t *testing.T) {
	// These reach Confine despite the sanitizer (direct callers, or a
	// future sanitizer regression): the prefix jail is the second lock
	// and must hold on its own.
	for _, name := range []string{"..", "../sibling.txt", "../../etc/passwd", "sub/../../escape"} {
		if _, err := Confine("/store/root", name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Confine(%q) err = %v, want ErrInvalidName", name, err)
		}
	}
}

func TestContentDispositionCarriesBothClauses(t *testing.T) {
	got := ContentDisposition("report.pdf")
	want := `attachment; filename="report.pdf"; filename*=UTF-8''report.pdf`
	if got != want {
		t.Errorf("plain name: got %q, want %q", got, want)
	}
}

func TestContentDispositionEscapesQuotesAndBackslashes(t *testing.T) {
	got := ContentDisposition(`evil".png`)
	if !strings.Contains(got, `filename="evil\".png"`) {
		t.Errorf("quote not escaped: %q", got)
	}
	if !strings.Contains(got, `filename*=UTF-8''evil%22.png`) {
		t.Errorf("ext-value missing the escaped quote: %q", got)
	}
}

func TestContentDispositionFallsBackToASCIIAndEncodesUnicode(t *testing.T) {
	got := ContentDisposition("lärm.png")
	if !strings.Contains(got, `filename="l_rm.png"`) {
		t.Errorf("ASCII fallback wrong: %q", got)
	}
	if !strings.Contains(got, `filename*=UTF-8''l%C3%A4rm.png`) {
		t.Errorf("unicode ext-value wrong: %q", got)
	}
}
