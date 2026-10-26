// The grpc topic's exhibit: REAL RPCs, run in-process. This builder dials
// the same grpcapi.NewGRPCServer the composition root will serve, over a
// bufconn listener (google.golang.org/grpc/test/bufconn — an in-memory
// pipe: real HTTP/2, real generated stubs, real status codes, no TCP
// port), against a FRESH in-memory posts repository so every id below is
// deterministic (seeds p-1…p-6; the first created id is p-7).
//
// TEACHING-EXCEPTION (layering): the family rule is presentation → domain
// ← data, and this file lives in domain/demos while constructing the
// data-layer adapter and the gRPC server — the same exemption the family
// grants teaching-exhibit handlers. The demo IS the exhibit: its subject
// is a whole in-process gRPC stack, and zero-input purity (same call, same
// payload; the handler stays thin) is preserved by building the stack
// inside the builder. It is marked, and it is the only such import in
// this package.
//
// Determinism rules: fixed exhibit inputs only; error CODES are recorded
// as data (codes.NotFound, codes.DeadlineExceeded — never timing text);
// the deadline case is a deliberately-tight context the server's slow
// path honors (no sleep race — 150ms always beats the 10s work); and no
// wall-clock timestamps are echoed into rows (storage stamps now-time on
// writes, so rows carry identity, not clocks).
package demos

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	postsdata "guide/internal/data/posts"
	"guide/internal/domain/posts"
	"guide/internal/grpcapi"
	pb "guide/internal/grpcapi/pb"
)

// GRPCTransportFacts records how the console reached the service — the
// part that makes the demo honest: these are real RPCs on a real
// *grpc.Server, just over an in-memory listener instead of a port.
type GRPCTransportFacts struct {
	Mode        string `json:"mode"`        // the listener story (bufconn, no port)
	DialTarget  string `json:"dialTarget"`  // the address grpc.NewClient was given
	Security    string `json:"security"`    // credentials story
	ProtoFile   string `json:"protoFile"`   // the contract this whole stack derives from
	Service     string `json:"service"`     // full proto service name
	GeneratedGo string `json:"generatedGo"` // where buf generate writes the Go code
	CodegenCmd  string `json:"codegenCmd"`  // the one command that regenerates it all
}

// GRPCResultRow is one observation from a round-trip: an item received, a
// message sent, a status observed. Labels name the thing; details carry
// the observed values.
type GRPCResultRow struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
}

// GRPCCallReport is one exhibit call's report: the method kind it
// demonstrates, the full method path, the observed status code (name and
// numeric — codes are the gRPC contract's error vocabulary), how many RPC
// round trips the exhibit made, and the rows it observed.
type GRPCCallReport struct {
	Kind         string          `json:"kind"`
	Method       string          `json:"method"`
	Pattern      string          `json:"pattern"`
	Summary      string          `json:"summary"`
	StatusCode   string          `json:"statusCode"`
	StatusNumber uint32          `json:"statusNumber"`
	RoundTrips   int             `json:"roundTrips"`
	Results      []GRPCResultRow `json:"results"`
}

// GRPCConsolePayload is the wire shape of GET /api/demo/api/grpc-console
// (camelCase JSON) — the grpc topic's inline demo: the transport facts
// plus one report per exhibit call, covering all four method kinds and
// the three statuses the topic teaches (OK, NotFound, DeadlineExceeded).
type GRPCConsolePayload struct {
	Transport GRPCTransportFacts `json:"transport"`
	Calls     []GRPCCallReport   `json:"calls"`
}

// The four method kinds, as the reports tag them.
const (
	grpcKindUnary        = "unary"
	grpcKindServerStream = "server-stream"
	grpcKindClientStream = "client-stream"
	grpcKindBidi         = "bidi"
)

// The full method paths, exactly as they appear on the wire (package +
// service + method — the string grpcurl and interceptors see).
const (
	grpcMethodGetPost     = "/guide.posts.v1.PostsService/GetPost"
	grpcMethodListPosts   = "/guide.posts.v1.PostsService/ListPosts"
	grpcMethodCreatePosts = "/guide.posts.v1.PostsService/CreatePosts"
	grpcMethodSyncTags    = "/guide.posts.v1.PostsService/SyncTags"
)

// callStatus observes one call outcome: the status name and numeric code
// grpc reports for err (OK when err is nil).
func callStatus(err error) (string, uint32) {
	code := status.Code(err)
	return code.String(), uint32(code)
}

