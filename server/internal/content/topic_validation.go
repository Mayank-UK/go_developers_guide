package content

import (
	"errors"
	"fmt"
)

// This file is topic 10 validation (group request-handling, curriculum
// order 10), authored on the exemplar pattern: prose is the default
// block, every must-source code block quotes a real repo file (the
// snippet ratchet verifies it), the live demo sits INLINE in the section
// that teaches choosing between the layers, and the topic closes with
// the bash terminal companion — including a real 422 round-trip against
// the running stack.

// validationTopic builds topic 10 — the boundary discipline: why input
// is validated where it enters, the three layers (decode discipline,
// hand-rolled rules as the backbone, the validator-library ecosystem by
// comparison), custom and cross-field rules, how to choose, and the one
// error-envelope shape every 422 in this app answers with.
func validationTopic() (Topic, error) {
	var buildErr error
	collect := func(what string, err error) {
		if err != nil {
			buildErr = errors.Join(buildErr, fmt.Errorf("%s: %w", what, err))
		}
	}
	text := func(id, body string) TextBlock {
		block, err := NewTextBlock(id, body)
		collect("text "+id, err)
		return block
	}
	code := func(id, language, caption, source, snippet string) CodeBlock {
		block, err := NewCodeBlock(id, language, caption, source, snippet)
		collect("code "+id, err)
		return block
	}
	demo := func(id, title, description string) DemoBlock {
		block, err := NewDemoBlock(id, title, description)
		collect("demo "+id, err)
		return block
	}
	list := func(id string, ordered bool, items ...string) ListBlock {
		block, err := NewListBlock(id, ordered, items)
		collect("list "+id, err)
		return block
	}
	section := func(id, title, lead string, blocks ...Block) Section {
		assembled, err := NewSection(id, title, lead, blocks)
		collect("section "+id, err)
		return assembled
	}

	sections := []Section{
		// ── 1. Why validation lives at the boundary ────────────────────
		section("why-at-the-boundary", "Why validation lives at the boundary",
			"Everything past the handler is written assuming the input is already sane — so sanity has to be checked exactly once, at the door.",
			text("why-pitch",
				"A request body is the least trustworthy data your server will ever touch: it arrives from "+
					"an untrusted network, was assembled by who-knows-what client, and may be hostile on "+
					"purpose. Every layer past the parse — the use case, the repository, the SQL, the "+
					"response writer — is written assuming the input is already sane: the store assumes "+
					"``dataMode`` is one of two words, the content model assumes titles are single "+
					"non-blank strings, the database assumes lengths fit their columns. Validation at the "+
					"boundary is what makes those assumptions honest. The alternative is validating a "+
					"little everywhere, which in practice means validating nowhere consistently: rules "+
					"drift apart, one path forgets a check, and the invariant you relied on becomes a "+
					"production incident. One door, one bouncer, everything behind it clean."),
			text("why-fail-loud",
				"The second reason is what validation buys when it fails: a REFUSAL at the boundary is "+
					"cheap, local, and actionable — status 422, a body naming the fields, no side effects, "+
					"nothing half-done to roll back. The same bad input caught three layers down is "+
					"expensive: it may have written rows, sent messages, or charged a card before "+
					"someone noticed the negative quantity. Failing fast is not negativity; it is "+
					"containment. And the refusal must carry information — WHICH fields, WHAT rule, "+
					"what the server actually got — because a 422 that says only ``invalid request`` "+
					"just moves the debugging into your logs, where the caller cannot see it."),
			text("why-inside-too",
				"Boundary-first does not mean boundary-only, and this repo is honest about that. The "+
					"content package validates its own invariants at construction — a text block refuses "+
					"to exist with a blank id or blank text, long before any HTTP request is involved — "+
					"because the cheapest place to catch bad data is where it is born, not where it "+
					"crosses the network:"),
			code("why-require-non-blank", "go",
				"The content model's own gate: constructors refuse invalid data at birth.",
				"server/internal/content/model.go",
				"func requireNonBlank(field, value string) error {\n"+
					"    if strings.TrimSpace(value) == \"\" {\n"+
					"        return fmt.Errorf(\"%s must not be blank\", field)\n"+
					"    }\n"+
					"    return nil\n"+
					"}"),
			text("why-inside-read",
				"Defense in depth reads like this: the boundary rejects hostile input before it costs "+
					"anything, and each type defends its own invariants for every caller — including "+
					"tests, background jobs, and other handlers, which never pass through the HTTP "+
					"boundary at all. The scenario store takes that second duty seriously: its enums "+
					"carry their own vocabulary check, so ANY caller can ask a value whether it is legal "+
					"without knowing the list:"),
			code("why-valid-method", "go",
				"A domain type that knows its own legal values.",
				"server/internal/core/scenario.go",
				"// Valid reports whether the mode is one of the two legal values.\n"+
					"func (m DataMode) Valid() bool {\n"+
					"    return m == DataModeFake || m == DataModeLive\n"+
					"}"),
			text("why-layers-intro",
				"This topic organizes Go's validation tooling into three layers, and the order matters. "+
					"Layer one is the decode discipline — ``encoding/json`` plus a few deliberate "+
					"settings, which answers every question a TYPE system can answer: is this one JSON "+
					"value, are the keys known, are the values the declared types. Layer two is "+
					"hand-rolled field rules — plain functions expressing everything types cannot "+
					"state: lengths, ranges, shapes, cross-field logic. Layer three is the ecosystem "+
					"of struct-tag validator libraries, taught here by comparison, which compresses "+
					"layer two into declarative tags at a price worth understanding before paying. "+
					"The hand-rolled layer is the backbone this guide builds on — the one you must "+
					"be able to write on a whiteboard — and the tour below walks all three:"),
			list("why-layer-map", false,
				"Decode discipline — the struct's types as rules: one JSON value, known fields, declared types, bounded bodies; refusal here never becomes a field issue.",
				"Hand-rolled rules — the backbone: length, range, shape, and cross-field checks as plain, testable Go functions that collect every issue.",
				"Validator libraries — struct tags driving a reflection engine; declarative and rich, at the cost of a dependency and semantics you must learn.",
				"The 422 envelope — one error shape for every refusal: code, message, request id, and structured details naming each broken field."),
		),
		// ── 2. Layer one: the decode discipline ────────────────────────
		section("layer-decode", "Layer one: the decode discipline",
			"Before any rule runs, the struct's own types are rules — if you let them work.",
			text("decode-pitch",
				"Go's secret weapon in validation is one you already own: a typed struct IS a "+
					"validation contract, if you decode with discipline instead of by reflex. The naive "+
					"``json.Unmarshal(body, &anything)`` answers only ``is it JSON`` — it silently "+
					"ignores unknown keys, accepts trailing garbage after the value, cannot bound the "+
					"read, and reports type mismatches as vague errors. The disciplined decoder asks "+
					"sharper questions. This repo's scenario PATCH handler is the canonical real "+
					"example — read it as a checklist:"),
			code("decode-scenario", "go",
				"The repo's real decode discipline: bounded, one value, typed, null-refused.",
				"server/internal/presentation/scenario.go",
				"invalid := func(reason string) (core.ScenarioPatch, error) {\n"+
					"    return core.ScenarioPatch{}, core.ValidationFailed(\n"+
					"        \"Invalid scenario patch body.\", map[string]string{\"body\": reason},\n"+
					"    )\n"+
					"}\n"+
					"\n"+
					"decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxScenarioBodyBytes))\n"+
					"var raw json.RawMessage\n"+
					"if err := decoder.Decode(&raw); err != nil {\n"+
					"    return invalid(err.Error())\n"+
					"}\n"+
					"var trailing any\n"+
					"if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {\n"+
					"    return invalid(\"body must contain exactly one JSON object\")\n"+
					"}"),
			text("decode-scenario-read",
				"Four moves, each earning its line. ``http.MaxBytesReader`` bounds the read before "+
					"any parsing happens — a scenario patch is a couple of fields, so a body past "+
					"four kilobytes is broken or hostile, and the bound also defeats the "+
					"decompression-bomb class of abuse for free. Decoding into ``json.RawMessage`` "+
					"first captures the value without interpreting it, which makes the SECOND decode "+
					"possible: if it does not hit ``io.EOF``, the body held two JSON documents "+
					"(or a value plus stray tokens) and is refused — a body is one document, not a "+
					"stream. Only then does the typed unmarshal run, and every failure — syntax, "+
					"wrong type — becomes the same shaped 422 with the decoder's own reason in "+
					"``details.body``. The handler never sees a half-parsed patch; the store is "+
					"never consulted with garbage."),
			text("decode-strict",
				"One deliberate choice is MISSING from that handler, and its absence is a decision "+
					"worth noticing: unknown keys are ignored (fastapi parity for that endpoint). "+
					"For create-shaped bodies the stricter posture is usually better — a client "+
					"sending ``priority`` to an endpoint with no ``priority`` field is probably "+
					"confused about its contract, and telling it so beats accepting the two fields "+
					"it got right. ``json.Decoder`` has the switch: "+
					"``DisallowUnknownFields``. The validation arena's judged form decodes with "+
					"the full strict posture:"),
			code("decode-strict-feedback", "go",
				"The arena's layer one: the same discipline, plus DisallowUnknownFields.",
				"server/internal/domain/demos/validation_arena.go",
				"func DecodeStrictFeedback(body []byte) (FeedbackForm, error) {\n"+
					"    decoder := json.NewDecoder(bytes.NewReader(body))\n"+
					"    decoder.DisallowUnknownFields()\n"+
					"    var form FeedbackForm\n"+
					"    if err := decoder.Decode(&form); err != nil {\n"+
					"        return FeedbackForm{}, err\n"+
					"    }\n"+
					"    // Exactly one JSON value: a second Decode must hit EOF, otherwise the\n"+
					"    // body is not one feedback document.\n"+
					"    var trailing any\n"+
					"    if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {\n"+
					"        return FeedbackForm{}, errBodyNotOneValue\n"+
					"    }\n"+
					"    return form, nil\n"+
					"}"),
			text("decode-strict-read",
				"What this layer catches for free is everything the TYPE SYSTEM can express — and "+
					"in Go that is a lot. A string where ``Score int`` lives is refused with "+
					"``*json.UnmarshalTypeError`` naming the struct, the field, and the expected "+
					"type. An unknown key is refused with ``*json.UnknownFieldError`` naming the "+
					"key. Broken syntax dies before any field exists. And crucially, Go's decoder "+
					"never COERCES: ``\"five\"`` does not become 5, ``\"true\"`` does not become "+
					"true, a numeric string does not slide into an int field. Dynamic-language "+
					"validators spend a whole feature category on coercion (original value in, "+
					"coerced value out); Go deletes the category — the value either already has "+
					"the declared type or the body is refused. The strictness is the validation."),
			list("decode-checklist", false,
				"Bound the body — http.MaxBytesReader before anything parses; oversize input is refusal, not memory pressure.",
				"One JSON value — a second Decode must hit io.EOF; two documents in one body is a protocol error.",
				"Known fields only — DisallowUnknownFields for create-shaped bodies; ignoring unknowns is a per-endpoint choice.",
				"Declared types enforced — the struct's types are rules; a wrong-typed value is refused with the field named.",
				"Null refused explicitly — JSON null decodes into any struct silently; check for it when null is meaningless."),
		),
		// ── 3. Layer two: hand-rolled rules ────────────────────────────
		section("layer-rules", "Layer two: hand-rolled field rules (the backbone)",
			"Everything types cannot say — lengths, ranges, shapes, cross-field logic — said in plain Go.",
			text("rules-pitch",
				"Layer one answers every question a type system can ask. The rest — is the title short "+
					"enough, is the score in range, does the email look like an email, does flag X "+
					"require field Y — is layer two, and in Go that layer is just FUNCTIONS. No "+
					"framework, no annotations, no registry: a function that takes the decoded struct "+
					"and returns its issues. The arena's judged form is a realistic size — four "+
					"fields, four rules, one optional — and its validator is short enough to read "+
					"in one glance, which is precisely the point:"),
			code("rules-form", "go",
				"The judged form: a plausible create body with every rule shape.",
				"server/internal/domain/demos/validation_arena.go",
				"type FeedbackForm struct {\n"+
					"    Title string `json:\"title\"`\n"+
					"    Body  string `json:\"body\"`\n"+
					"    Score int    `json:\"score\"`\n"+
					"    Email string `json:\"email\"`\n"+
					"}\n"+
					"\n"+
					"const (\n"+
					"    maxTitleRunes = 80\n"+
					"    maxBodyRunes  = 280\n"+
					"    minScore      = 1\n"+
					"    maxScore      = 5\n"+
					"})"),
			code("rules-validator", "go",
				"The backbone itself: every rule a readable branch, every failure an issue.",
				"server/internal/domain/demos/validation_arena.go",
				"func ValidateFeedback(form FeedbackForm) []ArenaIssue {\n"+
					"    var issues []ArenaIssue\n"+
					"    title := strings.TrimSpace(form.Title)\n"+
					"    switch {\n"+
					"    case title == \"\":\n"+
					"        issues = append(issues, ArenaIssue{\n"+
					"            Field: \"title\",\n"+
					"            Rule:  \"required — present-but-empty is data, and data can be wrong\",\n"+
					"            Got:   strconv.Quote(form.Title),\n"+
					"        })\n"+
					"    case utf8.RuneCountInString(title) > maxTitleRunes:\n"+
					"        issues = append(issues, ArenaIssue{\n"+
					"            Field: \"title\",\n"+
					"            Rule:  \"at most 80 runes, trimmed\",\n"+
					"            Got:   fmt.Sprintf(\"%d runes\", utf8.RuneCountInString(title)),\n"+
					"        })\n"+
					"    }\n"+
					"    // … the body-length, score-range, and email-shape branches follow the same shape\n"+
					"    return issues\n"+
					"}"),
			text("rules-collector",
				"Three design decisions in that function carry most of its quality. First, it "+
					"COLLECTS: every branch appends an issue instead of returning the first, so a "+
					"submission with three broken fields gets one 422 naming all three — the caller "+
					"fixes everything in one round trip instead of discovering failures one at a "+
					"time. Second, each issue is STRUCTURED — field, rule, got — not a formatted "+
					"string; that structure becomes the ``details`` array of the envelope, and a "+
					"client can render it per field without parsing prose. Third, the rules are "+
					"written against the DECODED value with the same helpers everywhere: "+
					"``strings.TrimSpace`` for what blank means, ``utf8.RuneCountInString`` for "+
					"what length means (runes — human characters — not bytes, so an accented "+
					"title is not silently half its apparent length), plain comparisons for ranges."),
			text("rules-empty-is-data",
				"The empty-string case deserves its own paragraph because it is where naive "+
					"validators go wrong. ``\"title\": \"\"`` is PRESENT — the key exists, the value "+
					"decodes cleanly, the field is a perfectly legal empty string. Empty is data, "+
					"and data can be wrong: the required rule judges it, exactly as it would judge "+
					"whitespace-only (the ``TrimSpace`` in the check). Conflating empty with absent "+
					"produces APIs where clearing a field is impossible and PATCH semantics break "+
					"quietly. The distinction runs through this whole stack: the client's DTO "+
					"layer has a ``requiredText`` validator precisely because present-but-empty "+
					"must pass transport shape checks on its way to the rule that judges it — "+
					"absence and emptiness are different facts that fail (or pass) for different "+
					"reasons."),
			text("rules-zero-blur",
				"The mirror problem is harder: for VALUE types, ``encoding/json`` cannot tell ABSENT "+
					"from EXPLICIT ZERO. A missing ``score`` decodes to the same ``0`` as "+
					"``\"score\": 0`` — there is no third state. The arena's range check catches "+
					"both (zero is out of 1–5 anyway, a nice accident of that domain), but when "+
					"zero is LEGAL, the blur bites and you must choose a sharper representation. "+
					"This repo's real answer lives in the scenario patch, where absence MEANS "+
					"\"leave unchanged\" and every value including false is meaningful — so the "+
					"fields are POINTERS, and a present-but-wrong-typed value fails decoding "+
					"rather than silently meaning unset:"),
			code("rules-scenario-patch", "go",
				"Present-only semantics: pointer fields make absence explicit.",
				"server/internal/core/scenario.go",
				"type ScenarioPatch struct {\n"+
					"    DataMode      *DataMode         `json:\"dataMode,omitempty\"`\n"+
					"    InjectFailure *FailureInjection `json:\"injectFailure,omitempty\"`\n"+
					"}"),
			text("rules-zero-blur-read",
				"A nil pointer means the key was absent; a non-nil pointer means present, whatever "+
					"its value. That is the whole trick, and it is the standard Go idiom for "+
					"optional request fields — ``*int`` when 0 is real, ``*bool`` when false is "+
					"real, ``*string`` when \"\" is real. The cost is dereference ceremony at the "+
					"use site, which is the honest price of three-state semantics over two-state "+
					"values. Rules of thumb: if absence and zero can mean the same thing in your "+
					"domain, use the plain value and let the zero value be the default (the "+
					"arena's optional ``Body`` does exactly this — absent decodes to \"\", which "+
					"IS the default). If they must be distinguished, use a pointer and validate "+
					"the pointed-through value when present. Reserve fancier machinery "+
					"(``json.RawMessage`` fields, custom unmarshaling) for the rare fields that "+
					"need their own parsing protocol."),
		),
		// ── 4. Layer three: struct tags and the ecosystem ──────────────
		section("layer-library", "Layer three: struct tags and the validator ecosystem",
			"What the library solution looks like, what it buys, and what it charges — taught by comparison, not adopted.",
			text("tags-pitch",
				"The ecosystem's center of gravity is ``github.com/go-playground/validator/v10`` — "+
					"the library you will meet in most Go codebases — and its idea is to move layer "+
					"two's rules into the one place layer one already lives: the struct tag. Where "+
					"this repo writes a ``switch`` and an ``append``, the library writes "+
					"``validate:\"required,max=80\"`` and does the rest by reflection at run time. "+
					"The arena carries the annotated counterpart of its form as a teaching "+
					"exhibit — INERT by design, nothing reads it, no dependency enters the "+
					"module — so the two styles can be compared line by line on the same contract:"),
			code("tags-exhibit", "go",
				"The same contract as tag data — inert here, alive only under a library that reads it.",
				"server/internal/domain/demos/validation_arena.go",
				"type taggedFeedback struct {\n"+
					"    Title string `json:\"title\" validate:\"required,max=80\"`\n"+
					"    Body  string `json:\"body\" validate:\"max=280\"`\n"+
					"    Score int    `json:\"score\" validate:\"required,gte=1,lte=5\"`\n"+
					"    Email string `json:\"email\" validate:\"required,email\"`\n"+
					"}"),
			text("tags-read",
				"Read what the tags actually say, because the semantics are the library's, not Go's. "+
					"``required`` does not mean present — it means NON-ZERO: an empty string, a "+
					"0, a nil slice all fail it, which quietly merges the empty-is-data and "+
					"zero-value-blur distinctions this topic just spent two sections "+
					"separating (and which you re-separate by switching the field to a "+
					"pointer, exactly as before). ``max=80`` on a string counts RUNES; on a "+
					"slice it counts elements; on a number it compares the value — one tag, "+
					"three type-dependent meanings. ``gte=1,lte=5`` is the numeric range, "+
					"``email`` invokes a full RFC-flavored grammar far stricter than the "+
					"arena's one-@ shape check. None of this is discoverable from the "+
					"language; it is a DSL you learn from the library's docs, and every tag "+
					"is checked at run time or — if you mistype it — discovered when a "+
					"request hits it."),
			list("tags-tradeoffs", false,
				"What it buys — a rich built-in vocabulary (lengths, ranges, formats, comparisons, conditionals), declarative next to the field, battle-tested edge cases.",
				"What it charges — a dependency, reflection cost per request, and a tag DSL whose semantics (required = non-zero, rune-counting max) must be learned and remembered.",
				"Error shape — the library returns validation.Errors keyed by field with tag-flavored messages; mapping them onto YOUR envelope's details is still your code.",
				"Custom rules — RegisterValidation adds your functions as new tags; the cross-field logic that is trivial in plain Go becomes struct-level validators.",
				"When it earns its keep — many endpoints, homogeneous CRUD-ish rules, a team that already knows the DSL; the tag density stays low and the semantics stay standard."),
			text("tags-honesty",
				"This guide hand-rolls, and the exhibit above is labeled inert for a reason: the "+
					"honest argument for the library is not that tags are better code — they are "+
					"less code — it is BUDGET. A validator library is one dependency paying rent "+
					"across dozens of endpoints by replacing boilerplate you would otherwise "+
					"copy-paste badly. Against that rent stands everything this repo values: "+
					"rules that compile (rename a field and the hand-rolled validator breaks "+
					"the BUILD, while the tag silently stops matching), errors you shape "+
					"yourself, zero reflection on the hot path, and semantics with no DSL to "+
					"forget. At this guide's scale — a handful of request shapes, each taught "+
					"explicitly — the hand-rolled layer wins on every axis that matters. At "+
					"CRUD-farm scale, reasonable engineers choose the library; now you know "+
					"exactly what they are trading."),
		),
		// ── 5. Custom validators and cross-field rules ─────────────────
		section("custom-validators", "Custom validators and cross-field rules",
			"The rules that cannot hang off one field: vocabularies, cross-field logic, and where they live.",
			text("custom-pitch",
				"Sooner or later a rule refuses to hang off a single field: the state must be one of "+
					"three words, a flag requires at least one tag, an end date must postdate the "+
					"start. In the hand-rolled layer these are not a special KIND of rule — they "+
					"are just branches in the same function, reading two fields instead of one, "+
					"or a helper the rule calls. The arena's email check is the smallest real "+
					"example of a custom rule: no library tag was consulted, ``strings.Cut`` and "+
					"three comparisons decided what shape means, and the rule's sentence rides "+
					"in the issue for the client to read. Vocabulary rules get their own "+
					"treatment in this repo — the closed set lives WITH the type:"),
			text("custom-scenario-intro",
				"The scenario store's update path is the production-grade example, because it "+
					"defends an invariant the HTTP layer cannot fully own: a patch may arrive "+
					"from untyped callers too, so the store re-validates the MERGED result "+
					"itself — a cross-field discipline in miniature (merge the patch, then judge "+
					"the whole):"),
			code("custom-store-update", "go",
				"Cross-field validation at the state boundary — with the expected vocabulary in details.",
				"server/internal/core/scenario.go",
				"func (s *ScenarioStore) Update(patch ScenarioPatch) (ScenarioState, error) {\n"+
					"    // …\n"+
					"    if patch.DataMode != nil && !patch.DataMode.Valid() {\n"+
					"        return s.current, ValidationFailed(\n"+
					"            \"Invalid scenario patch: unknown data mode.\",\n"+
					"            map[string]any{\"dataMode\": string(*patch.DataMode), \"expected\": []string{string(DataModeFake), string(DataModeLive)}},\n"+
					"        )\n"+
					"    }"),
			text("custom-scenario-read",
				"Two details lift this above a bare ``if bad``. The rule runs only when the field "+
					"is PRESENT (non-nil pointer), honoring the patch's present-only semantics "+
					"from the previous section — validating an absent field would make \"leave "+
					"unchanged\" impossible. And the error's ``details`` carries not just what "+
					"was wrong but what was EXPECTED: the legal values, as a slice a client can "+
					"render as options. That is the difference between a refusal and a "+
					"correction; a 422 that ships the vocabulary teaches the caller the "+
					"contract in the very act of enforcing it. The same function then merges "+
					"and stores only if every field survived — validation, mutation, and "+
					"error reporting in one honest sequence."),
			text("custom-library-side",
				"In the tag ecosystem the same needs appear as ``RegisterValidation`` — your "+
					"function, registered under a new tag name — and struct-level validators for "+
					"cross-field rules, invoked with a special tag on the whole struct. It "+
					"works, and mature codebases use it heavily; note only what it costs "+
					"relative to the plain branch: the rule's logic moves away from the "+
					"fields it reads, registration happens far from the struct (usually in "+
					"``main``), and a typo in the tag's name surfaces at run time. The "+
					"hand-rolled cross-field rule sits exactly where its inputs live, fails "+
					"to compile when its fields change shape, and needs no ceremony to test "+
					"— the same trade the whole layer makes, one rule larger."),
		),
		// ── 6. Choosing an approach ────────────────────────────────────
		section("choosing", "Choosing an approach",
			"Layers one and two by default; the library when its budget beats your boilerplate — and the arena to see it all run.",
			text("choose-pitch",
				"The decision is less ``which framework`` than ``where does each rule live``, and "+
					"the answer is mostly mechanical once you stop treating validation as one "+
					"thing. Every rule that a type can express belongs to layer one — it is "+
					"free, compiled, and impossible to forget on a new endpoint if the decode "+
					"helper is shared. Everything else starts life as a hand-rolled branch "+
					"next to the struct it judges, because that is where it is cheapest to "+
					"read, test, and change. The library question arrives only when the "+
					"SECOND boring CRUD endpoint copies the first one's validator — that is "+
					"the moment tags start paying rent, and not before. Condensed:"),
			list("choose-rules", false,
				"Decode discipline always — bound the body, one value, declared types; it is shared helper code, not per-endpoint effort.",
				"Hand-rolled by default — every rule types cannot state, as plain branches that collect structured issues.",
				"Pointers for three-state fields — when absent, empty, and zero must be distinct facts, represent them distinctly.",
				"Vocabularies on the type — closed sets live with the type as a Valid method, and 422 details ship the expected list.",
				"Library at scale — adopt when homogeneous endpoints multiply and the team knows the DSL; budget the dependency and the run-time semantics deliberately."),
			text("choose-composed-intro",
				"And when the layers are chosen, they compose into one sequence — this is "+
					"the entire per-request pipeline of the arena, the same shape a real "+
					"handler follows: decode first, and only a decoded value earns the "+
					"rules' attention:"),
			code("choose-composed", "go",
				"The layers composed: decode, then rules — exactly what a handler runs.",
				"server/internal/domain/demos/validation_arena.go",
				"form, err := DecodeStrictFeedback([]byte(spec.body))\n"+
					"    if err != nil {\n"+
					"        candidate.Decode = ArenaDecodeOutcome{\n"+
					"            Status:    decodeRejected,\n"+
					"            ErrorType: fmt.Sprintf(\"%T\", err),\n"+
					"            Error:     err.Error(),\n"+
					"        }\n"+
					"        candidate.Validate = ArenaRulesOutcome{Status: rulesSkipped}\n"+
					"        // … the decode refusal composes its 422 and returns here\n"+
					"    }\n"+
					"    candidate.Decode = ArenaDecodeOutcome{Status: decodeAccepted}\n"+
					"    issues := ValidateFeedback(form)"),
			text("choose-demo-intro",
				"Everything this topic has claimed is running RIGHT NOW behind the demo below. "+
					"The arena holds twelve candidate bodies — chosen at build time, so the "+
					"payload is deterministic — and every candidate was genuinely run through "+
					"the strict decoder and the hand-rolled rules you read above; every 422 "+
					"body on display was produced by this server's own central error "+
					"writer, not composed for display. Compare rows: which layer refuses, "+
					"with what error, and what the wire answers."),
			demo("validation:arena", "The validation arena",
				"Twelve candidate bodies judged by the real two-layer pipeline: decode discipline, "+
					"hand-rolled rules, and the composed 422 envelopes — plus the tag-ecosystem "+
					"comparison and the envelope's anatomy."),
			text("choose-demo-read",
				"Read the arena in two passes. Down the rows: the happy baseline, four decode "+
					"refusals (syntax, wrong type, unknown field, trailing document — each "+
					"carrying the decoder's own error in the details), five rule catches "+
					"(empty-required, too-long, negative, bad-email, and the zero-value blur), "+
					"the optional-absent row that validates with its default, and the finale "+
					"that breaks three fields at once and reports all three. Across the "+
					"columns: the same body, two verdicts, one response — the layering this "+
					"topic teaches, frozen into data you can diff. The comparison face pairs "+
					"each tag with the rule that does its job here, and the envelope face "+
					"dissects the 422 shape field by field — the subject of the last section "+
					"before the terminal."),
		),
		// ── 7. The Go error-envelope shape for 422s ────────────────────
		section("envelope-422", "The Go error-envelope shape for 422s",
			"One typed error kind, one central writer, one body shape — for every validation refusal.",
			text("env-pitch",
				"Validation only counts if the refusal is SHAPELY, and in Go that story starts "+
					"with the error taxonomy, not with JSON. A validation failure is one KIND "+
					"of failure among eight this app recognizes — a typed value the whole "+
					"stack can branch on with ``errors.Is``, not a string to compare. The "+
					"kind carries the HTTP status by table, so no handler ever picks 422 "+
					"itself:"),
			code("env-kind", "go",
				"The taxonomy's constructor: kind, message, and structured details in one value.",
				"server/internal/core/errors.go",
				"const (\n"+
					"    KindValidationFailed   Kind = \"validation_failed\"   // 422 (family convention; the express sibling uses 400)\n"+
					"    // …\n"+
					")\n"+
					"\n"+
					"// ValidationFailed builds the 422 kind with structured details naming what\n"+
					"// failed (details may be nil).\n"+
					"func ValidationFailed(message string, details any) *Error {\n"+
					"    return &Error{Kind: KindValidationFailed, Message: message, Details: details}\n"+
					"}"),
			text("env-kind-read",
				"``ValidationFailed`` takes ``details any`` — and that ``any`` is deliberate "+
					"freedom with deliberate discipline: this repo ships exactly two shapes, "+
					"chosen by which layer refused. Decode refusals carry "+
					"``map[string]string{\"body\": reason}`` (the decoder's own words); rule "+
					"refusals carry the issue array — field, rule, got — a client renders "+
					"per field. The envelope section of the arena shows both live. What the "+
					"constructor never does is build JSON itself: it builds a VALUE, and "+
					"exactly one function in the process turns values into responses."),
			code("env-body", "go",
				"The wire shape: camelCase, details omitted when absent — never null.",
				"server/internal/core/envelope.go",
				"type errorBody struct {\n"+
					"    Code      string `json:\"code\"`\n"+
					"    Message   string `json:\"message\"`\n"+
					"    RequestID string `json:\"requestId\"`\n"+
					"    Details   any    `json:\"details,omitempty\"`\n"+
					"}"),
			code("env-writer", "go",
				"The one place an error becomes a response: headers, status, body, done.",
				"server/internal/core/envelope.go",
				"func writeEnvelope(w http.ResponseWriter, status int, kind Kind, message, requestID string, details any) {\n"+
					"    w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"    w.Header().Set(RequestIDHeader, requestID)\n"+
					"    w.WriteHeader(status)\n"+
					"    body := errorEnvelope{Error: errorBody{\n"+
					"        Code:      string(kind),\n"+
					"        Message:   message,\n"+
					"        RequestID: requestID,\n"+
					"        Details:   details,\n"+
					"    }}"),
			text("env-anatomy",
				"Walk the shape once, because every failure in this app wears it. ``error.code`` "+
					"is the taxonomy's snake_case bucket — ``validation_failed`` — never a "+
					"free-form word, so clients branch on a closed set. ``error.message`` is "+
					"one human sentence, safe to show any user; internals ride the logs, "+
					"never the body. ``error.requestId`` matches the ``x-request-id`` header "+
					"on the same response and the server's warn-level log line — the join "+
					"key that turns \"it failed\" into a debugging session. ``error.details`` "+
					"is whatever structured context the failure carries, OMITTED when absent "+
					"and never ``null`` — the family's optionals rule, so clients test "+
					"presence once and trust it. And the status is 422 by family convention "+
					"(the express sibling answers 400 for the same bucket — a reminder that "+
					"the convention matters more than the number, and must simply be held "+
					"consistently)."),
			text("env-handler-read",
				"The payoff is what handlers look like. The scenario PATCH handler — the one "+
					"whose decode discipline opened this topic — is the complete validation "+
					"lifecycle in eleven lines: parse and shape-check the body, hand the typed "+
					"patch to the store's own validation, map ANY failure through the one "+
					"central writer, and answer 200 with the resulting state only when "+
					"everything survived. No handler in this repo formats an error response "+
					"itself; they build typed errors and let the writer shape them:"),
			code("env-handler", "go",
				"The whole lifecycle: decode, validate, apply — failures mapped by one writer.",
				"server/internal/presentation/scenario.go",
				"func (h *ScenarioPatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"    patch, err := decodeScenarioPatch(w, r)\n"+
					"    if err != nil {\n"+
					"        core.WriteError(w, r, err, h.logger)\n"+
					"        return\n"+
					"    }\n"+
					"    state, err := h.store.Update(patch)\n"+
					"    if err != nil {\n"+
					"        core.WriteError(w, r, err, h.logger)\n"+
					"        return\n"+
					"    }\n"+
					"    writeJSON(w, http.StatusOK, state)\n"+
					"}"),
		),
		// ── 8. Terminal companion ──────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The arena, the envelope, and a real 422 round-trip — from the shell.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands turn the topic into "+
					"something you can SEE. The arena endpoint is deterministic — curl it "+
					"twice and the bytes match — and the scenario PATCH route gives you a "+
					"live 422 vending machine: every malformed body below answers with the "+
					"exact envelope this topic dissected, request id and all. ``python3 -m "+
					"json.tool`` pretty-prints with zero extra tooling:"),
			code("term-arena", "bash", "", "",
				"# the arena: twelve candidates, both layers' verdicts, the composed 422s\n"+
					"curl -s localhost:8100/api/demo/validation/arena | python3 -m json.tool"),
			code("term-envelope-exhibit", "bash", "", "",
				"# the dissected 422: the multi-error candidate's real envelope + its anatomy\n"+
					"curl -s localhost:8100/api/demo/validation/arena | python3 -c \\\n"+
					"  'import json,sys; print(json.dumps(json.load(sys.stdin)[\"envelope\"], indent=2))'"),
			code("term-422-roundtrip", "bash", "", "",
				"# THE 422 round-trip: an unknown enum value, refused with the expected vocabulary\n"+
					"curl -si -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'Content-Type: application/json' -d '{\"dataMode\":\"bogus\"}'"),
			code("term-422-wrong-type", "bash", "", "",
				"# layer one on the wire: a number where a string enum lives → decode refusal\n"+
					"curl -si -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'Content-Type: application/json' -d '{\"dataMode\": 3}'"),
			code("term-422-null", "bash", "", "",
				"# the null trap: JSON null is not a JSON object — refused, store untouched\n"+
					"curl -si -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'Content-Type: application/json' -d 'null'"),
			code("term-405", "bash", "", "",
				"# wrong verb on the arena: truthful 405 + Allow + the shared envelope\n"+
					"curl -si -X POST localhost:8100/api/demo/validation/arena"),
			text("term-coda",
				"Every refusal on this tour wears the same shape — status 422, "+
					"``{error: {code, message, requestId, details}}``, an ``x-request-id`` "+
					"header matching the body and the logs — and every accepted PATCH "+
					"round-trips the full state. That consistency is not decoration: it is "+
					"what lets a client (and a tired on-call engineer at 3 a.m.) trust the "+
					"contract. The next topic zooms out to API design — resources, "+
					"methods, and the error responses that make an API legible — with "+
					"this discipline as its foundation."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"validation",
		"Validation",
		"Input is hostile: the three-layer boundary discipline — decode rigor, hand-rolled "+
			"rules, the tag ecosystem by comparison — and the one 422 envelope that answers "+
			"every refusal.",
		"request-handling",
		10,
		sections,
	)
}
