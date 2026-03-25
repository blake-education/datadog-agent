// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package main

import (
	"fmt"
	"strconv"

	"github.com/DataDog/datadog-agent/pkg/dyninst/testprogs/progs/sample/lib"
)

// --- Generic types ---

// typeWithGenerics is a generic struct with a comparable constraint.
type typeWithGenerics[V comparable] struct {
	Value V
}

// namedInt is a named type with the same underlying type as int.
// It shares go.shape.int but gets a different dictionary entry.
type namedInt int

// Pair is a generic struct with two type parameters.
type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

// --- Interface for method constraints ---

// doer is a non-basic interface (has a method).
// Type params constrained by this do NOT get pointer collapsing.
type doer interface {
	DoSomething() string
}

// largeGenericType is a generic struct large enough to be passed by hidden
// pointer per Go ABI (>128 bytes). This shifts the dict register position:
// the receiver becomes a hidden pointer in int reg 0, and the dict lands
// in int reg 1 (same as pointer-receiver methods).
type largeGenericType[V comparable] struct {
	A, B, C, D, E, F, G, H [16]byte // 128 bytes
	Value                   V        // pushes total over 128 bytes
}

// --- Methods on generic types ---

// Guess is a value-receiver method on a generic type.
//
//nolint:all
//go:noinline
func (x typeWithGenerics[V]) Guess(value V) bool {
	return x.Value == value
}

// SetValue is a pointer-receiver method on a generic type.
// Tests dict position when receiver is a pointer (occupies one int register).
//
//nolint:all
//go:noinline
func (x *Pair[K, V]) SetValue(k K, v V) {
	x.Key = k
	x.Value = v
}

// --- Generic free functions ---

// genericContains uses a comparable constraint.
// int and namedInt share the same shape, testing dict discrimination.
//
//nolint:all
//go:noinline
func genericContains[T comparable](haystack []T, needle T) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// genericDeref uses an unconstrained (any) type parameter.
// For basic interfaces, all pointer args *T collapse to *uint8 shape.
// The pointed-to types are different shapes (string vs bigStruct).
//
//nolint:all
//go:noinline
func genericDeref[T any](p *T) T {
	return *p
}

// genericDo uses a non-basic interface constraint (has a method).
// Pointer collapsing does NOT apply — firstBehavior and secondBehavior
// have different underlying struct types, so they get distinct shapes.
//
//nolint:all
//go:noinline
func genericDo[T doer](val T) string {
	return val.DoSomething()
}

// genericSwap uses two type params with the same constraint.
// Tests multi-param dictionary layout.
//
//nolint:all
//go:noinline
func genericSwap[A, B any](a A, b B) (B, A) {
	return b, a
}

// LargeGet is a value-receiver method on a >128-byte generic struct.
// Per Go ABI, the receiver is passed by hidden pointer (one int register),
// so the dict lands in int register 1, same position as pointer-receiver
// methods.
//
//nolint:all
//go:noinline
func (x largeGenericType[V]) LargeGet() V {
	return x.Value
}

