// Package obs is the data layer's observability adapters: pingers that
// satisfy the readiness port the domain owns (internal/domain/obs) over
// the process's EXISTING pgx pool and go-redis client — no new
// connections, no second pool, just the cheapest honest probe each
// driver already knows how to answer.
package obs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	domobs "guide/internal/domain/obs"
)

// Compile-time proofs the adapters satisfy the domain's readiness port.
var (
	_ domobs.Pinger = (*PgxPinger)(nil)
	_ domobs.Pinger = (*GoRedisPinger)(nil)
)

// PgxPinger proves PostgreSQL through the process's shared pool: one
// round trip on one pooled connection, bounded by the caller's context
// (the readiness check's own timeout).
type PgxPinger struct {
	pool *pgxpool.Pool
}

// NewPgxPinger wraps the shared pool as a readiness pinger.
func NewPgxPinger(pool *pgxpool.Pool) *PgxPinger {
	return &PgxPinger{pool: pool}
}

// Ping implements the readiness port: acquire, round-trip, release.
func (p *PgxPinger) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// GoRedisPinger proves Redis through the process's lazy client: the
// first command dials if needed (bounded by the client's own dial
// timeouts), so a down Redis answers with a fast refusal, not a hang —
// and the readiness check's timeout bounds even that.
type GoRedisPinger struct {
	client *goredis.Client
}

// NewGoRedisPinger wraps the shared client as a readiness pinger.
func NewGoRedisPinger(client *goredis.Client) *GoRedisPinger {
	return &GoRedisPinger{client: client}
}

// Ping implements the readiness port with the cheapest Redis command.
func (p *GoRedisPinger) Ping(ctx context.Context) error {
	if err := p.client.Ping(ctx).Err(); err != nil {
		return err
	}
	return nil
}
