// Package core holds the cross-cutting server plumbing: typed environment
// configuration and structured logging. The error taxonomy, request-id
// middleware, and ScenarioStore arrive in Phase 1.
package core

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// Environment variable names — the family pattern of a per-guide prefix
// (here GDG_, matching the fastapi guide's FDG_).
const (
	envPort        = "GDG_PORT"
	envEnv         = "GDG_ENV"
	envLogLevel    = "GDG_LOG_LEVEL"
	envGRPCAddr    = "GRPC_ADDR"
	envDatabaseURL = "GDG_DATABASE_URL"
	envMongoURI    = "GDG_MONGO_URI"
	envRedisAddr   = "GDG_REDIS_ADDR"
)

// Defaults applied when a variable is unset or empty: dev-safe values, no
// secrets, so the stack boots anywhere with zero configuration.
const (
	defaultPort     = 8100
	defaultEnv      = "dev"
	defaultLogLevel = slog.LevelInfo
	defaultGRPCAddr = ":8102"
	// defaultDatabaseURL points at the dev stack's PostgreSQL by its
	// compose-network service name (host port 5433 maps to 5432 inside
	// the network) with the compose-declared credentials. PostgreSQL is
	// part of the stack — not an optional service — so the server migrates
	// it at boot and fails fast when it is unreachable. DSN SHAPE is the
	// driver's to validate (at dial time), not config's.
	defaultDatabaseURL = "postgres://postgres:guide@postgres:5432/guide?sslmode=disable"
	// defaultMongoURI points at the compose-network mongo service — but
	// MongoDB is an OPTIONAL service (the mongodb topic's own discipline):
	// boot-connect degrades to the honest unavailable demo, never a boot
	// failure, so the URI's validity surfaces there, not here.
	defaultMongoURI = "mongodb://mongo:27017"
	// defaultRedisAddr points at the compose-network redis service. Redis
	// is wired LAZY — no boot ping at all — so the client dials on first
	// use and every failure maps to the demo's typed 503 at request time.
	defaultRedisAddr = "redis:6379"
)

// Config is the validated, typed application configuration, read once at
// boot and immutable by convention afterwards.
type Config struct {
	// Port is the TCP port the HTTP server listens on (GDG_PORT, 1-65535).
	Port int
	// Env names the deployment environment, e.g. "dev" or "prod" (GDG_ENV).
	Env string
	// LogLevel gates the slog JSON logger (GDG_LOG_LEVEL: debug, info,
	// warn, or error).
	LogLevel slog.Level
	// GRPCAddr is the TCP address the gRPC listener binds (GRPC_ADDR, a
	// host:port listen address; the guide's reserved port is :8102). An
	// unusable address fails at the net.Listen call at boot, never here.
	GRPCAddr string
	// DatabaseURL is the PostgreSQL DSN the server migrates and queries
	// (GDG_DATABASE_URL, a URL-shaped connection string). Unparseable or
	// unreachable values are a BOOT failure — caught when the pool dials,
	// never here: this struct carries configuration, not validation of
	// another package's syntax.
	DatabaseURL string
	// MongoURI is the connection string for the optional MongoDB service
	// (GDG_MONGO_URI). The server boot-connects with a bounded window and
	// DEGRADES the mongo demo when it is unreachable — never fails boot.
	MongoURI string
	// RedisAddr is the address of the optional Redis service
	// (GDG_REDIS_ADDR). The client is lazy — no boot ping — so an
	// unreachable Redis surfaces only as the demo's typed 503.
	RedisAddr string
}

// LoadConfig builds a Config from the environment through getenv, which is
// parameterized so tests inject values instead of mutating the process
// environment. Unset or empty variables fall back to dev-safe defaults;
// malformed values return a descriptive error naming the offending
// variable — configuration problems fail at boot, never panic.
func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:        defaultPort,
		Env:         defaultEnv,
		LogLevel:    defaultLogLevel,
		GRPCAddr:    defaultGRPCAddr,
		DatabaseURL: defaultDatabaseURL,
		MongoURI:    defaultMongoURI,
		RedisAddr:   defaultRedisAddr,
	}

	if raw := strings.TrimSpace(getenv(envPort)); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s %q: not a valid port number", envPort, raw)
		}
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("%s %q: port must be between 1 and 65535", envPort, raw)
		}
		cfg.Port = port
	}

	if raw := strings.TrimSpace(getenv(envEnv)); raw != "" {
		cfg.Env = raw
	}

	if raw := strings.TrimSpace(getenv(envLogLevel)); raw != "" {
		var level slog.Level
		if err := level.UnmarshalText([]byte(raw)); err != nil {
			return Config{}, fmt.Errorf("%s %q: must be one of debug, info, warn, error", envLogLevel, raw)
		}
		cfg.LogLevel = level
	}

	if raw := strings.TrimSpace(getenv(envGRPCAddr)); raw != "" {
		cfg.GRPCAddr = raw
	}

	if raw := strings.TrimSpace(getenv(envDatabaseURL)); raw != "" {
		cfg.DatabaseURL = raw
	}

	if raw := strings.TrimSpace(getenv(envMongoURI)); raw != "" {
		cfg.MongoURI = raw
	}

	if raw := strings.TrimSpace(getenv(envRedisAddr)); raw != "" {
		cfg.RedisAddr = raw
	}

	return cfg, nil
}
