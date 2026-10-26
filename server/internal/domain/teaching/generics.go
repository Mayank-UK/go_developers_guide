// Package teaching holds compilable teaching exhibits: small, real, tested
// implementations that exist because the guide's topics quote them, for
// mechanics the production code does not exercise yet. The first exhibit
// is the generics toolkit the go-language topic quotes; the snippet
// ratchet treats it like any other source of truth.
package teaching

import "cmp"

// Numeric is a constraint written as a union with underlying-type
// approximation: ~int means "any type whose UNDERLYING type is int", so
// type Hits int qualifies just as plain int does.
type Numeric interface {
	~int | ~int64 | ~float64
}

// Sum adds a slice of any Numeric, returning the type's own zero value
// when the slice is empty — var total T starts there, no special case.
func Sum[T Numeric](values []T) T {
	var total T
	for _, value := range values {
		total += value
	}
	return total
}

// Max returns the larger of two values of any ordered type. cmp.Ordered
// is the standard library's union constraint (~int | ~float64 | ~string
// and friends) — reach for it before writing your own.
func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// Map applies fn to every element of in and returns a new slice. T and U
// are independent type parameters, each constrained only by any.
func Map[T, U any](in []T, fn func(T) U) []U {
	out := make([]U, 0, len(in))
	for _, value := range in {
		out = append(out, fn(value))
	}
	return out
}

// Filter keeps the elements of in for which keep reports true.
func Filter[T any](in []T, keep func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, value := range in {
		if keep(value) {
			out = append(out, value)
		}
	}
	return out
}

// Set is a generic container type: value semantics over any comparable
// key — the classic [T comparable] payoff, since only comparable types can
// be map keys.
type Set[T comparable] map[T]struct{}

// NewSet builds a Set containing exactly the given values. The variadic
// type parameter list [T comparable] infers T from the arguments.
func NewSet[T comparable](values ...T) Set[T] {
	set := make(Set[T], len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

// Has reports whether the set contains value.
func (s Set[T]) Has(value T) bool {
	_, ok := s[value]
	return ok
}

// Len reports how many values the set holds.
func (s Set[T]) Len() int { return len(s) }
