// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package gosymname

// GenericParams holds the text of generic type parameters from a symbol name.
type GenericParams struct {
	// Raw is the text between the outermost '[' and ']', not including the
	// brackets themselves.
	Raw string
	// Start is the byte offset of '[' in the raw symbol name.
	Start int
	// End is the byte offset of the matching ']' in the raw symbol name.
	End int
}

// matchBracket finds the matching ']' for the '[' at position start in s,
// handling nested brackets. Returns the index of the matching ']', or -1 if
// not found.
func matchBracket(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
