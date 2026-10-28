// Package snapshot pins the module graph's selected versions: the committed
// module-snapshot.txt beside this file records `go list -m all` — one line
// per module MVS selected for the guide server, main module first — and the
// test in this package regenerates that listing and fails on any drift.
//
// The artifact is the Go counterpart of the spring guide's
// dependency-tree-snapshot.txt: with no hosted CI (family stance), the
// committed snapshot plus a local drift test is what makes an unconscious
// version change loud. The artifact is PURE payload — RegenerateCommand
// reproduces it byte-for-byte, so regeneration never needs a hand-restored
// header. The test tolerates (and strips) `#` comment lines should a future
// header be added deliberately. It regenerates offline (GOPROXY=off) against
// the warm module cache (the dev container's gdg-go volume), so it never
// touches the network and proves that property by failing if the cache
// cannot answer.
package snapshot

const (
	// ArtifactName is the committed listing this package pins, relative to
	// this package's directory.
	ArtifactName = "module-snapshot.txt"

	// RegenerateCommand is the exact command that refreshes the artifact
	// after an intentional dependency change — run it from server/ and
	// commit the result together with the go.mod/go.sum change.
	RegenerateCommand = "go list -m all > tools/snapshot/module-snapshot.txt"
)
