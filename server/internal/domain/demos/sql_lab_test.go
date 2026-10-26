package demos

import (
	"testing"
)

// The SQL-lab builder's pins: three exhibits in order, each carrying its
// own SQL and teaching line, rows flattened to display cells, and the
// gap-less oldest row rendering its null as an explicit dash cell.

func TestDBSQLLabPayloadShapesTheThreeExhibits(t *testing.T) {
	t.Parallel()

	observation := SQLLabObservation{
		TagTotals: []TagTotalRow{
			{Tag: "api", Posts: 2},
			{Tag: "go", Posts: 3},
		},
		LengthRanks: []LengthRankRow{
			{Title: "Structured logging with slog", Runes: 233, Rank: 1},
			{Title: "Why Go for backends", Runes: 201, Rank: 2},
		},
		PublishGaps: []PublishGapRow{
			{Title: "Why Go for backends", Created: "2026-01-05", Days: 0, HasGap: false},
			{Title: "ServeMux patterns in Go 1.22", Created: "2026-01-07", Days: 2, HasGap: true},
		},
	}
	payload := BuildSQLLabPayload(observation)

	if len(payload.Queries) != 3 {
		t.Fatalf("exhibits = %d, want three", len(payload.Queries))
	}
	wantIDs := []string{"tag-totals", "rank-by-length", "publish-gaps"}
	for i, want := range wantIDs {
		if payload.Queries[i].ID != want {
			t.Errorf("exhibit %d = %q, want %q", i, payload.Queries[i].ID, want)
		}
		if payload.Queries[i].SQL == "" || payload.Queries[i].Teach == "" || len(payload.Queries[i].Columns) < 2 {
			t.Errorf("exhibit %q lacks its SQL, teach line, or columns", want)
		}
	}

	if got := payload.Queries[0].Rows; len(got) != 2 || got[0][0] != "api" || got[0][1] != 2 {
		t.Errorf("tag-totals rows drifted: %v", got)
	}
	if got := payload.Queries[2].Rows; len(got) != 2 || got[0][2] != "—" || got[1][2] != 2 {
		t.Errorf("gap rows must render null as the dash and gaps as days: %v", got)
	}
}

func TestDBSQLLabPayloadMaterializesEmptyRows(t *testing.T) {
	t.Parallel()

	payload := BuildSQLLabPayload(SQLLabObservation{})
	for _, query := range payload.Queries {
		if query.Rows == nil {
			t.Errorf("exhibit %q rendered null rows — the wire demands []", query.ID)
		}
	}
}
