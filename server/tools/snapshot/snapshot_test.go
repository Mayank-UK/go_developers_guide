package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestModuleSnapshotMatchesSelectedVersions is the drift check: it
// regenerates `go list -m all` from the live module and compares it against
// the committed ArtifactName. Any change to a selected module version — an
// upgrade, a downgrade, a module entering or leaving the graph — fails here
// with the first differing lines and the command that refreshes the
// artifact. GOPROXY=off pins the offline property: the listing comes from
// the warm module cache (the dev container's gdg-go volume), never the
// network, so a cold-cache environment fails loudly instead of silently
// downloading.
func TestModuleSnapshotMatchesSelectedVersions(t *testing.T) {
	generated := goListModulesAll(t)
	committed := readCommittedSnapshot(t)

	if generated != committed {
		t.Helper()
		t.Fatalf("module snapshot drifted from the committed artifact — a selected module version changed.\n"+
			"first differing lines:\n%s\n"+
			"if the change is intentional, refresh the artifact from server/ with:\n"+
			"  %s\n"+
			"and commit it together with go.mod/go.sum",
			firstDiffLines(committed, generated, 6),
			RegenerateCommand)
	}
}

// TestSnapshotArtifactIsWellFormed pins the artifact's own shape so a
// hand-edit that breaks the format fails with a precise message rather than
// a confusing diff: comment header, then the main module line, then at
// least one module@version line — and every payload line after the first
// must carry a version (only the main module appears bare).
func TestSnapshotArtifactIsWellFormed(t *testing.T) {
	lines := payloadLines(readCommittedSnapshot(t))
	if len(lines) < 2 {
		t.Fatalf("snapshot payload has %d lines, want the main module line plus at least one module@version line", len(lines))
	}
	if lines[0] != "guide" {
		t.Errorf("first payload line = %q, want the bare main module line %q", lines[0], "guide")
	}
	for _, line := range lines[1:] {
		if !strings.Contains(line, " ") || strings.HasSuffix(line, " ") {
			t.Errorf("payload line %q is not a `module version` pair", line)
		}
	}
}

// goListModulesAll runs the listing exactly as the artifact header
// documents it, offline, from the module root two levels above this
// package's directory.
func goListModulesAll(t *testing.T) string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join(".", "..", ".."))
	if err != nil {
		t.Fatalf("resolving module root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(moduleRoot, "go.mod")); err != nil {
		t.Fatalf("module root has no go.mod (looked in %s): %v", moduleRoot, err)
	}

	cmd := exec.Command("go", "list", "-m", "all")
	cmd.Dir = moduleRoot
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go list -m all failed (offline, warm-cache): %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("running go list -m all: %v", err)
	}
	return strings.TrimRight(string(out), "\n")
}

// readCommittedSnapshot loads the artifact and reduces it to its payload:
// the header's comment lines stripped, payload lines joined as the
// regenerated listing is (trailing newline trimmed).
func readCommittedSnapshot(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(ArtifactName)
	if err != nil {
		t.Fatalf("reading committed snapshot %s: %v (regenerate it from server/ with `%s`)", ArtifactName, err, RegenerateCommand)
	}
	return strings.Join(payloadLines(string(raw)), "\n")
}

// payloadLines drops comment and blank lines, preserving payload order.
func payloadLines(content string) []string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// firstDiffLines renders up to max leading differing line pairs as a
// compact `-committed / +generated` block — enough to name the module that
// moved without dumping a 300-line diff into the failure message.
func firstDiffLines(committed, generated string, max int) string {
	oldLines := payloadLines(committed)
	newLines := payloadLines(generated)
	oldSet := make(map[string]bool, len(oldLines))
	newSet := make(map[string]bool, len(newLines))
	for _, line := range oldLines {
		oldSet[line] = true
	}
	for _, line := range newLines {
		newSet[line] = true
	}
	var diff []string
	for _, line := range oldLines {
		if !newSet[line] {
			diff = append(diff, "  - "+line)
		}
	}
	for _, line := range newLines {
		if !oldSet[line] {
			diff = append(diff, "  + "+line)
		}
	}
	if len(diff) > max {
		diff = append(diff[:max], fmtLinesRemaining(len(diff)-max)...)
	}
	return strings.Join(diff, "\n")
}

// fmtLinesRemaining keeps the truncated tail of a long diff honest.
func fmtLinesRemaining(n int) []string {
	return []string{strings.Repeat(" ", 4) + "...(" + strconv.Itoa(n) + " more differing lines)"}
}
