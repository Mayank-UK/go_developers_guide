package content

import (
	"errors"
	"fmt"
)

// This file is topic 15 grpc (group apis, curriculum order 15), authored
// on the exemplar pattern: prose is the default block, every must-source
// code block quotes a real repo file (the snippet ratchet verifies it),
// the live demo sits INLINE in the section that teaches serving, and the
// topic closes with the bash terminal companion — the demo endpoint tour
// plus the grpcurl invocation shape for the listener the composition root
// wires.

// grpcTopic builds topic 15 — gRPC on the real posts substrate: what gRPC
// is and why Go speaks it natively, designing the contract in .proto, the
// four method kinds, serving with grpc-go (typed errors onto status codes,
// deadlines honored), stubs and channels (bufconn included), status codes
// and deadlines as the error model, protobuf codegen with buf, and where
// gRPC and REST each win — with the four-kind console demo inline.
func grpcTopic() (Topic, error) {
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
		// ── 1. What gRPC is ──────────────────────────────────────────
		section("what-grpc-is", "What gRPC is — and why Go speaks it natively",
			"A contract-first RPC framework over HTTP/2 with protobuf as its language — and Go is one of its home turfs.",
			text("grpc-pitch",
				"gRPC is remote procedure calling with opinions: you define a service in a contract file (protobuf's "+
					"``.proto``), tooling generates a typed client and a typed server interface in every language you "+
					"use, and calls travel as binary protobuf messages over HTTP/2. It was born at Google — the same "+
					"engineering culture that produced Go — and the Go implementation, ``google.golang.org/grpc``, is "+
					"first-class rather than a port: it is idiomatic Go all the way down (``context.Context`` on every "+
					"call, errors as values, interfaces you implement), it is one of the most battle-tested Go "+
					"libraries in existence, and it powers a large share of internal microservice traffic industry-"+
					"wide. When people say a Go service \"speaks gRPC natively\", this is what they mean: the client "+
					"and server halves of a contract are Go code you generated before writing any logic."),
			text("grpc-transport",
				"The transport is where the first real difference from the JSON-over-HTTP APIs this guide has built "+
					"so far shows up. gRPC rides HTTP/2, which multiplexes many concurrent streams over ONE TCP "+
					"connection — a hundred in-flight calls share a connection instead of queueing behind each other "+
					"or opening a socket each — and it frames each stream as compact binary messages with a small "+
					"header, not text. The wire stops being readable-by-accident and becomes a format your tooling "+
					"reads for you, which is a fair trade: you stop hand-parsing payloads and start trusting a "+
					"contract that both ends were generated from. Every call also carries gRPC-native METADATA "+
					"(headers, binary-safe), a STATUS CODE as its error vocabulary, and a DEADLINE that propagates "+
					"from the caller through every hop — three things REST reuses from HTTP, gRPC defines as part "+
					"of the call itself."),
			text("grpc-fit",
				"Where gRPC earns its keep is the space between services. The guide's own posts domain is the honest "+
					"test bed: the same ``internal/domain/posts`` use cases that serve ``/api/posts`` over REST now "+
					"serve ``guide.posts.v1.PostsService`` over gRPC in ``internal/grpcapi`` — one domain, two "+
					"transports, and this topic builds the second one end to end. The toolchain is pinned in the "+
					"module like any other dependency, so the generated code and the plugins that make it are part "+
					"of the reviewable tree:"),
			code("grpc-mod-code", "mod",
				"The pinned protobuf/gRPC toolchain: version-locked tool dependencies, not ad-hoc installs.",
				"server/go.mod",
				"tool (\n"+
					"    github.com/99designs/gqlgen\n"+
					"    github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen\n"+
					"    google.golang.org/grpc/cmd/protoc-gen-go-grpc\n"+
					"    google.golang.org/protobuf/cmd/protoc-gen-go\n"+
					")"),
			text("grpc-pillars",
				"Four ideas carry the whole topic, and each gets a section: the CONTRACT (a .proto file that is the "+
					"single source of truth both ends generate from), the four METHOD KINDS (how many messages "+
					"each direction — everything from a single request/response to two open streams), STATUS CODES "+
					"plus DEADLINES (the error model that travels with every call), and CODEGEN (buf turning the "+
					"contract into Go before your first line of logic). Hold those four and the API surface below "+
					"is just the posts domain wearing a second transport:"),
			list("grpc-pillar-list", false,
				"Contract-first — the .proto file defines the API before any implementation exists; client and server generate from the same source.",
				"HTTP/2 multiplexing — many concurrent calls share one connection as independent streams; no head-of-line queueing per call.",
				"Binary protobuf — compact, typed messages with generated structs in every language; no hand-written DTO parsers.",
				"Status codes + deadlines — every call carries an error vocabulary and a cancellation budget that propagates end to end.",
				"Streaming — four method kinds shape the conversation: one-shot, server-push, client-batch, and full duplex."),
		),
		// ── 2. The contract: .proto ──────────────────────────────────
		section("proto-design", "The contract: designing in .proto",
			"The .proto file is the API's constitution — field numbers are the wire contract, and evolution is a discipline.",
			text("proto-pitch",
				"Design starts in ``server/api/proto/posts.proto``, and the first honest observation is that this "+
					"file plays exactly the role the OpenAPI spec plays in the documentation topic: a machine-"+
					"readable contract that exists BEFORE the implementation, that both ends derive from, and that "+
					"tooling can lint and diff. The header does three jobs in four lines — pins the language "+
					"dialect (``proto3``), names the API (the package, which becomes the method-path prefix on "+
					"the wire and a VERSION promise: a future v2 is a new package, so v1 clients keep working "+
					"while both are served), and tells the Go generator where generated code lives and what its "+
					"package name is:"),
			code("proto-header-code", "proto",
				"The contract's header: dialect, package (versioned), Go import path, and the well-known timestamp type.",
				"server/api/proto/posts.proto",
				"syntax = \"proto3\";\n"+
					"\n"+
					"package guide.posts.v1;\n"+
					"\n"+
					"option go_package = \"guide/internal/grpcapi/pb;pb\";\n"+
					"\n"+
					"import \"google/protobuf/timestamp.proto\";"),
			text("proto-numbers",
				"Then the discipline that protobuf veterans burn into newcomers: FIELD NUMBERS ARE THE CONTRACT, "+
					"not the field names. On the wire every field is encoded as its number and a wire type — the "+
					"name does not travel at all — so renaming ``title`` to ``heading`` is wire-compatible (old "+
					"messages still decode), while renumbering fields, or reusing a retired number for a new "+
					"field, silently corrupts every message ever serialized. The contract therefore records "+
					"removed fields as BURNED, with ``reserved`` claiming both the numbers and the names so a "+
					"future edit cannot quietly resurrect them — exactly the evolution story the Post message "+
					"teaches, with numbers 7–11 retired and unreusable forever:"),
			code("proto-post-code", "proto",
				"The Post message: numbers are identity, reserved burns retired fields, timestamps use the well-known type.",
				"server/api/proto/posts.proto",
				"message Post {\n"+
					"  string id = 1;                                 // \"p-3\" — stable identity\n"+
					"  string title = 2;                              // 1–120 runes (validated server-side)\n"+
					"  string body = 3;                               // 1–5000 runes (validated server-side)\n"+
					"  repeated string tags = 4;                      // normalized on write: trimmed, lowercase\n"+
					"  google.protobuf.Timestamp created_at = 5;      // set by storage, UTC\n"+
					"  google.protobuf.Timestamp updated_at = 6;      // moves on every write\n"+
					"\n"+
					"  reserved 7 to 11;\n"+
					"  reserved \"subtitle\", \"author\", \"status\", \"weight\", \"slug\";\n"+
					"}"),
			text("proto-zero-values",
				"proto3's value model surprises people coming from JSON schemas: scalars have NO required, and a "+
					"field set to its zero value (empty string, zero number, false) is simply absent from the "+
					"serialized bytes. An empty ``id`` therefore is not an error the schema can express — it is a "+
					"perfectly valid message, and PRESENT-BUT-EMPTY IS DATA, the same stance this guide's REST "+
					"API takes with its 422s. The consequence is architectural: validation lives at the boundary "+
					"in the SERVICE, which answers an empty id with ``codes.InvalidArgument`` rather than hoping "+
					"the schema catches it. Timestamps avoid the string-vs-instant trap by importing the well-"+
					"known ``google/protobuf.Timestamp`` — generated Go code gets ``*timestamppb.Timestamp`` with "+
					"a one-call conversion to ``time.Time``, and every client language knows the type instead of "+
					"negotiating an ISO-8601 dialect. One more numbering rule rides along for free in this file: "+
					"numbers scope PER MESSAGE — ``CreatePostRequest.title`` is 1 while ``Post.title`` is 2, and "+
					"nothing collides, which is why every new message starts fresh at 1."),
		),
		// ── 3. The four method kinds ─────────────────────────────────
		section("four-stream-kinds", "The four method kinds",
			"Unary, server-streaming, client-streaming, bidirectional — the keyword stream decides how many messages flow which way.",
			text("kinds-pitch",
				"Everything a gRPC method can be is encoded by where the word ``stream`` appears in its "+
					"declaration: no ``stream`` means unary (one request message, one response message); "+
					"``stream`` before the return type means the SERVER sends many messages; before the "+
					"parameter, the CLIENT sends many; on both sides, both do. That is the entire taxonomy, and "+
					"it is worth internalizing because every other difference (flow control, error timing, "+
					"message framing) follows from it. The PostsService deliberately implements all four on one "+
					"honest resource, so each kind below is a real method you can call, not a slide:"),
			code("kinds-service-code", "proto",
				"One service, all four kinds: unary fetch, server-streamed pages, client-streamed batches, bidirectional tag sync.",
				"server/api/proto/posts.proto",
				"service PostsService {\n"+
					"  rpc GetPost(PostRequest) returns (Post);\n"+
					"\n"+
					"  rpc ListPosts(ListRequest) returns (stream PostListItem);\n"+
					"\n"+
					"  rpc CreatePosts(stream CreatePostRequest) returns (CreateSummary);\n"+
					"\n"+
					"  rpc SyncTags(stream TagUpdate) returns (stream SyncAck);\n"+
					"}"),
			text("kinds-server-stream",
				"SERVER streaming (``ListPosts``) is one request answered by a stream of messages that the "+
					"client consumes with plain ``Recv()`` calls until ``io.EOF`` — the shape for pages, logs, "+
					"tail -f feeds, and anything that would otherwise force a client to poll. Pagination here is "+
					"CURSOR-based, the gRPC-idiomatic counterpart of the REST API's page numbers: each page's "+
					"final item carries an opaque ``next_page_token`` that the client hands back unmodified to "+
					"fetch the next page, and an empty token means the collection is exhausted. \"Opaque\" is a "+
					"contract, not a courtesy — the server happens to encode a page number today and could encode "+
					"an offset-plus-filter hash tomorrow, and no correct client would notice:"),
			code("kinds-list-code", "go",
				"The server-streaming handler: normalize the request through the domain, stream items, cursor the last.",
				"server/internal/grpcapi/server.go",
				"result, err := s.list.Execute(ctx, query)\n"+
					"    if err != nil {\n"+
					"        return statusFromDomain(err)\n"+
					"    }\n"+
					"    for i, post := range result.Items {\n"+
					"        if err := ctx.Err(); err != nil {\n"+
					"            return status.FromContextError(err).Err()\n"+
					"        }\n"+
					"        item := listItemProto(post)\n"+
					"        if i == len(result.Items)-1 && result.HasNext {\n"+
					"            item.NextPageToken = strconv.Itoa(result.Page + 1)\n"+
					"        }\n"+
					"        if err := stream.Send(item); err != nil {\n"+
					"            return err // the client is gone; the transport's error is the answer\n"+
					"        }\n"+
					"    }\n"+
					"    return nil"),
			text("kinds-client-stream",
				"CLIENT streaming (``CreatePosts``) inverts the arrows: the client sends many messages and "+
					"gets ONE answer — a ``CreateSummary`` — after it half-closes its side of the stream. This "+
					"is the batch shape: bulk imports, mobile upload queues, and any writer that would otherwise "+
					"pay a full round trip per item. The handler loop is the whole pattern — ``Recv()`` until "+
					"``io.EOF``, working each message as it arrives, then one ``SendAndClose``. The failure "+
					"semantics are a design DECISION this API makes explicitly: the first invalid draft fails the "+
					"entire RPC with ``codes.InvalidArgument`` and no summary is ever sent, because one RPC "+
					"carries exactly one status — an API that wants partial-failure reporting must design it into "+
					"its messages, which is precisely what the next kind does:"),
			code("kinds-create-code", "go",
				"The client-streaming handler: Recv until EOF, store as drafts arrive, one SendAndClose summary.",
				"server/internal/grpcapi/server.go",
				"req, err := stream.Recv()\n"+
					"        if errors.Is(err, io.EOF) {\n"+
					"            return stream.SendAndClose(&pb.CreateSummary{\n"+
					"                Received: received,\n"+
					"                Created:  created,\n"+
					"                Ids:      ids,\n"+
					"            })\n"+
					"        }\n"+
					"        if err != nil {\n"+
					"            return err // transport failure or client cancellation\n"+
					"        }\n"+
					"        received++\n"+
					"        post, err := s.create.Execute(ctx, posts.Draft{\n"+
					"            Title: req.GetTitle(),\n"+
					"            Body:  req.GetBody(),\n"+
					"            Tags:  req.GetTags(),\n"+
					"        })\n"+
					"        if err != nil {\n"+
					"            return statusFromDomain(err)\n"+
					"        }"),
			text("kinds-bidi",
				"BIDIRECTIONAL streaming (``SyncTags``) gives both sides an open stream on the same call — the "+
					"editor session syncing tag edits as they happen is the model: send an update whenever one "+
					"exists, receive an ack whenever one arrives, close when done. Here the failure semantics "+
					"are the deliberate CONTRAST with the batch: a bad update (unknown post id, invalid tags) "+
					"does NOT fail the RPC — it draws an ack with ``applied=false`` plus a note, and the stream "+
					"keeps going, because tag sync wants \"keep going, tell me which failed\" while batch create "+
					"wanted all-or-nothing. Same transport, different protocol, and the choice lives in the "+
					"MESSAGE DESIGN, not in gRPC:"),
			code("kinds-sync-code", "go",
				"The bidi handler: read one update, ack it, repeat — rejections are data in acks, not stream deaths.",
				"server/internal/grpcapi/server.go",
				"for {\n"+
					"        if err := ctx.Err(); err != nil {\n"+
					"            return status.FromContextError(err).Err()\n"+
					"        }\n"+
					"        update, err := stream.Recv()\n"+
					"        if errors.Is(err, io.EOF) {\n"+
					"            return nil // client half-closed; every update has been acked\n"+
					"        }\n"+
					"        if err != nil {\n"+
					"            return err\n"+
					"        }\n"+
					"        ack, err := s.ackOne(ctx, update)\n"+
					"        if err != nil {\n"+
					"            return err\n"+
					"        }\n"+
					"        if err := stream.Send(ack); err != nil {\n"+
					"            return err\n"+
					"        }\n"+
					"    }"),
			list("kinds-summary-list", false,
				"Unary — one in, one out; the method call shape: GetPost(PostRequest) returns (Post).",
				"Server-stream — one in, many out; Recv until io.EOF: pages, feeds, tail -f over RPC.",
				"Client-stream — many in, one out; batch writes answered by a single summary.",
				"Bidi — many both ways; per-item acks or live dialogue on one long-lived call.",
				"Failure design — one RPC has exactly one status; partial failure must be designed into messages (SyncAck.applied), not improvised.",
				"Cancellation — every kind watches ctx between messages; a canceled call ends for both sides."),
		),
		// ── 4. Serving with grpc-go ──────────────────────────────────
		section("serving-with-grpc-go", "Serving with grpc-go",
			"Implement the generated interface, map the error taxonomy onto status codes, register, listen — the composition root's shape.",
			text("serve-pitch",
				"Serving means implementing the interface the generator emitted: ``pb.PostsServiceServer``, one "+
					"method per rpc. The implementation is a struct holding the posts use cases — the SAME "+
					"instances the REST handlers hold — and each method is a thin translation: protobuf message "+
					"in, domain call, protobuf message or status error out. That makes this file the gRPC "+
					"counterpart of a handler layer, right down to embedding the generated "+
					"``UnimplementedPostsServiceServer`` for forward compatibility: add a method to the proto "+
					"tomorrow, regenerate, and unwired methods answer ``codes.Unimplemented`` instead of "+
					"breaking the build. The unary method shows the whole shape in miniature — boundary "+
					"validation first (the empty id the schema could not express), one use case call, one "+
					"mapping on failure:"),
			code("serve-getpost-code", "go",
				"The unary handler: validate the boundary, call the use case, map the failure.",
				"server/internal/grpcapi/server.go",
				"func (s *PostsServer) GetPost(ctx context.Context, req *pb.PostRequest) (*pb.Post, error) {\n"+
					"    if req.GetId() == \"\" {\n"+
					"        return nil, status.Error(\n"+
					"            codes.InvalidArgument,\n"+
					"            \"id is required — present-but-empty is data, and data can be wrong\",\n"+
					"        )\n"+
					"    }\n"+
					"    if req.GetId() == SlowPostID {\n"+
					"        return s.slowGetPost(ctx)\n"+
					"    }\n"+
					"    post, err := s.get.Execute(ctx, req.GetId())\n"+
					"    if err != nil {\n"+
					"        return nil, statusFromDomain(err)\n"+
					"    }\n"+
					"    return postProto(post), nil\n"+
					"}"),
			text("serve-errors",
				"The mapping is one function, and it is the gRPC twin of the REST writer ``core.WriteError``: "+
					"the app's eight-kind typed taxonomy becomes grpc status codes at a single seam, so no "+
					"method invents its own translation. The pairs are worth memorizing because clients branch "+
					"on them: a miss is ``codes.NotFound``, a broken payload is ``codes.InvalidArgument``, a "+
					"conflict is ``codes.AlreadyExists``, and anything unrecognized degrades to "+
					"``codes.Internal`` with NO internals in the message — the same degradation the HTTP "+
					"writer applies at 500. Both transports now speak the same semantics with two vocabularies, "+
					"and the demo below proves the miss case live:"),
			code("serve-mapping-code", "go",
				"The taxonomy → status seam: one place translates core.Error kinds into codes.",
				"server/internal/grpcapi/server.go",
				"var typed *core.Error\n"+
					"    if !errors.As(err, &typed) {\n"+
					"        return status.Error(codes.Internal, \"internal error\")\n"+
					"    }\n"+
					"    var code codes.Code\n"+
					"    switch typed.Kind {\n"+
					"    case core.KindValidationFailed:\n"+
					"        code = codes.InvalidArgument // 3 — the caller's message was wrong\n"+
					"    case core.KindNotFound:\n"+
					"        code = codes.NotFound // 5 — the named thing is not there\n"+
					"    // …unauthenticated, permission denied, already exists, exhausted, unavailable follow\n"+
					"    default:\n"+
					"        code = codes.Internal // 13 — our fault, no specifics on the wire\n"+
					"    }\n"+
					"    return status.Error(code, typed.Message)"),
			text("serve-wiring",
				"Registration and listening are deliberately separate steps. ``Register`` takes any "+
					"``grpc.ServiceRegistrar`` — a production server or a test harness — and the constructor "+
					"builds the complete ``*grpc.Server`` with the posts service registered, leaving TLS, "+
					"keepalive, and interceptors (gRPC's middleware — the authentication and observability "+
					"topics return to them) as options the caller passes. The composition root wires it on "+
					"the guide's reserved port; until that listener lands, the demo and the tests drive the "+
					"identical constructor over an in-memory listener:"),
			code("serve-constructor-code", "go",
				"The constructor the composition root wires: one call, one complete gRPC server.",
				"server/internal/grpcapi/server.go",
				"func NewGRPCServer(\n"+
					"    list *posts.ListPostsUseCase,\n"+
					"    get *posts.GetPostUseCase,\n"+
					"    create *posts.CreatePostUseCase,\n"+
					"    replace *posts.ReplacePostUseCase,\n"+
					"    opts ...grpc.ServerOption,\n"+
					") *grpc.Server {\n"+
					"    server := grpc.NewServer(opts...)\n"+
					"    NewPostsServer(list, get, create, replace).Register(server)\n"+
					"    return server\n"+
					"}"),
			text("serve-demo-intro",
				"The console below is not a narration — the server really runs. Each time the demo endpoint is "+
					"fetched, the builder constructs a fresh in-process stack (scratch repository, "+
					"``NewGRPCServer``, an in-memory bufconn listener), dials it with a real generated stub, and "+
					"walks the exhibit script: all four method kinds with real results, one ``GetPost`` miss "+
					"whose ``NotFound`` status and the server's own message are captured as data, a "+
					"three-draft ``CreatePosts`` batch answered by one summary (ids p-7, p-8, p-9 in arrival "+
					"order), a ``SyncTags`` session whose unknown-post rejection is acked per item while the "+
					"stream survives, a two-round-trip ``ListPosts`` walk by page token — and a 150ms deadline "+
					"that beats the slow-path exhibit's simulated 10 seconds of work, observed as "+
					"``DeadlineExceeded`` deterministically. Codes are reported as data, never as timing text:"),
			demo("api:grpc-console", "The gRPC console",
				"Real RPCs, run in-process over bufconn: the four method kinds on the posts service — unary "+
					"hit and miss, server-streamed pages walked by opaque token, client-streamed batch with "+
					"its single summary, bidirectional tag sync with per-item acks — plus a deadline case that "+
					"ends in codes.DeadlineExceeded. Status codes ride every report row as the contract's error "+
					"vocabulary."),
			text("serve-demo-read",
				"Read the console with the section above it in mind and every row names its lesson: the two "+
					"unary reports are the same method twice, once answering a post and once answering a "+
					"STATUS — the miss's row carries ``NotFound (5)`` and the server's message verbatim, which "+
					"is the taxonomy mapping you just read, observed from the client side. The list report's "+
					"two round trips show the cursor protocol: page 1 returns three items and token ``\"2\"``, "+
					"page 2 returns the partial remainder and an empty cursor. The create report's single "+
					"summary and the sync report's surviving stream are the two failure designs side by side — "+
					"all-or-nothing versus per-item acks — and the deadline report is the coda: the same unary "+
					"method, a tighter context, a different status."),
		),
		// ── 5. Stubs and channels ────────────────────────────────────
		section("stubs-and-channels", "Stubs and channels: calling from Go",
			"A channel is the connection abstraction, a stub is generated sugar over it — and bufconn lets tests dial a real server with no port.",
			text("stubs-pitch",
				"The client side has two words to hold. A CHANNEL (``*grpc.ClientConn``) is the connection "+
					"abstraction: you create it with an address and options, it manages name resolution, "+
					"HTTP/2 setup, reconnects, and load balancing across the addresses it resolves — and it "+
					"is LAZY, connecting on first call rather than at construction, so creating one is cheap "+
					"and safe at startup. A STUB is what the generator makes from the channel: "+
					"``pb.NewPostsServiceClient(conn)`` returns a struct with a typed method per rpc — unary "+
					"methods take ``context.Context`` and the request message; streaming methods return a "+
					"stream object with ``Send``/``Recv``/``CloseSend``. Application code holds the stub for "+
					"the service it calls and never touches the channel again; one channel can serve many "+
					"stubs."),
			text("stubs-dial",
				"Dialing has exactly two required decisions, and both are visible in the demo's dialer. "+
					"First, CREDENTIALS must be explicit — grpc refuses an implicit plaintext client, which "+
					"forces \"insecure\" to be a word you typed on purpose (production wires TLS here, the "+
					"same way the HTTPS story goes for REST). Second, the RESOLVER: ``passthrough:///`` hands "+
					"the address straight to a custom dialer instead of DNS, which is how the demo connects "+
					"to a ``bufconn`` listener — an in-memory pipe grpc-go ships precisely for in-process "+
					"testing, where the RPCs are real HTTP/2 but there is no socket and no port to collide "+
					"with anything:"),
			code("stubs-dial-code", "go",
				"The demo's dial: explicit plaintext credentials and a context dialer over bufconn.",
				"server/internal/domain/demos/grpc_console.go",
				"conn, err := grpc.NewClient(\n"+
					"        \"passthrough:///bufnet\",\n"+
					"        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {\n"+
					"            return listener.DialContext(ctx)\n"+
					"        }),\n"+
					"        grpc.WithTransportCredentials(insecure.NewCredentials()),\n"+
					"    )"),
			text("stubs-call",
				"Calling is deliberately anticlimactic — that is the point of generated stubs. The demo's "+
					"unary report is one function call with a context and a message; the deadline report is "+
					"the same call with a tighter context, and grpc does the rest: the deadline rides the "+
					"call's metadata, the server's context carries it, and an expired budget surfaces as a "+
					"status CODE on the client, not a timeout exception to catch. For streams the same stub "+
					"returns the stream object; the demo's list walker is then ``Recv()`` in a loop until "+
					"``io.EOF``, reading the cursor off the last message of each page — the exact protocol "+
					"the server side implements:"),
			code("stubs-call-code", "go",
				"A unary call and its deadline twin: same stub, same method, tighter context.",
				"server/internal/domain/demos/grpc_console.go",
				"post, err := client.GetPost(ctx, &pb.PostRequest{Id: id})\n"+
					"    codeName, codeNumber := callStatus(err)\n"+
					"    // …the hit path builds its report rows from post below\n"+
					"\n"+
					"    callCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)\n"+
					"    defer cancel()\n"+
					"\n"+
					"    _, err := client.GetPost(callCtx, &pb.PostRequest{Id: grpcapi.SlowPostID})"),
			text("stubs-interceptors",
				"Channels and stubs are also where gRPC's middleware lives: INTERCEPTORS are functions you "+
					"register on the channel (client side) or the server (the ``grpc.ServerOption`` the "+
					"constructor accepts), wrapping every call to log, retry, authenticate, or enforce "+
					"budgets — the exact role the request-id and logging middleware play in the HTTP stack, "+
					"one layer down. The guide leaves them unwired for now and returns to them in the "+
					"observability topic; what matters here is the seam EXISTS on both ends, takes the same "+
					"per-call context every handler already honors, and costs nothing until used."),
		),
		// ── 6. Status codes and deadlines ────────────────────────────
		section("status-and-deadlines", "Status codes and deadlines",
			"Codes are the shared error vocabulary; deadlines are the cancellation budget every call carries — and both travel.",
			text("status-pitch",
				"A gRPC call ends in exactly one status: ``OK``, or one of a small closed set of codes that "+
					"both ends, in every language, agree on — the counterpart of HTTP's status codes, minus "+
					"the decades of ambiguity, because the codes were designed for RPC semantics from day "+
					"one. They are pairs a Go engineer should know on sight, and the taxonomy mapping you "+
					"read in the serving section uses exactly these:"),
			list("status-code-list", false,
				"OK (0) — success; err is nil and the response message stands.",
				"InvalidArgument (3) — the caller's message is wrong: empty ids, broken drafts, forged page tokens.",
				"NotFound (5) — the named thing is not there: the honest miss on an unknown post id.",
				"DeadlineExceeded (4) — the call's time budget ran out before the work finished.",
				"Unavailable (14) — the server is not reachable right now; safe to retry with backoff.",
				"Unauthenticated (16) / PermissionDenied (7) — who are you, versus you may not.",
				"Internal (13) — the server's own failure; no internals on the wire, details belong in logs.",
				"Canceled (1) — the caller withdrew (context canceled), distinct from a deadline expiring."),
			text("deadline-pitch",
				"DEADLINES are the other half of the error model, and gRPC's answer is Go-native: the "+
					"deadline is simply the call's ``context.Context``. A caller sets a budget with "+
					"``context.WithTimeout``; grpc transmits it in the call's metadata; the server's handler "+
					"context carries it; any downstream call — storage, another RPC — inherits it through "+
					"Go's context tree. When the budget expires, both ends learn: the client's call returns "+
					"``codes.DeadlineExceeded``, and the server's ``ctx.Done()`` fires so long-running work "+
					"can stop. The rule this guide's server honors: anything that can block must watch "+
					"``ctx`` — which is what the slow-path exhibit exists to prove. It simulates ten "+
					"seconds of work as a timer raced against ``ctx.Done()``; the demo's 150ms deadline "+
					"always wins, deterministically, and the caller learns code 4 in 150ms:"),
			code("deadline-slow-code", "go",
				"The slow path races its work against the caller's deadline — the pattern every blocking handler owes its callers.",
				"server/internal/grpcapi/server.go",
				"timer := time.NewTimer(slowPathDelay)\n"+
					"    defer timer.Stop()\n"+
					"    select {\n"+
					"    case <-ctx.Done():\n"+
					"        return nil, status.FromContextError(ctx.Err()).Err()\n"+
					"    case <-timer.C:\n"+
					"        if err := ctx.Err(); err != nil {\n"+
					"            return nil, status.FromContextError(err).Err()\n"+
					"        }\n"+
					"    }"),
			text("deadline-read",
				"Two details in that snippet carry the lesson. ``status.FromContextError`` converts Go's "+
					"context errors into the right codes — ``DeadlineExceeded`` for an expired budget, "+
					"``Canceled`` for a withdrawn call — because a raw ``ctx.Err()`` string is not a status "+
					"and clients must never parse prose. And the second ``ctx.Err()`` check after the timer "+
					"fires handles the race where the budget expired while the last unit of work finished — "+
					"the caller already gave up, so answering success into a canceled call is a lie. The "+
					"demo pins the observable behavior: deadline in, code 4 out, no timing text anywhere in "+
					"the report — timing is a fact about the world, codes are a fact about the contract."),
		),
		// ── 7. Protobuf codegen with buf ─────────────────────────────
		section("protobuf-codegen-with-buf", "Protobuf codegen with buf",
			"buf is the toolchain around .proto: module config, lint, breaking-change detection, and one-command generation.",
			text("buf-pitch",
				"Historically protobuf work meant juggling ``protoc`` and plugin flags by hand; buf is the "+
					"modern toolchain that wraps it all — a module definition, lint rules, breaking-change "+
					"detection against a base, and generation driven by a config file instead of a shell "+
					"script. The guide pins buf v1.75.0 with the two Go plugins (protoc-gen-go for message "+
					"types, protoc-gen-go-grpc for the service interface and stubs) and does codegen with "+
					"ONE command: ``buf generate`` from ``server/``, no arguments, everything from config. "+
					"The module file says where the protos live and which rules govern them:"),
			code("buf-module-code", "yaml",
				"The buf module: proto root, lint standard (with the guide's two documented exceptions), strict breaking view.",
				"server/buf.yaml",
				"version: v2\n"+
					"modules:\n"+
					"  - path: api/proto\n"+
					"lint:\n"+
					"  use:\n"+
					"    - STANDARD\n"+
					"  except:\n"+
					"    - PACKAGE_DIRECTORY_MATCH\n"+
					"    - RPC_REQUEST_STANDARD_NAME\n"+
					"    - RPC_RESPONSE_STANDARD_NAME\n"+
					"breaking:\n"+
					"  use:\n"+
					"    - FILE"),
			text("buf-lint",
				"The lint configuration teaches by omission: buf's STANDARD set is on, with exactly three "+
					"rules scoped out and each exclusion a recorded DECISION rather than a shrug. "+
					"``PACKAGE_DIRECTORY_MATCH`` would demand ``guide/posts/v1/`` directories mirroring the "+
					"package; a single-service module gains nothing from the tree. The two RPC naming rules "+
					"want per-method ``GetPostRequest``/``GetPostResponse`` types; this API shares request "+
					"and response types across methods deliberately — ``Post`` answers ``GetPost`` and would "+
					"answer a future update the same way. ``buf lint`` runs clean, and ``buf breaking`` "+
					"against a base ref would catch the contract crimes this topic spent its second section "+
					"on — reusing a reserved number, deleting a field without reserving — before review "+
					"ever sees them."),
			code("buf-gen-code", "yaml",
				"The generation config: two local plugins, source-relative output into the committed pb package.",
				"server/buf.gen.yaml",
				"version: v2\n"+
					"plugins:\n"+
					"  - local: protoc-gen-go\n"+
					"    out: internal/grpcapi/pb\n"+
					"    opt: paths=source_relative\n"+
					"  - local: protoc-gen-go-grpc\n"+
					"    out: internal/grpcapi/pb\n"+
					"    opt: paths=source_relative"),
			text("buf-generated",
				"``paths=source_relative`` makes the output tree mirror the proto tree, so generation is "+
					"deterministic and the OUTPUT IS COMMITTED — the generated structs are reviewed like any "+
					"other code, and a build never depends on a tool being installed. Reading the generated "+
					"Post is worth a minute, because it shows the contract in Go clothes: every field gains "+
					"protobuf struct tags (the wire truth), JSON tags (for grpc-gateway and protojson), the "+
					"teaching comments from the .proto ride along, and the state/sizeCache machinery at the "+
					"bottom is protobuf's reflection-and-serialization engine — plus, off-screen, a zero-"+
					"checking ``GetTitle()`` getter per field, which is why Go code never nil-dereferences a "+
					"message field it just met:"),
			code("buf-generated-code", "go",
				"The generated Post struct: protobuf tags are the wire truth, comments ride over from the .proto.",
				"server/internal/grpcapi/pb/posts.pb.go",
				"type Post struct {\n"+
					"    state         protoimpl.MessageState `protogen:\"open.v1\"`\n"+
					"    Id            string                 `protobuf:\"bytes,1,opt,name=id,proto3\" json:\"id,omitempty\"`\n"+
					"    Title         string                 `protobuf:\"bytes,2,opt,name=title,proto3\" json:\"title,omitempty\"`\n"+
					"    Body          string                 `protobuf:\"bytes,3,opt,name=body,proto3\" json:\"body,omitempty\"`\n"+
					"    Tags          []string               `protobuf:\"bytes,4,rep,name=tags,proto3\" json:\"tags,omitempty\"`\n"+
					"    CreatedAt     *timestamppb.Timestamp `protobuf:\"bytes,5,opt,name=created_at,json=createdAt,proto3\" json:\"created_at,omitempty\"`\n"+
					"    UpdatedAt     *timestamppb.Timestamp `protobuf:\"bytes,6,opt,name=updated_at,json=updatedAt,proto3\" json:\"updated_at,omitempty\"`\n"+
					"    unknownFields protoimpl.UnknownFields\n"+
					"    sizeCache     protoimpl.SizeCache\n"+
					"}"),
			text("buf-discipline",
				"The workflow discipline is short and enforced by the repo's shape: edit the .proto, run "+
					"``buf generate``, commit BOTH the contract and the regenerated output in the same "+
					"change — a proto whose generated code is stale is a lie with a diff. The ratchet tests "+
					"this guide already runs keep the topic honest in the other direction: every quoted "+
					"snippet above is checked against the real files, including the generated ones, so the "+
					"teaching cannot drift from the tree. Regeneration from scratch, for the record, is "+
					"``cd server && buf generate`` — the pinned plugins do the rest."),
		),
		// ── 8. gRPC vs REST ──────────────────────────────────────────
		section("grpc-vs-rest", "gRPC vs REST: choosing",
			"Neither wins outright — they solve different halves of the API problem, and production systems run both.",
			text("versus-pitch",
				"After a whole topic of gRPC, the honest close is positioning, not conversion. REST over HTTP/"+
					"1.1 with JSON remains the lingua franca of PUBLIC APIs for reasons that are structural, "+
					"not inertia: every browser, phone, curl, and HTTP proxy speaks it with zero setup; "+
					"payloads are human-readable on the wire; caching, CDNs, and the entire web's "+
					"infrastructure assume its semantics; and a third-party developer needs nothing but an "+
					"HTTP client and a doc page to integrate. gRPC's strengths are the exact complement: "+
					"INTERNAL service-to-service traffic where both ends are yours, contracts can be "+
					"generated rather than documented, the four streaming kinds model long-lived "+
					"conversations HTTP/1.1 fakes with polling, and deadlines propagate through call chains "+
					"instead of dying at each hop."),
			list("versus-list", false,
				"Contracts — gRPC generates typed clients and servers from one .proto; REST documents a contract (OpenAPI) that humans keep honest.",
				"Wire — binary protobuf is compact and cheap to parse; JSON is bigger and text-parseable, and wins on debuggability-by-eyeball.",
				"Transport — HTTP/2 multiplexes many calls over one connection; HTTP/1.1 browsers still pipeline poorly without care.",
				"Streaming — four native method kinds; REST approximates with SSE, websockets, or polling (the realtime topic builds those).",
				"Deadlines — first-class and propagating; REST reuses per-request timeouts that rarely chain.",
				"Reach — REST works everywhere with no client generation; gRPC needs tooling (or gRPC-Web/Connect for browsers).",
				"Ecosystem — caching, CDNs, and proxies assume HTTP semantics; gRPC traffic often needs explicit support (ingress configs, load balancers speaking HTTP/2)."),
			text("versus-stance",
				"This guide's recorded stance mirrors what mature production systems converge on: REST for "+
					"the edge — anything a browser, a mobile app you do not control, or a partner calls — "+
					"and gRPC between your OWN services, where the contract, the streaming, and the deadline "+
					"propagation pay immediately and the tooling cost is amortized across a handful of "+
					"repositories. The posts substrate demonstrates the coexistence on purpose: one domain "+
					"layer, one set of use cases, two transports carrying identical semantics with different "+
					"vocabularies — the 404 envelope and ``codes.NotFound`` are the same sentence in two "+
					"languages, and neither transport knows the other exists. That seam is the real lesson "+
					"of this topic: transports are adapters, the domain is the product."),
		),
		// ── 9. Terminal companion ────────────────────────────────────
		section("terminal-companion", "Terminal companion: the console endpoint and grpcurl",
			"The demo endpoint is live now; the grpcurl tour below is the shape the wired :8102 listener answers.",
			text("term-pitch",
				"With the stack up (server on :8100), the console endpoint is live and needs nothing but "+
					"curl: every fetch runs the whole in-process round trip — server build, bufconn dial, "+
					"six calls, teardown — and answers with the reports you just read. The grpcurl lines "+
					"that follow are the same service from the shell once the composition root wires the "+
					"real listener on the guide's reserved gRPC port (:8102, address from ``GRPC_ADDR``): "+
					"grpcurl is to gRPC what curl is to REST, and its ``-proto`` flag replaces server "+
					"reflection (which the wiring step can add later via the reflection package) — the "+
					"commands are recorded here so the tour is one paste away the moment the port opens:"),
			code("term-console", "bash", "", "",
				"# the console: real RPCs run in-process per fetch — codes as data\n"+
					"curl -s localhost:8100/api/demo/api/grpc-console | python3 -m json.tool | head -30"),
			code("term-statuses", "bash", "", "",
				"# every exhibit's observed status, one line each: OK ×4, NotFound, DeadlineExceeded\n"+
					"curl -s localhost:8100/api/demo/api/grpc-console | python3 -m json.tool | grep '\"statusCode\"'"),
			code("term-grpcurl-describe", "bash", "", "",
				"# once the listener is wired (GRPC_ADDR, default :8102) — describe the service from the contract\n"+
					"grpcurl -plaintext -proto server/api/proto/posts.proto localhost:8102 describe guide.posts.v1.PostsService"),
			code("term-grpcurl-unary", "bash", "", "",
				"# unary: a hit, then a miss — the miss answers a status, not a post\n"+
					"grpcurl -plaintext -proto server/api/proto/posts.proto \\\n"+
					"  -d '{\"id\": \"p-3\"}' localhost:8102 guide.posts.v1.PostsService/GetPost\n"+
					"grpcurl -plaintext -proto server/api/proto/posts.proto \\\n"+
					"  -d '{\"id\": \"p-404\"}' localhost:8102 guide.posts.v1.PostsService/GetPost"),
			code("term-grpcurl-streams", "bash", "", "",
				"# server-stream a page, then walk it by the opaque token from the last item\n"+
					"grpcurl -plaintext -proto server/api/proto/posts.proto \\\n"+
					"  -d '{\"tag\": \"go\", \"pageSize\": 3}' localhost:8102 guide.posts.v1.PostsService/ListPosts"),
			code("term-deadline", "bash", "", "",
				"# the deadline exhibit: a 150ms budget beats 10s of simulated work — every time\n"+
					"grpcurl -plaintext -proto server/api/proto/posts.proto -max-time 0.15 \\\n"+
					"  -d '{\"id\": \"p-slow\"}' localhost:8102 guide.posts.v1.PostsService/GetPost"),
			code("term-buf", "bash", "", "",
				"# the codegen loop itself: lint the contract, regenerate, commit both\n"+
					"cd server && buf lint && buf generate"),
			text("term-coda",
				"Notice what the tour shares with every companion before it: nothing is simulated for the "+
					"sake of the shell. The console endpoint's statuses are observed, not asserted; the "+
					"grpcurl calls are the same methods the tests and the demo drive; and the deadline line "+
					"will fail identically forever, on purpose — a command whose documented behavior is "+
					"\"times out with code 4\" is a working lesson, not a broken one. From here the "+
					"curriculum turns to the data layer: the same posts domain that has now spoken REST, "+
					"GraphQL, and gRPC gets a real database, and the in-memory repository this topic relied "+
					"on becomes the swap seam the schema topic was designed around."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"grpc",
		"gRPC",
		"Contract-first RPC in Go: the .proto contract, all four stream kinds, serving with grpc-go over the "+
			"posts domain, stubs and channels, status codes and deadlines, buf codegen — live in-process in the console.",
		"apis",
		15,
		sections,
	)
}
