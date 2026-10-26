package content

import (
	"strings"
	"testing"
)

// Topic 15 grpc — pinned the same way the exemplar pins topic 01: exact
// section order, unique block ids, the inline demo in the section that
// teaches serving with grpc-go, the terminal companion last, the family
// prose floor, and the must-source belt (the snippet-sources ratchet in
// snippet_sources_test.go verifies every quoted line against the real
// file; this belt catches the authoring mistake of a must-source block
// with no source path at all). The proto blocks make this topic's ratchet
// belt especially load-bearing: the .proto contract is quoted extensively
// and must stay byte-honest against server/api/proto/posts.proto.
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic; the wave-close wiring adds grpc to that
// list), so this file asserts Order 15 and group "apis" on the built topic
// itself rather than on the production set.

const grpcID = "grpc"

var grpcSectionIDs = []string{
	"what-grpc-is",
	"proto-design",
	"four-stream-kinds",
	"serving-with-grpc-go",
	"stubs-and-channels",
	"status-and-deadlines",
	"protobuf-codegen-with-buf",
	"grpc-vs-rest",
	"terminal-companion",
}

// minGRPCProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minGRPCProseWords = 2500

// builtGRPC builds the topic directly; the builder must assemble every
// block without a single constructor error.
func builtGRPC(t *testing.T) Topic {
	t.Helper()
	topic, err := grpcTopic()
	if err != nil {
		t.Fatalf("grpc builder must build cleanly: %v", err)
	}
	return topic
}

// registeredGRPC validates the builder's output through the registry's
// own rules (the real production gate) on a one-topic registry carrying
// the topic's curriculum group.
func registeredGRPC(t *testing.T) Topic {
	t.Helper()
	topic := builtGRPC(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "apis", Title: "APIs", Order: 4}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestGRPCIsAPIsOrderFifteen(t *testing.T) {
	topic := registeredGRPC(t)
	if topic.Group != "apis" || topic.Order != 15 {
		t.Errorf("topic group/order = %q/%d, want apis/15", topic.Group, topic.Order)
	}
	if topic.ID != grpcID {
		t.Errorf("topic id = %q, want %q", topic.ID, grpcID)
	}
}

func TestGRPCSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(grpcSectionIDs))
	for _, section := range builtGRPC(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(grpcSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, grpcSectionIDs)
	}
}

func TestGRPCBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtGRPC(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestGRPCDemoSitsInlineInTheServingSection(t *testing.T) {
	topic := builtGRPC(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "serving-with-grpc-go" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("serving-with-grpc-go section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "api:grpc-console" {
		t.Errorf("demo blocks = %+v, want exactly api:grpc-console", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("serving-with-grpc-go section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestGRPCClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtGRPC(t).Sections
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
	if codeBlocks < 5 {
		t.Errorf("terminal companion needs its curl + grpcurl tour, got %d bash blocks", codeBlocks)
	}
}

func TestGRPCProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtGRPC(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minGRPCProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minGRPCProseWords)
	}
	t.Logf("grpc prose word count: %d", words)
}

func TestGRPCMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet. The proto language is
	// must-source, and this topic quotes posts.proto heavily.
	for _, section := range builtGRPC(t).Sections {
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

func TestGRPCSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization — proto
	// blocks against the contract, go blocks against the implementation
	// and the demo, yaml blocks against the buf configs, mod against the
	// pinned toolchain.
	topic := builtGRPC(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "apis", Title: "APIs", Order: 4}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}

func TestGRPCQuotesTheProtoContract(t *testing.T) {
	// The topic's proto blocks must quote the REAL contract — including
	// the four-kind service declaration and the reserved-field discipline
	// the proto teaches — sourced to server/api/proto/posts.proto.
	topic := builtGRPC(t)
	protoBlocks := 0
	sawService := false
	sawReserved := false
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			code, ok := block.(CodeBlock)
			if !ok || code.Language != "proto" {
				continue
			}
			if code.Source != "server/api/proto/posts.proto" {
				t.Errorf("proto block %q sourced to %q, want server/api/proto/posts.proto", code.ID, code.Source)
			}
			protoBlocks++
			if strings.Contains(code.Snippet, "service PostsService") {
				sawService = true
			}
			if strings.Contains(code.Snippet, "reserved 7 to 11") {
				sawReserved = true
			}
		}
	}
	if protoBlocks < 2 {
		t.Errorf("proto blocks = %d, want at least the header and the Post message quoted", protoBlocks)
	}
	if !sawService {
		t.Error("no proto block quotes the service declaration (the four kinds)")
	}
	if !sawReserved {
		t.Error("no proto block quotes the reserved-field discipline")
	}
}
