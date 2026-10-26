//go:build tools

// Package tools pre-lands the Phase 6 (data layer) and Phase 7 (auth/
// security) library pins: these are deliberate go.mod requires with no
// importing server package yet, so the blank imports below keep them
// anchored through `go mod tidy`. Each one is absorbed by its real import
// when its topic lands (pgx/gorm/go-redis/mongo-driver/testcontainers in
// topics 16-20, x/crypto argon2id in topic 23, x/time rate in topic 25) —
// this file shrinks away as that happens and is removed once empty.
//
// The `tools` build tag keeps this package out of every normal build and
// test binary; only `go mod tidy` (and explicit `-tags tools` builds) see it.
package tools

import (
	_ "github.com/jackc/pgx/v5"
	_ "github.com/redis/go-redis/v9"
	_ "github.com/testcontainers/testcontainers-go"
	_ "go.mongodb.org/mongo-driver/v2/mongo"
	_ "golang.org/x/crypto/argon2"
	_ "golang.org/x/time/rate"
	_ "gorm.io/driver/postgres"
	_ "gorm.io/gorm"
)
