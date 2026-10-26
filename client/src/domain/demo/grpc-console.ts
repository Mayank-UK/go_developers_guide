/**
 * Domain models for the grpc topic's console demo — the client-side shape
 * of `GET /api/demo/api/grpc-console`. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and domain
 * may diverge later).
 */

/** The four gRPC method kinds the console demonstrates. */
export type GRPCMethodKind = 'unary' | 'server-stream' | 'client-stream' | 'bidi';

/**
 * The statuses the server's exhibit script can observe — gRPC's closed
 * error vocabulary, recorded as data (never as timing text).
 */
export type GRPCStatusName = 'OK' | 'NotFound' | 'InvalidArgument' | 'DeadlineExceeded' | 'Unavailable';

/** How the console reached the service — the facts that make it honest. */
export interface GRPCTransportFacts {
  /** The listener story: in-process bufconn, real HTTP/2, no TCP port. */
  mode: string;
  /** The address grpc.NewClient was given (`passthrough:///bufnet`). */
  dialTarget: string;
  /** The credentials story (explicit plaintext over an in-memory pipe). */
  security: string;
  /** The contract the whole stack derives from. */
  protoFile: string;
  /** The full proto service name, e.g. `guide.posts.v1.PostsService`. */
  service: string;
  /** Where `buf generate` writes the Go code the demo calls through. */
  generatedGo: string;
  /** The one command that regenerates the whole stack. */
  codegenCmd: string;
}

/** One observation from a round trip: an item, a message, a status. */
export interface GRPCResultRow {
  /** What this row is: `request · page 1`, `ack · p-7`, … */
  label: string;
  /** The observed values, verbatim from the RPC result. */
  detail: string;
  /** false marks rejection rows (a miss, an expired deadline, a rejected ack). */
  ok: boolean;
}

/** One exhibit call's report: kind, method, observed status, rows. */
export interface GRPCCallReport {
  /** Which of the four method kinds this call demonstrates. */
  kind: GRPCMethodKind;
  /** Full method path, e.g. `/guide.posts.v1.PostsService/GetPost`. */
  method: string;
  /** The arrow pattern, e.g. `1 request → 1 response`. */
  pattern: string;
  /** One-line what happened. */
  summary: string;
  /** The observed status name — the contract's error vocabulary. */
  statusCode: GRPCStatusName;
  /** The status's numeric code (OK 0, DeadlineExceeded 4, NotFound 5). */
  statusNumber: number;
  /** How many RPC round trips this exhibit made (2 for the token walk). */
  roundTrips: number;
  /** The real RPC results, in order. */
  results: GRPCResultRow[];
}

/** The demo payload: transport facts + one report per exhibit call. */
export interface GRPCConsoleDemoData {
  transport: GRPCTransportFacts;
  calls: GRPCCallReport[];
}
