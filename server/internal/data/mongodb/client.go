// Package mongodb is the guide's MongoDB adapter layer: the dialer that
// owns the optional-service discipline, the events.Store adapter behind
// the domain's port, and the mongo-console demo runner. Everything here
// speaks mongo-driver v2 — go.mongodb.org/mongo-driver/v2 — the current
// driver line (v1 is frozen, maintenance-only). Domain packages never
// import the driver; this package never imports a handler.
package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"guide/internal/core"
)

// connectTimeout bounds the boot-time proof-of-life ping.
const connectTimeout = 5 * time.Second

// Connect dials MongoDB and proves it with a ping, or refuses with the
// typed service_unavailable error. MongoDB is an OPTIONAL service in this
// guide — unlike PostgreSQL, an unreachable Mongo never fails the boot:
// the composition root logs the failure and keeps serving everything
// else, wiring the demo endpoints over the unavailable fallback instead.
// The failure stays typed end-to-end, so the handler's one mapping
// produces the honest 503 envelope — never a panic, never a 500, never a
// crashed process.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetConnectTimeout(connectTimeout))
	if err != nil {
		return nil, unavailable(err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, unavailable(err)
	}
	return client, nil
}

// unavailable is the one failure mapping this package applies: any driver
// error — dial, ping, operation — becomes the typed service_unavailable
// error carrying the cause, the same stance the pgx adapters take for an
// unreachable PostgreSQL.
func unavailable(cause error) error {
	return core.Wrap(cause, core.KindServiceUnavailable, "MongoDB is not reachable right now.")
}
