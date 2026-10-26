package teaching

import (
	"slices"
	"testing"
)

// topicHits exercises the ~ approximation: a DEFINED type whose underlying
// type is int still satisfies Numeric.
type topicHits int

func TestSumOverEveryNumericShape(t *testing.T) {
	if got := Sum([]int{1, 2, 3}); got != 6 {
		t.Errorf("Sum(ints) = %d, want 6", got)
	}
	if got := Sum([]float64{1.5, 2.5}); got != 4 {
		t.Errorf("Sum(floats) = %g, want 4", got)
	}
	if got := Sum([]topicHits{4, 8, 31}); got != 43 {
		t.Errorf("Sum(topicHits) = %d, want 43 — ~int must admit defined types", got)
	}
	if got := Sum[int](nil); got != 0 {
		t.Errorf("Sum(nil) = %d, want the zero value 0", got)
	}
}

func TestMaxUsesTheStdlibConstraint(t *testing.T) {
	if got := Max(7, 43); got != 43 {
		t.Errorf("Max(7, 43) = %d, want 43", got)
	}
	if got := Max("foundations", "ship"); got != "ship" {
		t.Errorf(`Max("foundations", "ship") = %q, want "ship"`, got)
	}
}

func TestMapTransformsBetweenTypes(t *testing.T) {
	lengths := Map([]string{"go", "server", "guide"}, func(s string) int { return len(s) })
	if !slices.Equal(lengths, []int{2, 6, 5}) {
		t.Errorf("Map lengths = %v, want [2 6 5]", lengths)
	}
	if got := Map([]int{}, func(i int) string { return "" }); got == nil || len(got) != 0 {
		t.Errorf("Map over empty = %v, want non-nil empty slice", got)
	}
}

func TestFilterKeepsMatchingElements(t *testing.T) {
	kept := Filter([]int{1, 2, 3, 4}, func(i int) bool { return i%2 == 0 })
	if !slices.Equal(kept, []int{2, 4}) {
		t.Errorf("Filter evens = %v, want [2 4]", kept)
	}
}

func TestSetIsAGenericTypeWithMethods(t *testing.T) {
	set := NewSet("text", "code", "demo")
	if !set.Has("demo") || set.Has("list") {
		t.Errorf("Set membership wrong for a 3-element set: %v", set)
	}
	if set.Len() != 3 {
		t.Errorf("Set.Len() = %d, want 3", set.Len())
	}
	if NewSet[int]().Len() != 0 {
		t.Error("empty Set must hold nothing")
	}
}
