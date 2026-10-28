// Topic 04 go-http-fundamentals' live demo: the running server introducing
// itself. Express fundamentals introspects a framework object; Go has no
// framework object to introspect — so this demo introspects the next honest
// thing: the binary itself. runtime/debug.ReadBuildInfo reports the module
// the server was compiled from (path, version, toolchain, dependency
// count), the runtime package reports the machine facts the scheduler
// actually sees, and the builder's one parameter carries the environment
// configuration the process booted with. The go-http-fundamentals topic
// quotes this file and its handler as the anatomy-of-a-handler exhibit.
package demos

import (
	"runtime"
	"runtime/debug"
	"sort"

	"guide/internal/core"
)

// ModuleInfo is who this binary is, exactly as debug.ReadBuildInfo reports
// it: the module path from go.mod, the main module's version, the toolchain
// that built it, and how many module dependencies the binary links (the
// linked set, which differs from go.mod's require list — the dependency
// topic teaches that manifest-versus-binary gap; each binary of this module
// reports its own count).
type ModuleInfo struct {
	Path            string `json:"path"`
	Version         string `json:"version"`
	GoVersion       string `json:"goVersion"`
	DependencyCount int    `json:"dependencyCount"`
}

// RuntimeInfo is the machine as the Go scheduler sees it. NumCPU is the
// logical CPUs available; GoMaxProcs is the GOMAXPROCS the process runs
// with — equal unless an explicit setting or a container CPU limit says
// otherwise (topic 02 tells that story; this demo just reports both).
type RuntimeInfo struct {
	NumCPU     int `json:"numCPU"`
	GoMaxProcs int `json:"goMaxProcs"`
}

// EnvironmentInfo is the typed configuration the running server actually
// holds: the port it listens on, the environment name, and the log level —
// the three values core.LoadConfig read from GDG_* variables at boot.
type EnvironmentInfo struct {
	Port     int    `json:"port"`
	Env      string `json:"env"`
	LogLevel string `json:"logLevel"`
}

// BuildSettingRow is one toolchain flag the build stamped into the binary
// (build mode, compiler, CGO and GOARCH/GOOS targets). Values may be empty
// strings — an unset CGO_CFLAGS is data, not absence.
type BuildSettingRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// AppInfoPayload is the wire shape of GET /api/demo/gohttp/app-info
// (camelCase JSON). Deterministic for a given binary and boot config: the
// build settings are sorted by key, and every collection is non-nil so the
// wire renders [] — never null — when empty.
type AppInfoPayload struct {
	Module        ModuleInfo        `json:"module"`
	Runtime       RuntimeInfo       `json:"runtime"`
	Environment   EnvironmentInfo   `json:"environment"`
	BuildSettings []BuildSettingRow `json:"buildSettings"`
}

// BuildAppInfoPayload observes the live process — build info, CPU facts —
// and folds them together with the booted configuration. The environment
// values arrive as a parameter because the builder must stay pure: the
// handler owns the config, the builder owns the shaping, and the same call
// always produces the same payload for the same inputs.
func BuildAppInfoPayload(cfg core.Config) AppInfoPayload {
	buildInfo, _ := debug.ReadBuildInfo()
	return buildAppInfoPayload(buildInfo, runtime.NumCPU(), runtime.GOMAXPROCS(0), cfg)
}

// buildAppInfoPayload shapes already-observed facts into the payload, which
// makes every branch testable with synthetic inputs — including the
// no-build-info guard: the go tool always stamps a Go binary, but a nil
// BuildInfo reports as empty strings and zero counts rather than panicking,
// so the payload stays honest in every context it can be built in (go run,
// go test, and a production binary each stamp slightly different facts).
func buildAppInfoPayload(buildInfo *debug.BuildInfo, numCPU, goMaxProcs int, cfg core.Config) AppInfoPayload {
	module := ModuleInfo{}
	if buildInfo != nil {
		module = ModuleInfo{
			Path:            buildInfo.Main.Path,
			Version:         buildInfo.Main.Version,
			GoVersion:       buildInfo.GoVersion,
			DependencyCount: len(buildInfo.Deps),
		}
	}
	settings := make([]BuildSettingRow, 0, len(buildInfoSettings(buildInfo)))
	for _, setting := range buildInfoSettings(buildInfo) {
		settings = append(settings, BuildSettingRow{Key: setting.Key, Value: setting.Value})
	}
	sort.Slice(settings, func(i, j int) bool { return settings[i].Key < settings[j].Key })
	return AppInfoPayload{
		Module:        module,
		Runtime:       RuntimeInfo{NumCPU: numCPU, GoMaxProcs: goMaxProcs},
		Environment:   EnvironmentInfo{Port: cfg.Port, Env: cfg.Env, LogLevel: cfg.LogLevel.String()},
		BuildSettings: settings,
	}
}

// buildInfoSettings reads a build info's settings nil-safely.
func buildInfoSettings(buildInfo *debug.BuildInfo) []debug.BuildSetting {
	if buildInfo == nil {
		return nil
	}
	return buildInfo.Settings
}