// genericMax is an inlineable generic function (no //go:noinline).
// Small enough to be inlined, but has actual logic so the compiler
// preserves instructions. When inlined, the dict parameter is optimized
// away. Tests graceful fallback to shape types for inlined generics.
//
//nolint:all
func genericMax[T interface{ ~int | ~float64 | ~string }](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// --- Named types sharing shapes ---

// score is a named type with underlying type int.
type score int

// celsius is another named type with underlying type int.
// score, celsius, namedInt, and int all share go.shape.int.
type celsius int

// --- Pointer collapsing types ---

// smallStruct is a small struct for pointer collapsing tests.
type smallStruct struct {
	X int
	Y int
}

// tinyStruct is another small struct — different type, same pointer shape
// when constraint is `any` (basic interface → all pointers collapse to *uint8).
type tinyStruct struct {
	Name string
}

// genericPtrIdentity takes a pointer with `any` constraint.
// All pointer args collapse to *uint8 shape, regardless of the pointee.
// This means *smallStruct, *tinyStruct, *int all share one shape function
// but each call has a different dict entry.
//
//nolint:all
//go:noinline
func genericPtrIdentity[T any](p *T) *T {
	return p
}

// --- Complex generic scenarios ---

// outerFilter calls lib.Filter from a generic context, testing subdicts.
// Probing lib.Filter should work — it gets its own dict from outerFilter's
// subdict entry.
//
//nolint:all
//go:noinline
func outerFilter[T any](items []T, pred func(T) bool) []T {
	return lib.Filter(items, pred)
}

// wrapBox wraps a value in a lib.Box from a different package.
// Tests cross-package generic type instantiation.
//
//nolint:all
//go:noinline
func wrapBox[T any](val T) lib.Box[T] {
	return lib.Box[T]{Value: val}
}

// nestedGeneric takes a Box[T] — a generic parameter that is itself a
// generic type instantiation. Tests deeply nested shape types like
// lib.Box[go.shape.int].
//
//nolint:all
//go:noinline
func nestedGeneric[T any](box lib.Box[T]) T {
	return box.Value
}

// --- Execution ---

//nolint:all
func executeGenericFuncs() {
	// Value-receiver method on generic type, two different shapes.
	x := typeWithGenerics[string]{Value: "generics work"}
	x.Guess("generics work")
	y := typeWithGenerics[int]{Value: 42}
	y.Guess(21)

	// Comparable free function — int and string are different shapes.
	genericContains([]string{"a", "b", "c"}, "b")
	genericContains([]int{1, 2, 3}, 2)

	// Named type sharing go.shape.int — same shape function as int,
	// different dict entry.
	genericContains([]namedInt{1, 2, 3}, namedInt(2))

	// Unconstrained pointer (any) — *bigStruct and *string collapse
	// to the same *uint8 pointer shape for the pointer itself,
	// but T has different shapes (string vs struct).
	bs := bigStruct{z: 1}
	genericDeref(&bs)
	s := "hello"
	genericDeref(&s)

	// Non-basic interface constraint — no pointer collapsing.
	// firstBehavior and secondBehavior have different underlying types.
	genericDo(firstBehavior{s: "first"})
	genericDo(secondBehavior{i: 42})

	// Multiple type params.
	genericSwap[int, string](42, "hello")
	genericSwap[string, bool]("world", true)

	// Pointer-receiver method on generic type.
	p := Pair[string, int]{}
	p.SetValue("key", 123)
	q := Pair[int, bool]{}
	q.SetValue(1, true)

	// Large value-receiver method — receiver >128 bytes, passed by
	// hidden pointer per Go ABI. Dict shifts to int reg 1.
	lg := largeGenericType[int]{Value: 99}
	lg.LargeGet()
	lgs := largeGenericType[string]{Value: "large"}
	lgs.LargeGet()

	// Inlineable generic function — when inlined, the dict parameter
	// is optimized away. Tests graceful fallback to shape types.
	_ = genericMax(10, 20)
	_ = genericMax(3.14, 2.72)
	_ = genericMax("abc", "xyz")

	// Multiple named types sharing go.shape.int.
	// score, celsius, namedInt all have underlying type int.
	// Each call uses the same shape function but a different dict.
	genericContains([]score{10, 20, 30}, score(20))
	genericContains([]celsius{0, 100}, celsius(100))

	// Pointer collapsing: *smallStruct, *tinyStruct, *int all collapse
	// to the same *uint8 shape under `any` constraint. The dict is the
	// only way to tell them apart at runtime.
	ss := smallStruct{X: 1, Y: 2}
	genericPtrIdentity(&ss)
	ts := tinyStruct{Name: "tiny"}
	genericPtrIdentity(&ts)
	ii := 42
	genericPtrIdentity(&ii)

	// Cross-package generic function (lib.Map with Box).
	intBox := lib.Box[int]{Value: 42}
	strBox := lib.Map(intBox, strconv.Itoa)

	// Generic calling another generic (outerFilter → lib.Filter).
	// Probing lib.Filter tests subdict resolution.
	filtered := outerFilter([]int{1, 2, 3, 4, 5}, func(x int) bool { return x > 2 })

	// Nested generic: Box[T] as a type parameter.
	nested := nestedGeneric(lib.Box[string]{Value: "nested"})
	nestedInt := nestedGeneric(lib.Box[int]{Value: 99})

	// Cross-package generic type with wrapBox.
	wb := wrapBox("boxed")
	wbi := wrapBox(123)

	// Prevent dead code elimination.
	fmt.Println(x, y, bs, s, p, q, lg, lgs, strBox, filtered, nested, nestedInt, wb, wbi)
}
