package demos

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"guide/internal/core"
)

// The go-http-fundamentals topic's prose claims specific facts about this
// payload ("module guide, the deliberate direct requires, (devel) until VCS
// stamping, build settings sorted"); these tests pin those claims next to
// the code that must keep them true — a toolchain or config change that
// moves any fact fails HERE first.

func appInfoTestConfig() core.Config {
	return core.Config{Port: 8100, Env: "dev", LogLevel: slog.LevelInfo}
}

func TestAppInfoModuleFactsMatchTheLiveBuildInfo(t *testing.T) {
	payload := BuildAppInfoPayload(appInfoTestConfig())
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("a go-built test binary always carries build info")
	}
	if payload.Module.Path != buildInfo.Main.Path || payload.Module.Path != "guide" {
		t.Errorf("module path = %q, want the guide module", payload.Module.Path)
	}
	if payload.Module.GoVersion != buildInfo.GoVersion || !strings.HasPrefix(payload.Module.GoVersion, "go1.") {
		t.Errorf("go version = %q, want the live toolchain's go1.x string", payload.Module.GoVersion)
	}
	// 16 = the linked-module count of THIS package's test binary (go.mod
	// advertises 15 direct requires — the dependency topic teaches that
	// manifest-versus-binary gap); pinned against the live build info so
	// any link-graph change fails here first.
	if payload.Module.DependencyCount != len(buildInfo.Deps) || payload.Module.DependencyCount != 16 {
		t.Errorf("dependency count = %d, want 16 (this test binary's linked-module count, confirmed live)", payload.Module.DependencyCount)
	}
	// go run, go test, and an untagged go build all report "(devel)"; a
	// VCS-stamped release build reports its version. Either is honest —
	// pin only that the field is never blank.
	if payload.Module.Version == "" {
		t.Error("module version is blank; ReadBuildInfo always stamps (devel) or a real version")
	}
}

func TestAppInfoRuntimeMatchesTheLiveProcess(t *testing.T) {
	payload := BuildAppInfoPayload(appInfoTestConfig())
	if payload.Runtime.NumCPU != runtime.NumCPU() || payload.Runtime.NumCPU <= 0 {
		t.Errorf("numCPU = %d, want the live runtime.NumCPU() > 0", payload.Runtime.NumCPU)
	}
	if payload.Runtime.GoMaxProcs != runtime.GOMAXPROCS(0) || payload.Runtime.GoMaxProcs <= 0 {
		t.Errorf("goMaxProcs = %d, want the live GOMAXPROCS(0) > 0", payload.Runtime.GoMaxProcs)
	}
}

func TestAppInfoEnvironmentEchoesTheBootedConfig(t *testing.T) {
	cfg := core.Config{Port: 8123, Env: "prod", LogLevel: slog.LevelWarn}
	env := BuildAppInfoPayload(cfg).Environment
	if env.Port != 8123 || env.Env != "prod" || env.LogLevel != "WARN" {
		t.Errorf("environment = %+v, want the booted config verbatim (8123/prod/WARN)", env)
	}
}

func TestAppInfoBuildSettingsAreSortedUniqueAndComplete(t *testing.T) {
	settings := BuildAppInfoPayload(appInfoTestConfig()).BuildSettings
	live, _ := debug.ReadBuildInfo()
	if len(settings) != len(live.Settings) {
		t.Fatalf("build settings = %d rows, want the live %d", len(settings), len(live.Settings))
	}
	seen := make(map[string]bool, len(settings))
	for i, row := range settings {
		if row.Key == "" {
			t.Errorf("row %d has a blank key: %+v", i, row)
		}
		if seen[row.Key] {
			t.Errorf("duplicate build setting key %q", row.Key)
		}
		seen[row.Key] = true
		if i > 0 && settings[i-1].Key > row.Key {
			t.Errorf("build settings not sorted: %q before %q", settings[i-1].Key, row.Key)
		}
	}
}

func TestAppInfoWithoutBuildInfoStaysHonest(t *testing.T) {
	payload := buildAppInfoPayload(nil, 4, 2, appInfoTestConfig())
	if !reflect.DeepEqual(payload.Module, ModuleInfo{}) {
		t.Errorf("module = %+v, want zeroed module facts when no build info exists", payload.Module)
	}
	if payload.Runtime.NumCPU != 4 || payload.Runtime.GoMaxProcs != 2 {
		t.Errorf("runtime = %+v, want the observed facts passed in", payload.Runtime)
	}
	if payload.BuildSettings == nil {
		t.Error("buildSettings must never be nil; the wire contract renders [] not null")
	}

	// The wire shape itself: camelCase keys, and an empty settings
	// collection marshals as [] — never null.
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"buildSettings":[]`) {
		t.Errorf("empty buildSettings must marshal as []: %s", raw)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"module", "runtime", "environment", "buildSettings"} {
		if _, present := decoded[key]; !present {
			t.Errorf("payload key %q missing from the wire shape", key)
		}
	}
	module, _ := decoded["module"].(map[string]any)
	for _, key := range []string{"path", "version", "goVersion", "dependencyCount"} {
		if _, present := module[key]; !present {
			t.Errorf("module key %q missing from the wire shape", key)
		}
	}
}

func TestAppInfoBuilderIsPure(t *testing.T) {
	first := buildAppInfoPayload(&debug.BuildInfo{GoVersion: "go1.27.3"}, 8, 8, appInfoTestConfig())
	second := buildAppInfoPayload(&debug.BuildInfo{GoVersion: "go1.27.3"}, 8, 8, appInfoTestConfig())
	if !reflect.DeepEqual(first, second) {
		t.Error("builder must be pure: same inputs returned different payloads")
	}
	if len(first.BuildSettings) != 0 {
		t.Errorf("a zero BuildInfo carries no settings; got %d rows", len(first.BuildSettings))
	}
}