// row is the row-builder every report below shares.
func row(label, detail string, ok bool) GRPCResultRow {
	return GRPCResultRow{Label: label, Detail: detail, OK: ok}
}

// unaryGetReport runs GetPost for one id and reports the round trip —
// the same method for a hit (OK) and a miss (NotFound): the status code
// IS the error model.
func unaryGetReport(ctx context.Context, client pb.PostsServiceClient, id string) GRPCCallReport {
	post, err := client.GetPost(ctx, &pb.PostRequest{Id: id})
	codeName, codeNumber := callStatus(err)
	report := GRPCCallReport{
		Kind:         grpcKindUnary,
		Method:       grpcMethodGetPost,
		Pattern:      "1 request → 1 response",
		StatusCode:   codeName,
		StatusNumber: codeNumber,
		RoundTrips:   1,
	}
	if err != nil {
		report.Summary = fmt.Sprintf("GetPost(%q) answered a status, not a post — and that is the whole error model", id)
		report.Results = []GRPCResultRow{
			row("request", fmt.Sprintf(`PostRequest{Id: %q}`, id), false),
			row("observed status", fmt.Sprintf("%s (%d): %s", codeName, codeNumber, statusMessage(err)), false),
		}
		return report
	}
	report.Summary = fmt.Sprintf("GetPost(%q) returned the full post — one message in, one message out", id)
	report.Results = []GRPCResultRow{
		row("request", fmt.Sprintf(`PostRequest{Id: %q}`, id), true),
		row("response", fmt.Sprintf("%q · tags: %s", post.GetTitle(), strings.Join(post.GetTags(), ", ")), true),
	}
	return report
}

// unaryDeadlineReport runs GetPost against the slow-path exhibit id with
// a deliberately-tight context: the server's slow path honors ctx.Done(),
// so the 150ms deadline wins deterministically (the simulated work is
// 10s) and the client observes codes.DeadlineExceeded — as data, with no
// timing text anywhere in the report.
func unaryDeadlineReport(ctx context.Context, client pb.PostsServiceClient) GRPCCallReport {
	callCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()

	_, err := client.GetPost(callCtx, &pb.PostRequest{Id: grpcapi.SlowPostID})
	codeName, codeNumber := callStatus(err)
	if codeName != "DeadlineExceeded" {
		// In-process bufconn cannot make 150ms lose a 10s race; a
		// different code means the exhibit itself broke — fail loudly.
		panic(fmt.Sprintf("grpc-console demo: deadline case observed %s, want DeadlineExceeded", codeName))
	}
	return GRPCCallReport{
		Kind:         grpcKindUnary,
		Method:       grpcMethodGetPost,
		Pattern:      "1 request → 1 response, under a deadline",
		Summary:      "A 150ms deadline on the slow-path id — the server races its work against ctx.Done() and the deadline wins",
		StatusCode:   codeName,
		StatusNumber: codeNumber,
		RoundTrips:   1,
		Results: []GRPCResultRow{
			row("request", fmt.Sprintf(`PostRequest{Id: %q} · context.WithTimeout(150ms)`, grpcapi.SlowPostID), true),
			row("observed status", fmt.Sprintf("%s (%d) — deadlines propagate end to end; nothing waits out the work", codeName, codeNumber), false),
		},
	}
}

