package content

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DemoIDPattern pins demo block ids to "<area>:<name>" with lowercase kebab
// segments (e.g. "runtime:async") — the client demo registry's lookup key.
var DemoIDPattern = regexp.MustCompile(`^[a-z0-9-]+:[a-z0-9-]+$`)

// Registry is the validated, immutable set of registered topics and groups —
// the single topic-id source of truth. NewRegistry runs EVERY validation
// rule and refuses to exist otherwise: broken content fails the process at
// startup (main.go exits non-zero), never at request time. An empty
// registry (zero topics, zero groups) is valid — the fresh state before
// Phase 3 registers real topics.
type Registry struct {
	topics     []Topic
	groups     []TopicGroup
	topicsByID map[string]Topic
}

// NewRegistry validates topics and groups, then stores them ordered: topics
// and groups both sort by ascending order, so every consumer (summaries,
// nav, detail) serves a stable, sorted view without re-sorting at the edge.
func NewRegistry(topics []Topic, groups []TopicGroup) (Registry, error) {
	if err := validateGroups(groups); err != nil {
		return Registry{}, err
	}
	if err := validateTopics(topics, groups); err != nil {
		return Registry{}, err
	}

	sortedGroups := append([]TopicGroup(nil), groups...)
	sort.SliceStable(sortedGroups, func(i, j int) bool {
		return sortedGroups[i].Order < sortedGroups[j].Order
	})
	sortedTopics := append([]Topic(nil), topics...)
	sort.SliceStable(sortedTopics, func(i, j int) bool {
		return sortedTopics[i].Order < sortedTopics[j].Order
	})

	byID := make(map[string]Topic, len(sortedTopics))
	for _, topic := range sortedTopics {
		byID[topic.ID] = topic
	}
	return Registry{topics: sortedTopics, groups: sortedGroups, topicsByID: byID}, nil
}

// Summaries returns one summary per topic (ordered by topic order), each
// with its section count.
func (r Registry) Summaries() []TopicSummary {
	summaries := make([]TopicSummary, 0, len(r.topics))
	for _, topic := range r.topics {
		summaries = append(summaries, TopicSummary{
			ID:           topic.ID,
			Title:        topic.Title,
			Subtitle:     topic.Subtitle,
			Group:        topic.Group,
			Order:        topic.Order,
			SectionCount: len(topic.Sections),
		})
	}
	return summaries
}

// TopicByID returns the full topic for an id, or false when unknown.
func (r Registry) TopicByID(id string) (Topic, bool) {
	topic, ok := r.topicsByID[id]
	return topic, ok
}

// Groups returns all groups, ordered by group order. The result is always
// a non-nil slice so the wire renders an empty registry as [] rather than
// null — the client's DTO rejects a non-array.
func (r Registry) Groups() []TopicGroup {
	groups := make([]TopicGroup, len(r.groups))
	copy(groups, r.groups)
	return groups
}

func validateGroups(groups []TopicGroup) error {
	seenIDs := make(map[string]bool, len(groups))
	seenOrders := make(map[int]string, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(group.ID) == "" {
			return fmt.Errorf("group id must not be blank (title %q)", group.Title)
		}
		if strings.TrimSpace(group.Title) == "" {
			return fmt.Errorf("group title must not be blank (id %q)", group.ID)
		}
		if seenIDs[group.ID] {
			return fmt.Errorf("duplicate group id: %q", group.ID)
		}
		seenIDs[group.ID] = true
		if other, clash := seenOrders[group.Order]; clash {
			return fmt.Errorf("duplicate group order: %d used by %q and %q", group.Order, other, group.ID)
		}
		seenOrders[group.Order] = group.ID
	}
	return nil
}

func validateTopics(topics []Topic, groups []TopicGroup) error {
	knownGroups := make(map[string]bool, len(groups))
	for _, group := range groups {
		knownGroups[group.ID] = true
	}

	seenTopicIDs := make(map[string]bool, len(topics))
	seenOrders := make(map[int]string, len(topics))
	for _, topic := range topics {
		if strings.TrimSpace(topic.ID) == "" {
			return fmt.Errorf("topic id must not be blank (title %q)", topic.Title)
		}
		if seenTopicIDs[topic.ID] {
			return fmt.Errorf("duplicate topic id: %q", topic.ID)
		}
		seenTopicIDs[topic.ID] = true

		if !knownGroups[topic.Group] {
			return fmt.Errorf(
				"unknown group: topic %q references group %q that the registry does not define",
				topic.ID, topic.Group,
			)
		}

		if other, clash := seenOrders[topic.Order]; clash {
			return fmt.Errorf("duplicate topic order: %d used by %q and %q", topic.Order, other, topic.ID)
		}
		seenOrders[topic.Order] = topic.ID

		if err := validateTopicContent(topic); err != nil {
			return err
		}
	}
	return nil
}

