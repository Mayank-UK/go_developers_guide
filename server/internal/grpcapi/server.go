// Package grpcapi is the gRPC adapter over the posts domain: the
// PostsService implementation generated from api/proto/posts.proto, wired
// to the same use cases the REST /api/posts routes serve. The layering is
// the family rule with a second transport: this adapter plays the
// presentation role for gRPC — it translates protobuf messages into domain
// drafts and queries, calls ONE use case per method, and maps the app's
// typed error taxonomy onto grpc status codes (core.ErrNotFound →
// codes.NotFound, core.ErrValidationFailed → codes.InvalidArgument), so a
// caller on either transport meets the same semantics with a different
// wire.
//
// The composition root (cmd/guide/main.go) owns listener wiring: construct
// this package's NewGRPCServer over the posts use cases, net.Listen on the
// GRPC_ADDR address (the guide's reserved port is :8102), and Serve on a
// goroutine with GracefulStop on shutdown — the doc on NewGRPCServer
// records the exact shape. Until that wiring lands, the grpc-console demo
// and these tests drive the same server in-process over a bufconn
// listener (no port, no TCP) — the pattern grpc-go's own tests use.
package grpcapi

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"guide/internal/core"
	"guide/internal/domain/posts"
	pb "guide/internal/grpcapi/pb"
)

// SlowPostID is the deadline exhibit: GetPost for this id takes a
// deliberately slow path (simulated expensive work) that honors the call's
// context — a caller with a 50ms deadline learns DeadlineExceeded in
// exactly 50ms, never the full work duration. The id deliberately matches
// no stored post, so a patient caller (no deadline) eventually receives
// the honest NotFound once the slow path finishes.
const SlowPostID = "p-slow"

// slowPathDelay is the simulated work duration behind SlowPostID. It is
// long enough that any sane test deadline wins the race deterministically,
// and finite so a no-deadline caller still gets an answer.
const slowPathDelay = 10 * time.Second

// PostsServer implements the generated PostsServiceServer interface by
// delegating to the posts use cases. It embeds the generated
// UnimplementedPostsServiceServer for forward compatibility: adding a
// method to posts.proto then regenerating makes unwired methods answer
// codes.Unimplemented instead of failing to compile the whole server.
type PostsServer struct {
	pb.UnimplementedPostsServiceServer

	list    *posts.ListPostsUseCase
	get     *posts.GetPostUseCase
	create  *posts.CreatePostUseCase
	replace *posts.ReplacePostUseCase
}

// NewPostsServer wires the four posts use cases the service delegates to —
// the same instances the REST handlers hold, constructed once at the
// composition root.
func NewPostsServer(
	list *posts.ListPostsUseCase,
	get *posts.GetPostUseCase,
	create *posts.CreatePostUseCase,
	replace *posts.ReplacePostUseCase,
) *PostsServer {
	return &PostsServer{list: list, get: get, create: create, replace: replace}
}

// Register adds the service to any grpc.ServiceRegistrar — a production
// *grpc.Server or a test harness. Keeping registration separate from
// construction is what lets the bufconn demo and the composition root share
// one constructor.
func (s *PostsServer) Register(registrar grpc.ServiceRegistrar) {
	pb.RegisterPostsServiceServer(registrar, s)
}

// NewGRPCServer builds a complete *grpc.Server with the posts service
// registered — the constructor the composition root wires at wave close:
//
//	grpcServer := grpcapi.NewGRPCServer(postsList, postsGet, postsCreate, postsReplace)
//	listener, err := net.Listen("tcp", grpcAddr) // GRPC_ADDR env; the guide's reserved port is ":8102"
//	if err != nil { ... }
//	go func() { grpcServer.Serve(listener) }()   // HTTP/2 multiplexed RPCs on one listener
//	// …on shutdown: grpcServer.GracefulStop()
//
// opts (grpc.ServerOption) carries production concerns at the call site —
// TLS credentials, keepalive enforcement, interceptors for logging and
// auth — while the demo and tests pass none and serve over bufconn.
func NewGRPCServer(
	list *posts.ListPostsUseCase,
	get *posts.GetPostUseCase,
	create *posts.CreatePostUseCase,
	replace *posts.ReplacePostUseCase,
	opts ...grpc.ServerOption,
) *grpc.Server {
	server := grpc.NewServer(opts...)
	NewPostsServer(list, get, create, replace).Register(server)
	return server
}

// ── Unary ────────────────────────────────────────────────────────────────

// GetPost is the unary method: one request in, one response out — the
// shape that makes gRPC feel like a local function call with a network in
// the middle. Empty ids and unknown ids are distinguished the way the REST
// API's 422 and 404 distinguish them, via the status-code mapping below.
func (s *PostsServer) GetPost(ctx context.Context, req *pb.PostRequest) (*pb.Post, error) {
	if req.GetId() == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"id is required — present-but-empty is data, and data can be wrong",
		)
	}
	if req.GetId() == SlowPostID {
		return s.slowGetPost(ctx)
	}
	post, err := s.get.Execute(ctx, req.GetId())
	if err != nil {
		return nil, statusFromDomain(err)
	}
	return postProto(post), nil
}