// serverStreamListReport walks the tag-filtered collection by page token:
// page 1 (three items + a cursor), then the cursor's page (the partial
// remainder, no cursor). Two round trips demonstrate that tokens are the
// pagination protocol — opaque strings you hand back unmodified.
func serverStreamListReport(ctx context.Context, client pb.PostsServiceClient) GRPCCallReport {
	report := GRPCCallReport{
		Kind:       grpcKindServerStream,
		Method:     grpcMethodListPosts,
		Pattern:    "1 request → a stream of items",
		RoundTrips: 2,
	}
	var results []GRPCResultRow
	results = append(results, row("request · page 1", `ListRequest{Tag: "go", PageSize: 3}`, true))

	var token string
	for page := 1; ; page++ {
		stream, err := client.ListPosts(ctx, &pb.ListRequest{Tag: "go", PageSize: 3, PageToken: token})
		if err != nil {
			panic(fmt.Sprintf("grpc-console demo: ListPosts page %d: %v", page, err))
		}
		var items []*pb.PostListItem
		for {
			item, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(fmt.Sprintf("grpc-console demo: ListPosts recv: %v", err))
			}
			items = append(items, item)
		}
		token = ""
		if len(items) > 0 {
			token = items[len(items)-1].GetNextPageToken()
		}
		var ids []string
		for _, item := range items {
			ids = append(ids, item.GetId())
		}
		itemNoun := "items"
		if len(items) == 1 {
			itemNoun = "item"
		}
		results = append(results, row(
			fmt.Sprintf("page %d received", page),
			fmt.Sprintf("%d %s: %s", len(items), itemNoun, strings.Join(ids, ", ")),
			true,
		))
		if token == "" {
			results = append(results, row("cursor after page "+fmt.Sprint(page), "empty — the collection is exhausted; no more pages exist", true))
			break
		}
		results = append(results, row(
			fmt.Sprintf("cursor after page %d", page),
			fmt.Sprintf("%q — opaque by contract: hand it back unmodified as the next PageToken", token),
			true,
		))
	}

	codeName, codeNumber := callStatus(nil)
	report.StatusCode = codeName
	report.StatusNumber = codeNumber
	report.Summary = "ListPosts(tag: \"go\", page_size: 3) walked by page token — four seeds match, so page 2 returns the partial remainder and no cursor"
	report.Results = results
	return report
}

// clientStreamCreateReport streams three drafts in ONE call and reports
// the single CreateSummary — the batch shape that would otherwise cost
// one round trip per item.
func clientStreamCreateReport(ctx context.Context, client pb.PostsServiceClient) GRPCCallReport {
	writer, err := client.CreatePosts(ctx)
	if err != nil {
		panic(fmt.Sprintf("grpc-console demo: CreatePosts open: %v", err))
	}
	drafts := []*pb.CreatePostRequest{
		{Title: "Streaming a batch in", Body: "Three drafts, one call, one summary.", Tags: []string{"grpc"}},
		{Title: "One round trip, many messages", Body: "The client sends; the server stores as they arrive.", Tags: []string{"grpc"}},
		{Title: "The summary answers all", Body: "received, created, and ids in arrival order.", Tags: []string{"grpc", "batch"}},
	}
	for _, draft := range drafts {
		if err := writer.Send(draft); err != nil {
			panic(fmt.Sprintf("grpc-console demo: CreatePosts send: %v", err))
		}
	}
	summary, err := writer.CloseAndRecv()
	if err != nil {
		panic(fmt.Sprintf("grpc-console demo: CreatePosts summary: %v", err))
	}

	codeName, codeNumber := callStatus(nil)
	return GRPCCallReport{
		Kind:         grpcKindClientStream,
		Method:       grpcMethodCreatePosts,
		Pattern:      "a stream of requests → 1 response",
		Summary:      "CreatePosts streamed three drafts and answered once — the summary carries received, created, and the ids in arrival order",
		StatusCode:   codeName,
		StatusNumber: codeNumber,
		RoundTrips:   1,
		Results: []GRPCResultRow{
			row("messages sent", fmt.Sprintf("%d CreatePostRequest drafts on one stream", len(drafts)), true),
			row("summary received", fmt.Sprintf("received %d · created %d", summary.GetReceived(), summary.GetCreated()), true),
			row("ids, in arrival order", strings.Join(summary.GetIds(), ", "), true),
		},
	}
}

