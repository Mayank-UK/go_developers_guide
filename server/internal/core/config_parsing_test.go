package core

import (
	"log/slog"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults when unset",
			env:  nil,
			want: Config{Port: 8100, Env: "dev", LogLevel: slog.LevelInfo, GRPCAddr: ":8102", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name: "defaults when set to empty strings",
			env:  map[string]string{"GDG_PORT": "", "GDG_ENV": "", "GDG_LOG_LEVEL": "  ", "GRPC_ADDR": "  ", "GDG_DATABASE_URL": "  "},
			want: Config{Port: 8100, Env: "dev", LogLevel: slog.LevelInfo, GRPCAddr: ":8102", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name: "explicit valid values",
			env:  map[string]string{"GDG_PORT": "9001", "GDG_ENV": "prod", "GDG_LOG_LEVEL": "debug"},
			want: Config{Port: 9001, Env: "prod", LogLevel: slog.LevelDebug, GRPCAddr: ":8102", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name: "grpc address override is trimmed and honored",
			env:  map[string]string{"GRPC_ADDR": " :9002 "},
			want: Config{Port: 8100, Env: "dev", LogLevel: slog.LevelInfo, GRPCAddr: ":9002", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name: "database url override is trimmed and honored",
			env:  map[string]string{"GDG_DATABASE_URL": " postgres://postgres:guide@127.0.0.1:5433/guide?sslmode=disable "},
			want: Config{Port: 8100, Env: "dev", LogLevel: slog.LevelInfo, GRPCAddr: ":8102", DatabaseURL: "postgres://postgres:guide@127.0.0.1:5433/guide?sslmode=disable", MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name: "mongo uri and redis addr overrides are trimmed and honored",
			env:  map[string]string{"GDG_MONGO_URI": " mongodb://elsewhere:27018 ", "GDG_REDIS_ADDR": " elsewhere:6380 "},
			want: Config{Port: 8100, Env: "dev", LogLevel: slog.LevelInfo, GRPCAddr: ":8102", DatabaseURL: defaultDatabaseURL, MongoURI: "mongodb://elsewhere:27018", RedisAddr: "elsewhere:6380"},
		},
		{
			name: "log level is case-insensitive",
			env:  map[string]string{"GDG_LOG_LEVEL": "WARN"},
			want: Config{Port: 8100, Env: "dev", LogLevel: slog.LevelWarn, GRPCAddr: ":8102", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name: "surrounding whitespace is trimmed",
			env:  map[string]string{"GDG_PORT": " 9001 ", "GDG_ENV": " prod "},
			want: Config{Port: 9001, Env: "prod", LogLevel: slog.LevelInfo, GRPCAddr: ":8102", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name: "boundary port 1",
			env:  map[string]string{"GDG_PORT": "1"},
			want: Config{Port: 1, Env: "dev", LogLevel: slog.LevelInfo, GRPCAddr: ":8102", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name: "boundary port 65535",
			env:  map[string]string{"GDG_PORT": "65535"},
			want: Config{Port: 65535, Env: "dev", LogLevel: slog.LevelInfo, GRPCAddr: ":8102", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},
		},
		{
			name:    "port not a number",
			env:     map[string]string{"GDG_PORT": "eighty"},
			wantErr: true,
		},
		{
			name:    "port zero",
			env:     map[string]string{"GDG_PORT": "0"},
			wantErr: true,
		},
		{
			name:    "port negative",
			env:     map[string]string{"GDG_PORT": "-1"},
			wantErr: true,
		},
		{
			name:    "port above 65535",
			env:     map[string]string{"GDG_PORT": "65536"},
			wantErr: true,
		},
		{
			name:    "unknown log level",
			env:     map[string]string{"GDG_LOG_LEVEL": "trace"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := LoadConfig(func(key string) string { return tt.env[key] })

			if tt.wantErr {
				if err == nil {
					t.Fatalf("LoadConfig(%v) error = nil, want an error", tt.env)
				}
				// Errors must name the offending GDG_ variable to be
				// actionable at boot.
				if !strings.Contains(err.Error(), "GDG_") {
					t.Errorf("error %q does not name a GDG_ variable", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig(%v) unexpected error: %v", tt.env, err)
			}
			if got != tt.want {
				t.Errorf("LoadConfig(%v) = %+v, want %+v", tt.env, got, tt.want)
			}
		})
	}
}