// slowGetPost is the deadline-respect exhibit. Real handlers do work that
// takes time — storage round trips, fan-out calls — and grpc delivers the
// caller's deadline on ctx. The select below is the pattern every slow
// handler owes its callers: race the work against ctx.Done() and return
// the moment the deadline fires, converting ctx.Err() into a proper
// status (DeadlineExceeded for expired deadlines, Canceled for
// cancellation) with status.FromContextError.
func (s *PostsServer) slowGetPost(ctx context.Context) (*pb.Post, error) {
	timer := time.NewTimer(slowPathDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	case <-timer.C:
		if err := ctx.Err(); err != nil {
			return nil, status.FromContextError(err).Err()
		}
	}
	// SlowPostID matches no stored post: a patient caller gets the honest
	// miss, exactly as a fast GetPost for an unknown id would.
	post, err := s.get.Execute(ctx, SlowPostID)
	if err != nil {
		return nil, statusFromDomain(err)
	}
	return postProto(post), nil
}

// ── Server streaming ─────────────────────────────────────────────────────

// ListPosts is the server-streaming method: one request in, a stream of
// items out, page-token paginated. The token is opaque to the CLIENT by
// contract (send back exactly what the last item gave you); the server
// owns the encoding — here, the next page number — and refuses tokens it
// never issued with codes.InvalidArgument. The domain's own
// NormalizeListQuery does the boundary math, the same rules the REST list
// route applies, so both transports enforce identical bounds.
func (s *PostsServer) ListPosts(req *pb.ListRequest, stream pb.PostsService_ListPostsServer) error {
	ctx := stream.Context()

	page := "1"
	if token := req.GetPageToken(); token != "" {
		parsed, err := strconv.Atoi(token)
		if err != nil || parsed < 2 {
			return status.Errorf(
				codes.InvalidArgument,
				"page token %q is not a cursor this server issued — tokens come from a previous page, unmodified",
				token,
			)
		}
		page = token
	}
	size := req.GetPageSize()
	if size == 0 {
		size = int32(posts.DefaultPageLimit)
	}
	query, err := posts.NormalizeListQuery(
		page,
		strconv.Itoa(int(size)),
		strings.ToLower(strings.TrimSpace(req.GetTag())),
		"", // sort: newest first, the service's single honest ordering
	)
	if err != nil {
		return statusFromDomain(err)
	}

	result, err := s.list.Execute(ctx, query)
	if err != nil {
		return statusFromDomain(err)
	}
	for i, post := range result.Items {
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		item := listItemProto(post)
		if i == len(result.Items)-1 && result.HasNext {
			item.NextPageToken = strconv.Itoa(result.Page + 1)
		}
		if err := stream.Send(item); err != nil {
			return err // the client is gone; the transport's error is the answer
		}
	}
	return nil
}

// ── Client streaming ─────────────────────────────────────────────────────

// CreatePosts is the client-streaming method: many drafts in, one summary
// out. The loop is the whole shape — Recv until io.EOF (the client closed
// its side), validating and storing as messages arrive, then a single
// SendAndClose. The FIRST invalid draft fails the entire RPC with
// codes.InvalidArgument and no summary is ever sent: one RPC carries
// exactly one status, so partial-failure reporting cannot be improvised —
// an API that wants it must design it into its messages, the way SyncTags
// does. Drafts earlier in the stream are already stored when the failure
// lands (streaming semantics — work happens as messages arrive); callers
// needing atomic batches send smaller streams or a transactional message
// of their own design.
func (s *PostsServer) CreatePosts(stream pb.PostsService_CreatePostsServer) error {
	ctx := stream.Context()

	var ids []string
	var received, created int32
	for {
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&pb.CreateSummary{
				Received: received,
				Created:  created,
				Ids:      ids,
			})
		}
		if err != nil {
			return err // transport failure or client cancellation
		}
		received++
		post, err := s.create.Execute(ctx, posts.Draft{
			Title: req.GetTitle(),
			Body:  req.GetBody(),
			Tags:  req.GetTags(),
		})
		if err != nil {
			return statusFromDomain(err)
		}
		ids = append(ids, post.ID)
		created++
	}
}

// ── Bidirectional streaming ──────────────────────────────────────────────