// bidiSyncTagsReport streams three tag updates and reports the per-item
// acks: two applied, one rejected (unknown post) — the rejection is DATA
// in an ack, not a failed RPC, and the stream keeps going. Requires the
// posts clientStreamCreateReport created (p-7, p-8).
func bidiSyncTagsReport(ctx context.Context, client pb.PostsServiceClient) GRPCCallReport {
	stream, err := client.SyncTags(ctx)
	if err != nil {
		panic(fmt.Sprintf("grpc-console demo: SyncTags open: %v", err))
	}
	updates := []*pb.TagUpdate{
		{PostId: "p-7", Tags: []string{"go", "rpc", "Streaming"}}, // normalized on write
		{PostId: "p-999", Tags: []string{"ghost"}},                // unknown — acked, not fatal
		{PostId: "p-8", Tags: []string{"grpc", "bidi"}},           // proves the stream survived
	}
	for _, update := range updates {
		if err := stream.Send(update); err != nil {
			panic(fmt.Sprintf("grpc-console demo: SyncTags send: %v", err))
		}
	}
	if err := stream.CloseSend(); err != nil {
		panic(fmt.Sprintf("grpc-console demo: SyncTags close: %v", err))
	}

	var acks []*pb.SyncAck
	for {
		ack, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(fmt.Sprintf("grpc-console demo: SyncTags ack: %v", err))
		}
		acks = append(acks, ack)
	}

	var results []GRPCResultRow
	results = append(results, row("messages sent", fmt.Sprintf("%d TagUpdates on one call, then CloseSend", len(updates)), true))
	for _, ack := range acks {
		if ack.GetApplied() {
			results = append(results, row(
				"ack · "+ack.GetPostId(),
				fmt.Sprintf("applied — tags now: %s", strings.Join(ack.GetTags(), ", ")),
				true,
			))
			continue
		}
		results = append(results, row(
			"ack · "+ack.GetPostId(),
			fmt.Sprintf("rejected — %s", ack.GetNote()),
			false,
		))
	}

	codeName, codeNumber := callStatus(nil)
	return GRPCCallReport{
		Kind:         grpcKindBidi,
		Method:       grpcMethodSyncTags,
		Pattern:      "a stream of requests ⇄ a stream of responses",
		Summary:      "SyncTags took three updates and acked each one on the same call — the unknown post drew a rejection ack, and the stream kept going",
		StatusCode:   codeName,
		StatusNumber: codeNumber,
		RoundTrips:   1,
		Results:      results,
	}
}

// statusMessage extracts the human message from a status error ("" for
// nil or non-status errors).
func statusMessage(err error) string {
	if err == nil {
		return ""
	}
	if typed, ok := status.FromError(err); ok {
		return typed.Message()
	}
	return err.Error()
}

// dialBufconn opens the client connection over the in-memory listener:
// passthrough:///bufnet with a context dialer that bypasses DNS and TCP
// entirely, and EXPLICIT plaintext credentials — grpc refuses an implicit
// insecure client, which is the right default to be forced to say out
// loud.
func dialBufconn(listener *bufconn.Listener) *grpc.ClientConn {
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(fmt.Sprintf("grpc-console demo: dial bufconn: %v", err))
	}
	return conn
}

// BuildGRPCConsolePayload assembles the whole demo: it builds the server,
// serves it on a bufconn listener, dials it, runs the six exhibit calls
// (four kinds, three statuses), tears everything down, and returns the
// reports. Same call, same payload — the stack inside is what makes the
// demo real rather than narrated.
func BuildGRPCConsolePayload() GRPCConsolePayload {
	repository := postsdata.NewInMemoryRepository()
	server := grpcapi.NewGRPCServer(
		posts.NewListPostsUseCase(repository),
		posts.NewGetPostUseCase(repository),
		posts.NewCreatePostUseCase(repository),
		posts.NewReplacePostUseCase(repository),
	)
	listener := bufconn.Listen(256 * 1024)
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	defer func() {
		server.Stop()
		if err := listener.Close(); err != nil {
			panic(fmt.Sprintf("grpc-console demo: close listener: %v", err))
		}
		<-serveDone
	}()

	conn := dialBufconn(listener)
	defer func() {
		if err := conn.Close(); err != nil {
			panic(fmt.Sprintf("grpc-console demo: close conn: %v", err))
		}
	}()
	client := pb.NewPostsServiceClient(conn)
	ctx := context.Background()

	return GRPCConsolePayload{
		Transport: GRPCTransportFacts{
			Mode:        "in-process bufconn listener — real HTTP/2 RPCs, no TCP port",
			DialTarget:  "passthrough:///bufnet",
			Security:    "plaintext credentials, explicit — grpc refuses implicit insecure",
			ProtoFile:   "server/api/proto/posts.proto",
			Service:     "guide.posts.v1.PostsService",
			GeneratedGo: "server/internal/grpcapi/pb",
			CodegenCmd:  "buf generate",
		},
		Calls: []GRPCCallReport{
			unaryGetReport(ctx, client, "p-3"),
			unaryGetReport(ctx, client, "p-404"),
			serverStreamListReport(ctx, client),
			clientStreamCreateReport(ctx, client),
			bidiSyncTagsReport(ctx, client),
			unaryDeadlineReport(ctx, client),
		},
	}
}
