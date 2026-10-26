package grpcapi

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	postsmemory "guide/internal/data/posts"
	"guide/internal/domain/posts"
	pb "guide/internal/grpcapi/pb"
)

// The PostsService implementation, exercised end-to-end over bufconn — the
// in-memory listener grpc-go ships for exactly this: real HTTP/2 RPCs, real
// generated stubs, real status codes, zero TCP. Every test below spins the
// same NewGRPCServer the composition root will wire, against a FRESH
// seeded in-memory repository, so ids and ordering are deterministic
// (seeds p-1…p-6, newest first; the first created id is p-7).

// servePosts starts the service on a bufconn listener and returns a real
// generated client plus a cleanup that closes both sides and waits for the
// server goroutine to exit — no leaks, -race clean.
func servePosts(t *testing.T) (pb.PostsServiceClient, func()) {
	t.Helper()

	repository := postsmemory.NewRepository()
	server := NewGRPCServer(
		posts.NewListPostsUseCase(repository),
		posts.NewGetPostUseCase(repository),
		posts.NewCreatePostUseCase(repository),
		posts.NewReplacePostUseCase(repository),
	)
	listener := bufconn.Listen(256 * 1024)
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		// bufconn is plaintext in-process — credentials must still be
		// EXPLICIT (grpc refuses an implicit plaintext client), and
		// "insecure" names exactly what the demo means by it: no TLS over
		// an in-memory pipe. Production wires TLS at the composition root.
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	cleanup := func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close client conn: %v", err)
		}
		server.Stop()
		if err := listener.Close(); err != nil {
			t.Errorf("close bufconn listener: %v", err)
		}
		<-serveDone
	}
	return pb.NewPostsServiceClient(conn), cleanup
}

func TestUnaryGetPostRoundTripsASeedPost(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	post, err := client.GetPost(context.Background(), &pb.PostRequest{Id: "p-3"})
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if post.GetTitle() != "Errors are values" {
		t.Errorf("title = %q, want the p-3 seed title", post.GetTitle())
	}
	if post.GetId() != "p-3" || len(post.GetTags()) != 2 {
		t.Errorf("post drifted: %v", post)
	}
	if post.GetCreatedAt().AsTime().IsZero() {
		t.Error("created_at must carry the storage timestamp")
	}
}

func TestUnaryEmptyIDIsInvalidArgument(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	_, err := client.GetPost(context.Background(), &pb.PostRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty id code = %v, want InvalidArgument (present-but-empty is data)", status.Code(err))
	}
}

func TestUnaryUnknownIDIsNotFound(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	_, err := client.GetPost(context.Background(), &pb.PostRequest{Id: "p-404"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown id code = %v, want NotFound", status.Code(err))
	}
	if !strings.Contains(err.Error(), "p-404") {
		t.Errorf("status message should name the missing id: %v", err)
	}
}

func TestUnarySlowPathHonorsTheDeadline(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.GetPost(ctx, &pb.PostRequest{Id: SlowPostID})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("slow path with 150ms deadline code = %v, want DeadlineExceeded", status.Code(err))
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("deadline must win the race, took %v", elapsed)
	}
}

func TestServerStreamPaginatesByPageToken(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()
	ctx := context.Background()

	first, err := client.ListPosts(ctx, &pb.ListRequest{Tag: "go", PageSize: 2})
	if err != nil {
		t.Fatalf("ListPosts page 1: %v", err)
	}
	var pageOne []*pb.PostListItem
	for {
		item, err := first.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("page 1 recv: %v", err)
		}
		pageOne = append(pageOne, item)
	}
	if len(pageOne) != 2 {
		t.Fatalf("page 1 = %d items, want 2", len(pageOne))
	}
	if got := pageOne[0].GetId() + "," + pageOne[1].GetId(); got != "p-6,p-3" {
		t.Errorf("page 1 order = %s, want p-6,p-3 (newest first; four seeds carry the go tag)", got)
	}
	token := pageOne[len(pageOne)-1].GetNextPageToken()
	if token != "2" {
		t.Fatalf("page 1 cursor = %q, want \"2\"", token)
	}

	second, err := client.ListPosts(ctx, &pb.ListRequest{Tag: "go", PageSize: 2, PageToken: token})
	if err != nil {
		t.Fatalf("ListPosts page 2: %v", err)
	}
	var pageTwo []*pb.PostListItem
	for {
		item, err := second.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("page 2 recv: %v", err)
		}
		pageTwo = append(pageTwo, item)
	}
	if len(pageTwo) != 2 || pageTwo[0].GetId() != "p-2" || pageTwo[1].GetId() != "p-1" {
		t.Fatalf("page 2 = %v, want p-2,p-1 (the collection is exhausted)", pageTwo)
	}
	if pageTwo[len(pageTwo)-1].GetNextPageToken() != "" {
		t.Errorf("exhausted page must carry no cursor, got %q", pageTwo[len(pageTwo)-1].GetNextPageToken())
	}
}

func TestServerStreamRefusesForgedPageTokens(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	for _, token := range []string{"not-a-cursor", "0", "1"} {
		stream, err := client.ListPosts(context.Background(), &pb.ListRequest{PageToken: token})
		if err != nil {
			t.Fatalf("ListPosts(%q): %v", token, err)
		}
		if _, err := stream.Recv(); status.Code(err) != codes.InvalidArgument {
			t.Errorf("token %q code = %v, want InvalidArgument", token, status.Code(err))
		}
	}
}