// validateTopicContent checks one topic's sections and blocks. The rules
// re-validate everything the constructors already enforce — struct literals
// bypass constructors in Go, so the registry is the real production gate.
func validateTopicContent(topic Topic) error {
	if len(topic.Sections) == 0 {
		return fmt.Errorf("topic %q needs at least one section", topic.ID)
	}

	seenSectionIDs := make(map[string]bool, len(topic.Sections))
	seenBlockIDs := make(map[string]string)
	totalBlocks := 0
	hasText := false
	for _, section := range topic.Sections {
		if strings.TrimSpace(section.ID) == "" {
			return fmt.Errorf("section id must not be blank in topic %q", topic.ID)
		}
		if seenSectionIDs[section.ID] {
			return fmt.Errorf("duplicate section id: %q in topic %q", section.ID, topic.ID)
		}
		seenSectionIDs[section.ID] = true

		// A section needs at least one block — but the family authoring rule
		// (fastapi registry: "TOPIC (≥2 blocks incl. ≥1 text)") is per TOPIC,
		// not per section: single-block narrative or terminal-companion
		// sections are legal authoring.
		if len(section.Blocks) < 1 {
			return fmt.Errorf(
				"empty section: %q in topic %q needs at least one block",
				section.ID, topic.ID,
			)
		}
		for _, block := range section.Blocks {
			if block == nil {
				return fmt.Errorf("nil block in section %q of topic %q", section.ID, topic.ID)
			}
			if err := validateBlock(topic, section, block, seenBlockIDs); err != nil {
				return err
			}
			totalBlocks++
			if _, ok := block.(TextBlock); ok {
				hasText = true
			}
		}
	}
	if totalBlocks < 2 {
		return fmt.Errorf("topic too short: %q needs at least 2 blocks across its sections", topic.ID)
	}
	if !hasText {
		return fmt.Errorf("topic without text block: %q needs at least one text block", topic.ID)
	}
	return nil
}

func validateBlock(topic Topic, section Section, block Block, seenBlockIDs map[string]string) error {
	id := blockIDOf(block)
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("block id must not be blank in section %q of topic %q", section.ID, topic.ID)
	}
	if other, clash := seenBlockIDs[id]; clash {
		return fmt.Errorf(
			"duplicate block id: %q in sections %q and %q of topic %q",
			id, other, section.ID, topic.ID,
		)
	}
	seenBlockIDs[id] = section.ID

	// The wire "kind" tag must match the concrete type — a mismatched tag
	// would render as a mystery block on the client.
	switch b := block.(type) {
	case TextBlock:
		if b.Kind != KindText {
			return fmt.Errorf("block %q is a TextBlock but carries kind %q", id, b.Kind)
		}
		if strings.TrimSpace(b.Text) == "" {
			return fmt.Errorf("empty text block: %q", id)
		}
	case CodeBlock:
		if b.Kind != KindCode {
			return fmt.Errorf("block %q is a CodeBlock but carries kind %q", id, b.Kind)
		}
		if strings.TrimSpace(b.Language) == "" {
			return fmt.Errorf("code block %q needs a language", id)
		}
		if strings.TrimSpace(b.Snippet) == "" {
			return fmt.Errorf("empty snippet: %q", id)
		}
		if MustSourceLanguages[strings.ToLower(b.Language)] && strings.TrimSpace(b.Source) == "" {
			return fmt.Errorf(
				"unsourced code block: %q language %q requires a non-empty source path",
				id, b.Language,
			)
		}
	case DemoBlock:
		if b.Kind != KindDemo {
			return fmt.Errorf("block %q is a DemoBlock but carries kind %q", id, b.Kind)
		}
		if strings.TrimSpace(b.Title) == "" {
			return fmt.Errorf("demo block %q needs a title", id)
		}
		if !DemoIDPattern.MatchString(id) {
			return fmt.Errorf(
				"invalid demo id: %q must match <area>:<name> with lowercase kebab segments", id,
			)
		}
	case ListBlock:
		if b.Kind != KindList {
			return fmt.Errorf("block %q is a ListBlock but carries kind %q", id, b.Kind)
		}
		if len(b.Items) < ListItemsMin || len(b.Items) > ListItemsMax {
			return fmt.Errorf(
				"list block size: %q has %d items; the standard allows %d–%d",
				id, len(b.Items), ListItemsMin, ListItemsMax,
			)
		}
		seenItems := make(map[string]bool, len(b.Items))
		for _, item := range b.Items {
			if strings.TrimSpace(item) == "" {
				return fmt.Errorf(
					"empty list item: %q every item needs a non-empty 'Lead term — explanation.'", id,
				)
			}
			if seenItems[item] {
				return fmt.Errorf("list block %q repeats item %q", id, item)
			}
			seenItems[item] = true
		}
	default:
		return fmt.Errorf("unknown block type %T in section %q of topic %q", block, section.ID, topic.ID)
	}
	return nil
}

