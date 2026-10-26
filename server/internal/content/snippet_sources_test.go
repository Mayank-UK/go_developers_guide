package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Snippet-source ratchet: every must-source block quotes a real repo file.
//
// THE CODE IS THE SOURCE OF TRUTH. For every block in a registry whose
// language is must-source: the source path must resolve under the repo
// root, the file must exist, and the snippet must match it (RATCHET = 0
// violations). The production registry carries real topics from Phase 3
// on — today it is empty, so the walker runs trivially green — and the
// canary below proves the harness actually bites: an intentionally bad
// block sourcing a REAL file with a snippet line that exists nowhere in it
// must be flagged, and only it.

// RATCHET is the maximum allowed violations. Never raise it; drive
// violations to zero and keep them there.
const RATCHET = 0

func TestRepoRootResolvesTheRealRepo(t *testing.T) {
	// The locator walks up from the WORKING DIRECTORY — go test runs each
	// package in its own dir, so this proves the walk from this package's
	// depth (internal/content) finds the server module's PARENT, not some
	// other module above it.
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	if !filepath.IsAbs(root) {
		t.Errorf("RepoRoot %q is not absolute", root)
	}
	raw, err := os.ReadFile(filepath.Join(root, "server", "go.mod"))
	if err != nil {
		t.Fatalf("repo root %q does not contain server/go.mod: %v", root, err)
	}
	if !strings.Contains(string(raw), "module guide") {
		t.Errorf("server/go.mod under %q is not this module — wrong root", root)
	}
}

// violations walks every must-source code block in the registry and
// reports each sourcing violation as "block id: reason". Containment is
// lexical (filepath.Rel): the ratchet's threat model is authoring
// mistakes, not symlink adversaries.
func violations(t *testing.T, registry Registry) []string {
	t.Helper()
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	var found []string
	for _, summary := range registry.Summaries() {
		topic, ok := registry.TopicByID(summary.ID)
		if !ok {
			t.Fatalf("summary topic %q missing from registry", summary.ID)
		}
		for _, section := range topic.Sections {
			for _, block := range section.Blocks {
				code, isCode := block.(CodeBlock)
				if !isCode || !MustSourceLanguages[strings.ToLower(code.Language)] {
					continue
				}
				if strings.TrimSpace(code.Source) == "" {
					found = append(found, code.ID+": missing source path")
					continue
				}
				path := filepath.Join(root, code.Source)
				if filepath.IsAbs(code.Source) {
					found = append(found, code.ID+": source escapes the repo root: "+code.Source)
					continue
				}
				if rel, relErr := filepath.Rel(root, path); relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					found = append(found, code.ID+": source escapes the repo root: "+code.Source)
					continue
				}
				raw, readErr := os.ReadFile(path)
				if readErr != nil {
					found = append(found, code.ID+": source file not found: "+code.Source)
					continue
				}
				if ok, reason := MatchSnippet(string(raw), code.Snippet, code.Language); !ok {
					found = append(found, code.ID+": "+reason)
				}
			}
		}
	}
	return found
}

func TestProductionRegistryRatchetIsZero(t *testing.T) {
	registry, err := NewProductionRegistry()
	if err != nil {
		t.Fatalf("production registry must boot: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}

func TestCanaryHarnessFlagsExactlyTheBadBlock(t *testing.T) {
	// A real-file-sourced block with an invented line — flagged, and only
	// it: the sourced go.mod block passes, the bash terminal tour is exempt.
	good, err := NewCodeBlock("good-mod", "mod", "", "server/go.mod", "module guide")
	if err != nil {
		t.Fatalf("canary good block: %v", err)
	}
	bad, err := NewCodeBlock("bad-go", "go", "", "server/cmd/guide/main.go", "THIS_LINE_EXISTS_IN_NO_REPO_FILE = true")
	if err != nil {
		t.Fatalf("canary bad block: %v", err)
	}
	tour, err := NewCodeBlock("bash-tour", "bash", "", "", "curl localhost:8100/api/content")
	if err != nil {
		t.Fatalf("canary tour block: %v", err)
	}

	canary := buildRegistry(t,
		[]Topic{buildTopic(t, "ratchet-canary", 1,
			buildSection(t, "good", buildText(t, "sourced:"), good),
			buildSection(t, "bad", buildText(t, "invented:"), bad),
			buildSection(t, "terminal", buildText(t, "tour:"), tour),
		)},
		[]TopicGroup{buildGroup(t, "foundations", 1)},
	)

	found := violations(t, canary)
	if len(found) != 1 {
		t.Fatalf("canary flagged %d violations (%v), want exactly the bad block", len(found), found)
	}
	if !strings.HasPrefix(found[0], "bad-go:") || !strings.Contains(found[0], "not found in source") {
		t.Errorf("canary violation %q does not name the bad block's reason", found[0])
	}
}

func TestSourcePathsMustStayUnderTheRepoRoot(t *testing.T) {
	outside, err := NewCodeBlock("escaping", "go", "", "../../../../../etc/hosts", "localhost")
	if err != nil {
		t.Fatalf("escape block: %v", err)
	}
	registry := buildRegistry(t,
		[]Topic{buildTopic(t, "outside", 1,
			buildSection(t, "s1", buildText(t, "escaping:"), outside))},
		[]TopicGroup{buildGroup(t, "foundations", 1)},
	)
	found := violations(t, registry)
	if len(found) != 1 || !strings.HasPrefix(found[0], "escaping: source escapes the repo root:") {
		t.Errorf("violations = %v, want exactly the escaping-source message", found)
	}
}

func TestMissingSourceFileIsAViolation(t *testing.T) {
	missing, err := NewCodeBlock("ghost", "go", "", "server/does/not/exist.go", "func No() {}")
	if err != nil {
		t.Fatalf("missing block: %v", err)
	}
	registry := buildRegistry(t,
		[]Topic{buildTopic(t, "ghost-topic", 1,
			buildSection(t, "s1", buildText(t, "missing:"), missing))},
		[]TopicGroup{buildGroup(t, "foundations", 1)},
	)
	found := violations(t, registry)
	if len(found) != 1 || !strings.HasPrefix(found[0], "ghost: source file not found:") {
		t.Errorf("violations = %v, want exactly the missing-file message", found)
	}
}