// SyncTags is the bidirectional method: TagUpdates in, SyncAcks out, both
// sides sending on the same call without waiting for the other to close.
// The loop reads one update, answers it, reads the next — an
// acknowledgment protocol. The deliberate contrast with CreatePosts: a bad
// update does NOT fail the RPC; it draws applied=false plus a note, and
// the stream keeps going, because tag sync wants "keep going, tell me
// which failed" while batch create wants all-or-nothing.
func (s *PostsServer) SyncTags(stream pb.PostsService_SyncTagsServer) error {
	ctx := stream.Context()

	for {
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		update, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil // client half-closed; every update has been acked
		}
		if err != nil {
			return err
		}
		ack, err := s.ackOne(ctx, update)
		if err != nil {
			return err
		}
		if err := stream.Send(ack); err != nil {
			return err
		}
	}
}

// ackOne processes a single TagUpdate into a SyncAck. Full-replace
// semantics, like the REST API's PUT: the update's tags ARE the next tag
// set. The post's own title and body ride along unchanged — the update
// message carries only tags, so the current draft is fetched first.
func (s *PostsServer) ackOne(ctx context.Context, update *pb.TagUpdate) (*pb.SyncAck, error) {
	id := update.GetPostId()
	if id == "" {
		return &pb.SyncAck{
			PostId: id,
			Note:   "post_id is required — present-but-empty is data, and data can be wrong",
		}, nil
	}
	current, err := s.get.Execute(ctx, id)
	if err != nil {
		ack, handled := perItemRejection(id, err)
		if handled {
			return ack, nil
		}
		return nil, statusFromDomain(err)
	}
	next, err := s.replace.Execute(ctx, current.ID, posts.Draft{
		Title: current.Title,
		Body:  current.Body,
		Tags:  update.GetTags(),
	})
	if err != nil {
		ack, handled := perItemRejection(id, err)
		if handled {
			return ack, nil
		}
		return nil, statusFromDomain(err)
	}
	return &pb.SyncAck{
		PostId:    next.ID,
		Applied:   true,
		Tags:      append([]string(nil), next.Tags...),
		UpdatedAt: timestamppb.New(next.UpdatedAt),
	}, nil
}

// perItemRejection converts the two caller-fixable taxonomy kinds into a
// per-item ack instead of an RPC failure, reporting whether it handled the
// error. Everything else escalates to statusFromDomain — a store failure
// fails the call honestly.
func perItemRejection(id string, err error) (*pb.SyncAck, bool) {
	var typed *core.Error
	if !errors.As(err, &typed) {
		return nil, false
	}
	switch typed.Kind {
	case core.KindNotFound:
		return &pb.SyncAck{
			PostId: id,
			Note:   typed.Message + " — acked per item; the stream continues",
		}, true
	case core.KindValidationFailed:
		return &pb.SyncAck{
			PostId: id,
			Note:   typed.Message + " — the tag set was not applied",
		}, true
	default:
		return nil, false
	}
}

// ── The taxonomy → status mapping ────────────────────────────────────────

// statusFromDomain is the gRPC counterpart of core.WriteError: one place
// that maps the app's typed error taxonomy onto grpc status codes, so
// every method fails with the code its kind implies. Unknown errors
// degrade to codes.Internal with no internals in the message — the same
// degradation the HTTP writer applies at 500.
func statusFromDomain(err error) error {
	var typed *core.Error
	if !errors.As(err, &typed) {
		return status.Error(codes.Internal, "internal error")
	}
	var code codes.Code
	switch typed.Kind {
	case core.KindValidationFailed:
		code = codes.InvalidArgument // 3 — the caller's message was wrong
	case core.KindNotFound:
		code = codes.NotFound // 5 — the named thing is not there
	case core.KindUnauthorized:
		code = codes.Unauthenticated // 16 — who are you?
	case core.KindForbidden:
		code = codes.PermissionDenied // 7 — I know you; you may not
	case core.KindConflict:
		code = codes.AlreadyExists // 6 — the thing is already there
	case core.KindRateLimited:
		code = codes.ResourceExhausted // 8 — quota/limits consumed
	case core.KindServiceUnavailable:
		code = codes.Unavailable // 14 — retry later, maybe elsewhere
	default:
		code = codes.Internal // 13 — our fault, no specifics on the wire
	}
	return status.Error(code, typed.Message)
}

// ── Domain ↔ protobuf converters ─────────────────────────────────────────

// postProto maps the domain Post onto the wire message, deep-copying tags
// so a handed-out proto never aliases the domain's slice.
func postProto(post posts.Post) *pb.Post {
	return &pb.Post{
		Id:        post.ID,
		Title:     post.Title,
		Body:      post.Body,
		Tags:      append([]string(nil), post.Tags...),
		CreatedAt: timestamppb.New(post.CreatedAt),
		UpdatedAt: timestamppb.New(post.UpdatedAt),
	}
}

// listItemProto maps the list-view shape: identity and classification, no
// body — the same cut the REST API's list view makes.
func listItemProto(post posts.Post) *pb.PostListItem {
	return &pb.PostListItem{
		Id:        post.ID,
		Title:     post.Title,
		Tags:      append([]string(nil), post.Tags...),
		CreatedAt: timestamppb.New(post.CreatedAt),
	}
}