// NewProductionRegistry builds the registry the running server serves: the
// guide's ten curriculum groups and every registered topic. Phase 3 onward
// adds one topic_<id>.go file per topic in this package (a builder
// returning (Topic, error)) and lists it here — this function is the
// single place topic registration happens, so its error fails startup.
func NewProductionRegistry() (Registry, error) {
	groups := []TopicGroup{
		{ID: "foundations", Title: "Foundations", Order: 1},
		{ID: "framework", Title: "Framework", Order: 2},
		{ID: "request-handling", Title: "Request handling", Order: 3},
		{ID: "apis", Title: "APIs", Order: 4},
		{ID: "data", Title: "Data", Order: 5},
		{ID: "state-auth", Title: "State & auth", Order: 6},
		{ID: "media-realtime", Title: "Media & realtime", Order: 7},
		{ID: "platform", Title: "Platform", Order: 8},
		{ID: "ship", Title: "Ship", Order: 9},
		{ID: "capstones", Title: "Capstones", Order: 10},
	}
	goLanguage, err := goLanguageTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build go-language topic: %w", err)
	}
	goRuntime, err := goRuntimeTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build go-runtime topic: %w", err)
	}
	httpWeb, err := httpWebTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build http-web topic: %w", err)
	}
	goHTTPFundamentals, err := goHTTPFundamentalsTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build go-http-fundamentals topic: %w", err)
	}
	interfacesComposition, err := interfacesCompositionTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build interfaces-and-composition topic: %w", err)
	}
	goroutinesChannels, err := goroutinesChannelsTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build goroutines-and-channels topic: %w", err)
	}
	routing, err := routingTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build routing topic: %w", err)
	}
	middleware, err := middlewareTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build middleware topic: %w", err)
	}
	requestResponse, err := requestResponseTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build request-response topic: %w", err)
	}
	validation, err := validationTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build validation topic: %w", err)
	}
	apiDesign, err := apiDesignTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build api-design topic: %w", err)
	}
	apiDocumentation, err := apiDocumentationTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build api-documentation topic: %w", err)
	}
	graphql, err := graphqlTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build graphql topic: %w", err)
	}
	webhooks, err := webhooksTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build webhooks topic: %w", err)
	}
	grpc, err := grpcTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build grpc topic: %w", err)
	}
	sqlSchema, err := sqlSchemaTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build sql-schema topic: %w", err)
	}
	databasesSQL, err := databasesSQLTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build databases-sql topic: %w", err)
	}
	sqlcGorm, err := sqlcGormTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build sqlc-gorm topic: %w", err)
	}
	mongodb, err := mongodbTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build mongodb topic: %w", err)
	}
	redis, err := redisTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build redis topic: %w", err)
	}
	caching, err := cachingTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build caching topic: %w", err)
	}
	messaging, err := messagingTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build messaging topic: %w", err)
	}
	authentication, err := authenticationTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build authentication topic: %w", err)
	}
	authorization, err := authorizationTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build authorization topic: %w", err)
	}
	security, err := securityTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build security topic: %w", err)
	}
	filesMedia, err := filesMediaTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build files-media topic: %w", err)
	}
	internationalization, err := internationalizationTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build internationalization topic: %w", err)
	}
	realtime, err := realtimeTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build realtime topic: %w", err)
	}
	backgroundJobs, err := backgroundJobsTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build background-jobs topic: %w", err)
	}
	errorHandling, err := errorHandlingTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build error-handling topic: %w", err)
	}
	dependencyManagement, err := dependencyManagementTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build dependency-management topic: %w", err)
	}
	architecture, err := architectureTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build architecture topic: %w", err)
	}
	observability, err := observabilityTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build observability topic: %w", err)
	}
	testingContent, err := testingTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build testing topic: %w", err)
	}
	debugging, err := debuggingTopic()
	if err != nil {
		return Registry{}, fmt.Errorf("build debugging topic: %w", err)
	}
	return NewRegistry(
		[]Topic{
			goLanguage, goRuntime, httpWeb, goHTTPFundamentals, interfacesComposition,
			goroutinesChannels, routing, middleware, requestResponse, validation, apiDesign,
			apiDocumentation, graphql, webhooks, grpc, sqlSchema, databasesSQL, sqlcGorm,
			mongodb, redis, caching, messaging, authentication, authorization, security,
			filesMedia, internationalization, realtime, backgroundJobs, errorHandling,
			dependencyManagement, architecture, observability, testingContent, debugging,
		},
		groups,
	)
}
