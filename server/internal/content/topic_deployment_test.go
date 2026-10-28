package content

import (
	"strings"
	"testing"
)

// Topic 37 deployment — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the demo inline in
// the section that teaches it (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the
// must-source belt, and the full snippet ratchet on a one-topic
// registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 37 and group
// "ship" on the built topic itself.

const deploymentID = "deployment"

var deploymentSectionIDs = []string{
	"ship-vs-run",
	"static-binary",
	"distroless-build",
	"compose-stack",
	"reverse-proxy",
	"graceful-shutdown",
	"ci-cd",
	"parity-secrets",
	"kubernetes",
	"rollouts",
	"infrastructure-as-code",
	"linux-survival",
	"terminal-companion",
}

// minDeploymentProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minDeploymentProseWords = 2500

func builtDeploymentTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := deploymentTopic()
	if err != nil {
		t.Fatalf("deployment builder must build cleanly: %v", err)
	}
	return topic
}

func registeredDeploymentTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtDeploymentTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "ship", Title: "Ship", Order: 9}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestDeploymentIsShipOrderThirtySeven(t *testing.T) {
	topic := registeredDeploymentTopic(t)
	if topic.Group != "ship" || topic.Order != 37 {
		t.Errorf("topic group/order = %q/%d, want ship/37", topic.Group, topic.Order)
	}
	if topic.ID != deploymentID {
		t.Errorf("topic id = %q, want %q", topic.ID, deploymentID)
	}
}

func TestDeploymentSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(deploymentSectionIDs))
	for _, section := range builtDeploymentTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(deploymentSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, deploymentSectionIDs)
	}
}

func TestDeploymentBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtDeploymentTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestDeploymentDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins deploy:stack to the compose stack
// section, co-present with prose and sourced code.
func TestDeploymentDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtDeploymentTopic(t)
	wants := map[string]string{
		"compose-stack": "deploy:stack",
	}

	total := 0
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			if demo, ok := block.(DemoBlock); ok {
				total++
				want, teachable := wants[section.ID]
				if !teachable || demo.ID != want {
					t.Errorf("demo %q sits in section %q; want %q", demo.ID, section.ID, wants[section.ID])
				}
			}
		}
	}
	if total != 1 {
		t.Errorf("topic carries %d demos, want exactly 1", total)
	}

	for sectionID := range wants {
		var section *Section
		for i := range topic.Sections {
			if topic.Sections[i].ID == sectionID {
				section = &topic.Sections[i]
				break
			}
		}
		if section == nil {
			t.Fatalf("section %s missing", sectionID)
		}
		kinds := make(map[string]bool)
		for _, block := range section.Blocks {
			kinds[blockKindTag(block)] = true
		}
		for _, kind := range []string{KindText, KindCode, KindDemo} {
			if !kinds[kind] {
				t.Errorf("section %s lacks a %q block; kinds = %v", sectionID, kind, kinds)
			}
		}
	}
}

func TestDeploymentClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtDeploymentTopic(t).Sections
	last := sections[len(sections)-1]
	if last.ID != "terminal-companion" {
		t.Fatalf("last section = %q, want terminal-companion", last.ID)
	}
	codeBlocks := 0
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			codeBlocks++
			if code.Language != "bash" || code.Source != "" {
				t.Errorf("terminal block %q must be bash with no source", code.ID)
			}
		}
	}
	if codeBlocks < 2 {
		t.Error("terminal companion needs its curl tour blocks")
	}
}

// TestDeploymentTerminalCompanionDrivesTheDeploymentSurface pins the
// companion's promise: the tour reads the live stack viewer, proves
// the compose rail refuses and then validates, and dry-run-validates
// the k8s manifests the way the repo actually does (kubectl when
// present, the suite's structural parse always).
func TestDeploymentTerminalCompanionDrivesTheDeploymentSurface(t *testing.T) {
	sections := builtDeploymentTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/api/demo/deploy/stack",
		"docker compose",
		"deploy/docker-compose.prod.yml",
		"POSTGRES_PASSWORD",
		"deploy/k8s/",
		"kubectl",
		"go test ./internal/domain/demos/",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the deployment surface", want)
		}
	}
}

func TestDeploymentProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtDeploymentTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minDeploymentProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minDeploymentProseWords)
	}
	t.Logf("deployment prose word count: %d", words)
}

func TestDeploymentMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtDeploymentTopic(t).Sections {
		for _, block := range section.Blocks {
			code, ok := block.(CodeBlock)
			if !ok || !MustSourceLanguages[strings.ToLower(code.Language)] {
				continue
			}
			if strings.TrimSpace(code.Source) == "" {
				t.Errorf("code block %q (language %q) lacks a source path", code.ID, code.Language)
			}
		}
	}
}

func TestDeploymentSnippetSourcesResolve(t *testing.T) {
	topic := builtDeploymentTopic(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "ship", Title: "Ship", Order: 9}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}

// TestDeploymentQuotesTheRealInstruments pins the topic's thesis: the
// subject of the deployment topic is THIS repo's real shipping
// machinery — the prod Dockerfile, the deployment topic's own compose
// artifact and k8s manifests, the nginx front door, the composition
// root's drain order, and the demo's own parser, scanner, and handler.
func TestDeploymentQuotesTheRealInstruments(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtDeploymentTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		".docker/server/Dockerfile.prod",
		".docker/client/nginx.conf",
		"deploy/docker-compose.prod.yml",
		"deploy/k8s/configmap.json",
		"deploy/k8s/secret.json",
		"deploy/k8s/deployment.json",
		"deploy/k8s/service.json",
		"server/cmd/guide/main.go",
		"server/internal/domain/demos/deploy_stack.go",
		"server/internal/presentation/demo_deploy.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}
