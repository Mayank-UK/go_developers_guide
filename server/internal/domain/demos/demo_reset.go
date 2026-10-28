// The shared demo reset: one POST, every stateful demo area, honest
// per-area rows — guide infrastructure, not a curriculum topic (the
// fastapi guide's terminal-companion utility, mirrored). A reader
// following the guide mutates the running stack's demo state — toggled
// the scenario away from its baseline, tripped the process-lifetime
// breaker — and wants the constructed state back with one call. The
// use case orchestrates participants the composition root wires, every
// one through a reset path the area already owns (the scenario store's
// own update API, the breaker's Reset) — no new business logic
// anywhere, only existing seams.
//
// Honesty rules, in the family's grain. Every reset row carries the
// REAL result of this request's action — state read back after the
// reset ran, never a claim. A participant behind a typed taxonomy
// error degrades to a degraded row naming the failure while the other
// areas still reset; anything that is not a taxonomy error propagates
// (nothing is swallowed — it is a bug, not a degraded demo). And the
// report says what it deliberately does NOT touch: the store-backed
// substrates (PostgreSQL, MongoDB, Redis), the files demo's route
// store on disk, and the auth stories' accumulating stores —
// destructive resets are out of scope, and saying so in the payload
// beats surprising a reader who expected them.
package demos

import (
	"errors"
	"fmt"

	"guide/internal/core"
)

// DemoResetPath is the shared reset endpoint — registered on the real
// mux by the composition root, beside the scenario panel whose
// baseline it restores. POST-only: the router answers every other
// method with the truthful 405 envelope.
const DemoResetPath = "/api/demo/reset"

// DemoOutcomeReset and DemoOutcomeDegraded are what one area's action
// really achieved; degraded names the failure instead of claiming a
// reset.
const (
	DemoOutcomeReset    = "reset"
	DemoOutcomeDegraded = "degraded"
)

// DemoResetAction is one participant's reset: run the existing reset
// path, then report honestly. The detail string carries state read
// back AFTER the action ran — counts and named values from the call
// that just happened, never a promise.
type DemoResetAction func() (detail string, err error)

// DemoResetParticipant is one stateful demo area: a stable area key,
// a human label, and the reset action. Registered only at the
// composition root.
type DemoResetParticipant struct {
	// Area is the stable key the report row carries (scenario,
	// reliability, …).
	Area string
	// Label is the human name the report row shows.
	Label string
	// Action runs the area's reset.
	Action DemoResetAction
}

// DemoResetRow is one area's real result on the wire — the area's
// state read back after its action, never a claim.
type DemoResetRow struct {
	Area    string `json:"area"`
	Label   string `json:"label"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail"`
}

// DemoNotResetRow names an area deliberately untouched, with a reason
// a reader can check.
type DemoNotResetRow struct {
	Area   string `json:"area"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

// DemoResetReport is the whole sweep on the wire: what returned to the
// constructed state, and what never moved on purpose.
type DemoResetReport struct {
	Reset    []DemoResetRow    `json:"reset"`
	NotReset []DemoNotResetRow `json:"notReset"`
	Detail   string            `json:"detail"`
}

// demoNotResetAreas is the scope made explicit in every report: these
// never move, on purpose. Destructive database, queue, and disk resets
// are out of scope; the auth demo stores own no reset hook yet (every
// story is self-contained, but the stores live for the process).
var demoNotResetAreas = []DemoNotResetRow{
	{
		Area:   "postgres",
		Label:  "PostgreSQL tables + goose migrations",
		Reason: "destructive database resets are out of scope — migrations are the deploy's job, and the db demos reseed idempotently through their own entry points",
	},
	{
		Area:   "mongo",
		Label:  "MongoDB collections",
		Reason: "same destructive-scope rule — the mongo console owns its guide_demos namespace and drives it per request",
	},
	{
		Area:   "redis",
		Label:  "Redis — asynq queues, streams, and demo keys",
		Reason: "the jobs story tears its own demo keys down and TTL-bounds the rest; a queue purge is destructive and out of scope",
	},
	{
		Area:   "files",
		Label:  "The files demo's route store on disk",
		Reason: "uploads are the reader's own artifacts — deleting them is destructive and out of scope",
	},
	{
		Area:   "auth",
		Label:  "The auth stories' demo stores (sessions, API keys, refresh families, OAuth codes)",
		Reason: "every auth story is self-contained, but its sessions and keys land in the console's process-lifetime stores, which own no reset hook yet",
	},
}

// DemoResetter runs every registered participant's reset and assembles
// the report. One instance per process, built at the composition
// root; the sweep itself is a report, never a transaction.
type DemoResetter struct {
	participants []DemoResetParticipant
}

// NewDemoResetter validates the wiring — non-blank area and label, a
// non-nil action, no two participants sharing an area (a duplicate
// registration is a wiring bug the composition root must hear about at
// boot, the same stance the spring guide's reset registry takes) — and
// holds the participants in registration order until asked.
func NewDemoResetter(participants ...DemoResetParticipant) (*DemoResetter, error) {
	seen := make(map[string]bool, len(participants))
	for _, participant := range participants {
		switch {
		case participant.Area == "":
			return nil, fmt.Errorf("demo resetter: participant area must not be blank")
		case participant.Label == "":
			return nil, fmt.Errorf("demo resetter: participant %q has a blank label", participant.Area)
		case participant.Action == nil:
			return nil, fmt.Errorf("demo resetter: participant %q has a nil action", participant.Area)
		}
		if seen[participant.Area] {
			return nil, fmt.Errorf("demo resetter: two participants share the area %q", participant.Area)
		}
		seen[participant.Area] = true
	}
	return &DemoResetter{participants: append([]DemoResetParticipant(nil), participants...)}, nil
}

// RegisteredAreas names the registered areas in registration order —
// the report's vocabulary, derived from the registrations rather than
// restated anywhere else.
func (d *DemoResetter) RegisteredAreas() []string {
	areas := make([]string, 0, len(d.participants))
	for _, participant := range d.participants {
		areas = append(areas, participant.Area)
	}
	return areas
}

// Run resets every registered area, in registration order, and
// answers with the report. A participant failing with a typed
// taxonomy error degrades its row and nothing else — per-area
// isolation, because one area's outage must not cost the areas that
// owe it nothing. Any other error is a bug, not a degraded demo: it
// aborts the sweep and propagates to the caller for the 500 envelope,
// swallowed by no one.
func (d *DemoResetter) Run() (DemoResetReport, error) {
	rows := make([]DemoResetRow, 0, len(d.participants))
	for _, participant := range d.participants {
		detail, err := participant.Action()
		if err != nil {
			var appErr *core.Error
			if !errors.As(err, &appErr) {
				return DemoResetReport{}, err
			}
			rows = append(rows, DemoResetRow{
				Area:    participant.Area,
				Label:   participant.Label,
				Outcome: DemoOutcomeDegraded,
				Detail:  fmt.Sprintf("not reset — %s: %s", appErr.Kind, appErr.Message),
			})
			continue
		}
		rows = append(rows, DemoResetRow{
			Area:    participant.Area,
			Label:   participant.Label,
			Outcome: DemoOutcomeReset,
			Detail:  detail,
		})
	}
	return DemoResetReport{
		Reset:    rows,
		NotReset: append([]DemoNotResetRow(nil), demoNotResetAreas...),
		Detail: "every reset row carries this request's real result — state read back after " +
			"the action ran; degraded rows name the failure instead of claiming a reset, " +
			"and the not-reset rows are deliberate scope, not omissions",
	}, nil
}
