package jobs

import (
	"strings"
	"testing"
	"time"
)

// The jobs ports are interfaces — their contract lives in the adapters'
// tests (internal/data/jobs). What THIS file pins is the one piece of
// behavior the domain owns: a TaskSpec that would confuse the broker is
// refused here, by name, before any store is touched.

func TestTaskSpecValidateAcceptsAMinimalSpec(t *testing.T) {
	spec := TaskSpec{ID: "t-1", Type: "demo:digest", Payload: []byte(`{}`), Queue: "demos"}
	if err := spec.Validate(); err != nil {
		t.Fatalf("minimal spec must validate: %v", err)
	}
}

func TestTaskSpecValidateRefusesBlankAndNegativeFields(t *testing.T) {
	valid := TaskSpec{ID: "t-1", Type: "demo:digest", Queue: "demos"}
	cases := []struct {
		name   string
		mutate func(*TaskSpec)
		want   string
	}{
		{"blank id", func(s *TaskSpec) { s.ID = "" }, "id must not be blank"},
		{"blank type", func(s *TaskSpec) { s.Type = "" }, "type must not be blank"},
		{"blank queue", func(s *TaskSpec) { s.Queue = "" }, "queue must not be blank"},
		{"negative retry", func(s *TaskSpec) { s.MaxRetry = -1 }, "must not be negative"},
	}
	for _, tc := range cases {
		spec := valid
		tc.mutate(&spec)
		err := spec.Validate()
		if err == nil {
			t.Errorf("%s: validate must refuse", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q must name %q", tc.name, err.Error(), tc.want)
		}
	}
}

func TestTaskSpecCarriesTheRetryVocabulary(t *testing.T) {
	// A spec exercising every field — the shape the asynq adapter maps
	// onto broker options one-for-one; pinned so the mapping's source
	// (this struct) cannot drift silently.
	spec := TaskSpec{
		ID: "t-2", Type: "demo:flaky", Payload: []byte(`{"run":"r"}`),
		Queue: "demos", MaxRetry: 2, ProcessIn: 900 * time.Millisecond,
		Timeout: 5 * time.Second,
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("full spec must validate: %v", err)
	}
	if spec.MaxRetry != 2 || spec.ProcessIn != 900*time.Millisecond || spec.Timeout != 5*time.Second {
		t.Errorf("spec fields drifted: %+v", spec)
	}
}
