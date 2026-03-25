// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package lib

var dummy int

//go:noinline
func Foo() {
	dummy++
	InlinedFunc()
}

func InlinedFunc() {
	dummy++
}

// --- Cross-package generics ---

// Box is a generic container type from a different package than main.
type Box[T any] struct {
	Value T
}

// Map applies a function to the value in a Box, returning a new Box.
// Tests cross-package generic function with generic type parameter.
//
//go:noinline
func Map[A, B any](box Box[A], f func(A) B) Box[B] {
	return Box[B]{Value: f(box.Value)}
}

// Filter tests a generic function that takes a slice and a predicate.
// When called from another generic function, this tests subdicts.
//
//go:noinline
func Filter[T any](items []T, pred func(T) bool) []T {
	var result []T
	for _, item := range items {
		if pred(item) {
			result = append(result, item)
		}
	}
	return result
}