func TestServerStreamRejectsPageSizeOutsideTheDomainBounds(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	stream, err := client.ListPosts(context.Background(), &pb.ListRequest{PageSize: 99})
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.InvalidArgument {
		t.Errorf("page_size 99 code = %v, want InvalidArgument (the same 1–50 bound the REST API enforces)", status.Code(err))
	}
}

func TestClientStreamCreatesABatchAndSummarizes(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	ctx := context.Background()
	writer, err := client.CreatePosts(ctx)
	if err != nil {
		t.Fatalf("CreatePosts: %v", err)
	}
	drafts := []*pb.CreatePostRequest{
		{Title: "Batch one", Body: "First of three.", Tags: []string{"grpc"}},
		{Title: "Batch two", Body: "Second of three.", Tags: []string{"grpc"}},
		{Title: "Batch three", Body: "Third of three.", Tags: []string{"grpc", "batch"}},
	}
	for _, draft := range drafts {
		if err := writer.Send(draft); err != nil {
			t.Fatalf("send draft %q: %v", draft.GetTitle(), err)
		}
	}
	summary, err := writer.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if summary.GetReceived() != 3 || summary.GetCreated() != 3 {
		t.Errorf("summary = %d/%d, want 3/3", summary.GetReceived(), summary.GetCreated())
	}
	if strings.Join(summary.GetIds(), ",") != "p-7,p-8,p-9" {
		t.Errorf("ids = %v, want p-7,p-8,p-9 in arrival order", summary.GetIds())
	}
}

func TestClientStreamFailsTheWholeRPCOnTheFirstInvalidDraft(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	ctx := context.Background()
	writer, err := client.CreatePosts(ctx)
	if err != nil {
		t.Fatalf("CreatePosts: %v", err)
	}
	if err := writer.Send(&pb.CreatePostRequest{Title: "Valid before the bad one", Body: "Stored first — streaming semantics."}); err != nil {
		t.Fatalf("send valid draft: %v", err)
	}
	if err := writer.Send(&pb.CreatePostRequest{Title: "", Body: "Empty title is data, and data can be wrong."}); err != nil {
		t.Fatalf("send invalid draft: %v", err)
	}
	if _, err := writer.CloseAndRecv(); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid draft code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestBidiStreamAcksPerItemAndSurvivesRejects(t *testing.T) {
	client, cleanup := servePosts(t)
	defer cleanup()

	ctx := context.Background()
	stream, err := client.SyncTags(ctx)
	if err != nil {
		t.Fatalf("SyncTags: %v", err)
	}
	updates := []*pb.TagUpdate{
		{PostId: "p-7", Tags: []string{"go", "rpc"}}, // needs a p-7: create it first
		{PostId: "p-999", Tags: []string{"ghost"}},   // unknown — must be acked, not fatal
		{PostId: "p-8", Tags: []string{"bidi"}},      // proves the stream survived the reject
	}
	// Create the two posts the sync targets (p-7, p-8 from a fresh seed set).
	batch, err := client.CreatePosts(ctx)
	if err != nil {
		t.Fatalf("CreatePosts: %v", err)
	}
	for _, draft := range []*pb.CreatePostRequest{
		{Title: "Sync target one", Body: "b", Tags: []string{"stale"}},
		{Title: "Sync target two", Body: "b", Tags: []string{"stale"}},
	} {
		if err := batch.Send(draft); err != nil {
			t.Fatalf("seed send: %v", err)
		}
	}
	if _, err := batch.CloseAndRecv(); err != nil {
		t.Fatalf("seed batch: %v", err)
	}

	for _, update := range updates {
		if err := stream.Send(update); err != nil {
			t.Fatalf("send update %v: %v", update.GetPostId(), err)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}

	var acks []*pb.SyncAck
	for {
		ack, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ack recv: %v", err)
		}
		acks = append(acks, ack)
	}
	if len(acks) != 3 {
		t.Fatalf("acks = %d, want 3 (one per update, rejects included)", len(acks))
	}
	if !acks[0].GetApplied() || strings.Join(acks[0].GetTags(), ",") != "go,rpc" {
		t.Errorf("first ack = %v, want applied with normalized tags go,rpc", acks[0])
	}
	if acks[1].GetApplied() || acks[1].GetNote() == "" {
		t.Errorf("second ack = %v, want applied=false with a reason", acks[1])
	}
	if !acks[2].GetApplied() {
		t.Errorf("third ack = %v, want applied=true — the reject must not kill the stream", acks[2])
	}
}

func TestNewGRPCServerRegistersAllFourMethods(t *testing.T) {
	repository := postsmemory.NewRepository()
	server := NewGRPCServer(
		posts.NewListPostsUseCase(repository),
		posts.NewGetPostUseCase(repository),
		posts.NewCreatePostUseCase(repository),
		posts.NewReplacePostUseCase(repository),
	)
	defer server.Stop()

	info, ok := server.GetServiceInfo()["guide.posts.v1.PostsService"]
	if !ok {
		t.Fatal("PostsService must be registered under its full proto name")
	}
	want := map[string]bool{
		"GetPost": true, "ListPosts": true, "CreatePosts": true, "SyncTags": true,
	}
	if len(info.Methods) != len(want) {
		t.Fatalf("methods = %d, want %d", len(info.Methods), len(want))
	}
	for _, method := range info.Methods {
		if !want[method.Name] {
			t.Errorf("unexpected method %q", method.Name)
		}
	}
}
