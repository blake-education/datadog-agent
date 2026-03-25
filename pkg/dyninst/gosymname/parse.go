// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package gosymname

import (
	"strings"
	"unicode/utf8"
)

// Parse parses a Go symbol name and returns a Symbol with the package split
// computed eagerly and interpretation generation deferred.
func Parse(name string, source SymbolSource) Symbol {
	var s Symbol
	ParseInto(&s, name, source)
	return s
}

// ParseInto parses a Go symbol name into an existing Symbol, avoiding
// allocation of the Symbol struct itself.
func ParseInto(dst *Symbol, name string, source SymbolSource) {
	*dst = Symbol{
		raw:    name,
		source: source,
	}

	// Strip ABI suffix for nm source before classification.
	effective := name
	if source.hasABISuffix() {
		effective, _ = stripABISuffix(name)
	}

	// Quick classification before full split.
	dst.class = classify(effective, source)
	if dst.class == ClassCompilerGenerated {
		dst.pkg = ""
		dst.local = effective
		if strings.HasPrefix(effective, "go:") || strings.HasPrefix(effective, "type:") || strings.HasPrefix(effective, "type..") {
			dst.local = effective[strings.IndexByte(effective, ':')+1:]
		}
		return
	}
	if dst.class == ClassBareName {
		dst.pkg = ""
		dst.local = effective
		return
	}
	if dst.class == ClassCFunction {
		dst.pkg = ""
		dst.local = effective
		return
	}

	// Split package from local name.
	escapedPkg, local := splitPkg(effective)
	if escapedPkg == "" {
		// No package found — treat as bare name.
		dst.pkg = ""
		dst.local = effective
		if dst.class != ClassGlobalClosure {
			dst.class = ClassBareName
		}
		return
	}

	// Unescape the package path.
	pkg, err := unescapePkg(escapedPkg)
	if err != nil {
		// Malformed escape — best effort, use raw.
		pkg = escapedPkg
	}
	dst.pkg = pkg
	dst.local = local

	// Handle global closure (glob.funcN).
	if pkg == "glob" {
		dst.class = ClassGlobalClosure
		return
	}

	// Re-classify based on local name now that we have it.
	dst.class = classifyLocal(local, dst.class)
}

// SplitPackage splits a symbol name into its package path and local name.
// The package path is unescaped.
func SplitPackage(name string, source SymbolSource) (pkg, local string) {
	effective := name
	if source.hasABISuffix() {
		effective, _ = stripABISuffix(name)
	}
	escapedPkg, local := splitPkg(effective)
	if escapedPkg == "" {
		return "", effective
	}
	pkg, err := unescapePkg(escapedPkg)
	if err != nil {
		return escapedPkg, local
	}
	return pkg, local
}

// stripABISuffix removes a trailing .abi0 or .abiinternal suffix and returns
// the name without the suffix and the suffix itself (without the leading dot).
func stripABISuffix(name string) (stripped, suffix string) {
	if strings.HasSuffix(name, ".abi0") {
		return name[:len(name)-5], "abi0"
	}
	if strings.HasSuffix(name, ".abiinternal") {
		return name[:len(name)-12], "abiinternal"
	}
	return name, ""
}

// classify performs quick classification of a symbol name.
func classify(name string, source SymbolSource) SymbolClass {
	// Compiler-generated prefixes.
	if strings.HasPrefix(name, "go:") || strings.HasPrefix(name, "type:") || strings.HasPrefix(name, "type..") {
		return ClassCompilerGenerated
	}
	// Compiler-generated suffixes/infixes.
	if strings.Contains(name, "..inittask") || strings.Contains(name, "..stmp_") || strings.Contains(name, "..dict.") {
		return ClassCompilerGenerated
	}

	// Global closure.
	if strings.HasPrefix(name, "glob.") {
		return ClassGlobalClosure
	}

	// C function heuristic: no '/' and has GCC optimization suffixes.
	if !strings.ContainsRune(name, '/') {
		if strings.Contains(name, ".isra.") || strings.Contains(name, ".part.") || strings.Contains(name, ".constprop.") {
			return ClassCFunction
		}
	}

	// Bare name: no '.' and no '/'.
	if !strings.ContainsRune(name, '.') && !strings.ContainsRune(name, '/') {
		return ClassBareName
	}

	// Default — further refined after package split.
	_ = source
	return ClassFunction
}

// classifyLocal refines classification based on the local name (after package
// split).
func classifyLocal(local string, current SymbolClass) SymbolClass {
	// Map init: map.init.N
	if strings.HasPrefix(local, "map.init.") {
		return ClassMapInit
	}

	// Init function: init or init.N
	if local == "init" || strings.HasPrefix(local, "init.") {
		allDigits := true
		suffix := ""
		if len(local) > 5 {
			suffix = local[5:]
		}
		for _, c := range suffix {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if local == "init" || (suffix != "" && allDigits) {
			return ClassInit
		}
	}

	// Closure detection: .funcN, .gowrapN, .deferwrapN, -rangeN anywhere.
	if hasClosure(local) {
		return ClassClosure
	}

	return current
}

// hasClosure returns true if the local name contains closure markers.
func hasClosure(local string) bool {
	for i := 0; i < len(local); i++ {
		if local[i] == '.' {
			rest := local[i+1:]
			if matchPrefix(rest, "func") || matchPrefix(rest, "gowrap") || matchPrefix(rest, "deferwrap") {
				return true
			}
		}
		if local[i] == '-' {
			rest := local[i+1:]
			if matchPrefix(rest, "range") {
				return true
			}
		}
	}
	return false
}

// matchPrefix checks if s starts with prefix followed by a digit.
func matchPrefix(s, prefix string) bool {
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	rest := s[len(prefix):]
	if len(rest) == 0 {
		// "deferwrap" with no number is still valid
		return prefix == "deferwrap" || prefix == "gowrap"
	}
	return rest[0] >= '0' && rest[0] <= '9'
}

// segment types used during chain decomposition.
type segmentKind int

const (
	segName    segmentKind = iota // plain identifier
	segPtrRecv                    // (*Type).Method
	segClosure                    // funcN, gowrapN, deferwrapN
	segNesting                    // bare number (closure nesting)
	segRange                      // -rangeN
	segFM                         // -fm
)

type segment struct {
	kind segmentKind
	text string // raw text of this segment

	// For segPtrRecv:
	receiver string
	recvGen  *GenericParams // receiver generic params
	method   string
	methGen  *GenericParams // method generic params

	// For segName:
	name    string
	nameGen *GenericParams
}

// buildInterpretations performs full chain decomposition and generates
// interpretations for the symbol.
func buildInterpretations(s *Symbol) {
	switch s.class {
	case ClassCompilerGenerated:
		s.interps = append(s.interps, Interpretation{
			OuterFunction: s.raw,
			Confidence:    1.0,
		})
		return
	case ClassBareName:
		s.interps = append(s.interps, Interpretation{
			OuterFunction: s.local,
			Confidence:    1.0,
		})
		return
	case ClassCFunction:
		s.interps = append(s.interps, Interpretation{
			OuterFunction: s.local,
			Confidence:    1.0,
		})
		return
	case ClassGlobalClosure:
		s.interps = append(s.interps, Interpretation{
			Package:       s.pkg,
			OuterFunction: s.local,
			Confidence:    1.0,
		})
		return
	case ClassMapInit:
		s.interps = append(s.interps, Interpretation{
			Package:       s.pkg,
			OuterFunction: s.local,
			Confidence:    1.0,
		})
		return
	case ClassInit:
		s.interps = append(s.interps, Interpretation{
			Package:       s.pkg,
			OuterFunction: s.local,
			Confidence:    1.0,
		})
		return
	}

	// Decompose the local name into segments.
	segments := decomposeChain(s.local)
	if len(segments) == 0 {
		s.interps = append(s.interps, Interpretation{
			Package:       s.pkg,
			OuterFunction: s.local,
			Confidence:    1.0,
		})
		return
	}

	// Determine ABI suffix.
	abiSuffix := ""
	if s.source.hasABISuffix() {
		_, abiSuffix = stripABISuffix(s.raw)
	}

	// Separate trailing closure/wrapper segments from the function chain.
	chainSegs, closureSuffix, closureDepth, wrapper := splitClosureChain(segments)

	if len(chainSegs) == 0 {
		// Only closure segments — shouldn't happen with a valid symbol, but
		// handle gracefully.
		s.interps = append(s.interps, Interpretation{
			Package:       s.pkg,
			OuterFunction: s.local,
			ClosureSuffix: closureSuffix,
			ClosureDepth:  closureDepth,
			Wrapper:       wrapper,
			ABISuffix:     abiSuffix,
			Confidence:    1.0,
		})
		return
	}

	// Build interpretations from the chain segments.
	// The first segment determines whether we have a receiver ambiguity.
	first := chainSegs[0]

	switch first.kind {
	case segPtrRecv:
		// Unambiguous pointer-receiver method.
		interp := Interpretation{
			Package:               s.pkg,
			OuterReceiver:         first.receiver,
			OuterReceiverKind:     ReceiverPointer,
			OuterReceiverGenerics: first.recvGen,
			OuterFunction:         first.method,
			OuterFuncGenerics:     first.methGen,
			ClosureSuffix:         closureSuffix,
			ClosureDepth:          closureDepth,
			Wrapper:               wrapper,
			ABISuffix:             abiSuffix,
			Confidence:            1.0,
		}
		// Remaining chain segments are inlined calls.
		interp.InlinedCalls = buildInlinedCalls(chainSegs[1:])
		s.interps = append(s.interps, interp)

	case segName:
		if len(chainSegs) == 1 {
			// Single name segment — unambiguous function.
			s.interps = append(s.interps, Interpretation{
				Package:           s.pkg,
				OuterFunction:     first.name,
				OuterFuncGenerics: first.nameGen,
				ClosureSuffix:     closureSuffix,
				ClosureDepth:      closureDepth,
				Wrapper:           wrapper,
				ABISuffix:         abiSuffix,
				Confidence:        1.0,
			})
		} else {
			second := chainSegs[1]
			if second.kind == segPtrRecv {
				// First is a function name, second is an unambiguous ptr-recv
				// inlined method. No ambiguity about the first segment.
				interp := Interpretation{
					Package:           s.pkg,
					OuterFunction:     first.name,
					OuterFuncGenerics: first.nameGen,
					ClosureSuffix:     closureSuffix,
					ClosureDepth:      closureDepth,
					Wrapper:           wrapper,
					ABISuffix:         abiSuffix,
					Confidence:        1.0,
				}
				interp.InlinedCalls = buildInlinedCalls(chainSegs[1:])
				s.interps = append(s.interps, interp)
			} else {
				// Ambiguous: first could be a value receiver or a function
				// name. Generate both interpretations.

				// Interp 1: first is a function, rest are inlined.
				interp1 := Interpretation{
					Package:           s.pkg,
					OuterFunction:     first.name,
					OuterFuncGenerics: first.nameGen,
					ClosureSuffix:     closureSuffix,
					ClosureDepth:      closureDepth,
					Wrapper:           wrapper,
					ABISuffix:         abiSuffix,
				}
				interp1.InlinedCalls = buildInlinedCalls(chainSegs[1:])

				// Interp 2: first is a value receiver, second is the method.
				interp2 := Interpretation{
					Package:               s.pkg,
					OuterReceiver:         first.name,
					OuterReceiverKind:     ReceiverValue,
					OuterReceiverGenerics: first.nameGen,
					ClosureSuffix:         closureSuffix,
					ClosureDepth:          closureDepth,
					Wrapper:               wrapper,
					ABISuffix:             abiSuffix,
				}
				if second.kind == segName {
					interp2.OuterFunction = second.name
					interp2.OuterFuncGenerics = second.nameGen
					interp2.InlinedCalls = buildInlinedCalls(chainSegs[2:])
				} else if second.kind == segPtrRecv {
					// Shouldn't happen (handled above), but be safe.
					interp2.OuterFunction = second.method
					interp2.OuterFuncGenerics = second.methGen
					interp2.InlinedCalls = buildInlinedCalls(chainSegs[2:])
				}

				// Confidence scoring: lowercase first char → more likely a
				// function name, uppercase → more likely a type.
				r, _ := utf8.DecodeRuneInString(first.name)
				if r >= 'A' && r <= 'Z' {
					interp1.Confidence = 0.4
					interp2.Confidence = 0.6
				} else {
					interp1.Confidence = 0.6
					interp2.Confidence = 0.4
				}

				s.interps = append(s.interps, interp1, interp2)
			}
		}
	}
}

// decomposeChain breaks the local name into a sequence of segments by scanning
// left-to-right, handling brackets, pointer receivers, closures, and nesting.
func decomposeChain(local string) []segment {
	var segments []segment
	i := 0
	for i < len(local) {
		// Skip leading dots between segments.
		if local[i] == '.' {
			i++
			continue
		}

		// Case 1: Pointer receiver (*Type).Method
		if i < len(local)-1 && local[i] == '(' && local[i+1] == '*' {
			seg, end := parsePtrRecvSegment(local, i)
			if end > i {
				segments = append(segments, seg)
				i = end
				continue
			}
		}

		// Case 2: Check for -range, -fm suffixes.
		if local[i] == '-' {
			rest := local[i+1:]
			if strings.HasPrefix(rest, "range") {
				// Find end of rangeN
				end := i + 1 + 5 // past "range"
				for end < len(local) && local[end] >= '0' && local[end] <= '9' {
					end++
				}
				segments = append(segments, segment{
					kind: segRange,
					text: local[i+1 : end],
				})
				i = end
				continue
			}
			if strings.HasPrefix(rest, "fm") && (i+3 >= len(local) || local[i+3] == '.') {
				segments = append(segments, segment{
					kind: segFM,
					text: "fm",
				})
				i += 3
				continue
			}
		}

		// Case 3: Closure markers (funcN, gowrapN, deferwrapN).
		if seg, end, ok := tryParseClosure(local, i); ok {
			segments = append(segments, seg)
			i = end
			continue
		}

		// Case 4: Bare number (closure nesting).
		if local[i] >= '0' && local[i] <= '9' {
			end := i
			for end < len(local) && local[end] >= '0' && local[end] <= '9' {
				end++
			}
			segments = append(segments, segment{
				kind: segNesting,
				text: local[i:end],
			})
			i = end
			continue
		}

		// Case 5: Name segment — scan to next dot (outside brackets).
		name, gen, end := parseNameSegment(local, i)
		segments = append(segments, segment{
			kind:    segName,
			text:    local[i:end],
			name:    name,
			nameGen: gen,
		})
		i = end
	}
	return segments
}

// parsePtrRecvSegment parses a (*Type[Generics]).Method[Generics] segment.
// Returns the segment and the position after the segment.
func parsePtrRecvSegment(local string, start int) (segment, int) {
	// local[start] == '(', local[start+1] == '*'
	i := start + 2

	// Find the receiver name (and optional generics).
	recvStart := i
	recvName := ""
	var recvGen *GenericParams

	for i < len(local) {
		if local[i] == '[' {
			recvName = local[recvStart:i]
			bracketEnd := MatchBracket(local, i)
			if bracketEnd == -1 {
				return segment{}, start
			}
			recvGen = &GenericParams{
				Raw:   local[i+1 : bracketEnd],
				Start: i,
				End:   bracketEnd,
			}
			i = bracketEnd + 1
			break
		}
		if local[i] == ')' {
			recvName = local[recvStart:i]
			break
		}
		i++
	}

	// Expect ')' then '.'
	if i >= len(local) || local[i] != ')' {
		return segment{}, start
	}
	i++ // skip ')'
	if i >= len(local) || local[i] != '.' {
		return segment{}, start
	}
	i++ // skip '.'

	// Parse the method name (and optional generics).
	methStart := i
	methName := ""
	var methGen *GenericParams

	for i < len(local) {
		if local[i] == '[' {
			methName = local[methStart:i]
			bracketEnd := MatchBracket(local, i)
			if bracketEnd == -1 {
				return segment{}, start
			}
			methGen = &GenericParams{
				Raw:   local[i+1 : bracketEnd],
				Start: i,
				End:   bracketEnd,
			}
			i = bracketEnd + 1
			break
		}
		if local[i] == '.' || local[i] == '-' {
			methName = local[methStart:i]
			break
		}
		i++
	}
	if methName == "" {
		methName = local[methStart:i]
	}

	seg := segment{
		kind:     segPtrRecv,
		text:     local[start:i],
		receiver: recvName,
		recvGen:  recvGen,
		method:   methName,
		methGen:  methGen,
	}
	return seg, i
}

// tryParseClosure attempts to parse a closure marker (funcN, gowrapN,
// deferwrapN) at position start. Returns the segment, end position, and
// whether it matched.
func tryParseClosure(local string, start int) (segment, int, bool) {
	rest := local[start:]

	prefixes := []string{"func", "gowrap", "deferwrap"}
	for _, prefix := range prefixes {
		if !strings.HasPrefix(rest, prefix) {
			continue
		}
		end := start + len(prefix)
		// Must be followed by a digit, or for gowrap/deferwrap, end of string
		// or dot is also ok.
		if end < len(local) && local[end] >= '0' && local[end] <= '9' {
			for end < len(local) && local[end] >= '0' && local[end] <= '9' {
				end++
			}
			return segment{
				kind: segClosure,
				text: local[start:end],
			}, end, true
		}
		if (prefix == "gowrap" || prefix == "deferwrap") && (end >= len(local) || local[end] == '.') {
			return segment{
				kind: segClosure,
				text: local[start:end],
			}, end, true
		}
	}
	return segment{}, 0, false
}

// parseNameSegment parses an identifier segment, potentially with generic
// parameters. Returns the name, optional GenericParams, and end position.
func parseNameSegment(local string, start int) (string, *GenericParams, int) {
	i := start
	for i < len(local) {
		if local[i] == '[' {
			name := local[start:i]
			bracketEnd := MatchBracket(local, i)
			if bracketEnd == -1 {
				// Unmatched bracket — treat rest as the name.
				return local[start:], nil, len(local)
			}
			gen := &GenericParams{
				Raw:   local[i+1 : bracketEnd],
				Start: i,
				End:   bracketEnd,
			}
			return name, gen, bracketEnd + 1
		}
		if local[i] == '.' || local[i] == '-' {
			return local[start:i], nil, i
		}
		i++
	}
	return local[start:i], nil, i
}

// splitClosureChain separates trailing closure/nesting/wrapper segments from
// the function/method chain, handling interleaved closure and inlined
// segments.
func splitClosureChain(segments []segment) (chain []segment, closureSuffix string, closureDepth int, wrapper WrapperKind) {
	// Find the first closure/nesting/range/fm segment. Everything before it
	// that is a name or ptr-recv is part of the chain. After the first
	// closure, we split: closure/nesting goes to suffix, but ptr-recv and
	// name segments between closures go to the chain as inlined calls.

	firstClosure := -1
	for i, seg := range segments {
		if seg.kind == segClosure || seg.kind == segNesting || seg.kind == segRange || seg.kind == segFM {
			firstClosure = i
			break
		}
	}

	if firstClosure == -1 {
		// No closure segments at all.
		return segments, "", 0, WrapperNone
	}

	// Everything before the first closure is unambiguously the chain.
	chain = append(chain, segments[:firstClosure]...)

	// Walk the rest, collecting closure parts into the suffix and inlined
	// calls into the chain.
	var suffixParts []string
	for _, seg := range segments[firstClosure:] {
		switch seg.kind {
		case segClosure:
			suffixParts = append(suffixParts, seg.text)
			// gowrap and deferwrap are wrappers, not additional closure
			// nesting levels.
			if !strings.HasPrefix(seg.text, "gowrap") && !strings.HasPrefix(seg.text, "deferwrap") {
				closureDepth++
			}
		case segNesting:
			suffixParts = append(suffixParts, seg.text)
			closureDepth++
		case segRange:
			// Range attaches to the last closure suffix part with '-'.
			if len(suffixParts) > 0 {
				suffixParts[len(suffixParts)-1] += "-" + seg.text
			} else {
				suffixParts = append(suffixParts, seg.text)
			}
		case segFM:
			wrapper = WrapperMethodExpr
		case segPtrRecv, segName:
			// Inlined call interleaved with closures.
			chain = append(chain, seg)
		}
	}

	// Check for gowrap/deferwrap in closure parts for wrapper kind.
	for _, part := range suffixParts {
		if strings.HasPrefix(part, "gowrap") {
			wrapper = WrapperGoWrap
		} else if strings.HasPrefix(part, "deferwrap") {
			wrapper = WrapperDeferWrap
		}
	}

	closureSuffix = strings.Join(suffixParts, ".")
	return chain, closureSuffix, closureDepth, wrapper
}

// buildInlinedCalls converts a slice of chain segments into InlinedCall
// structs.
func buildInlinedCalls(segments []segment) []InlinedCall {
	if len(segments) == 0 {
		return nil
	}
	calls := make([]InlinedCall, 0, len(segments))
	for _, seg := range segments {
		switch seg.kind {
		case segPtrRecv:
			calls = append(calls, InlinedCall{
				Receiver:         seg.receiver,
				ReceiverKind:     ReceiverPointer,
				ReceiverGenerics: seg.recvGen,
				Function:         seg.method,
				FuncGenerics:     seg.methGen,
				Raw:              seg.text,
			})
		case segName:
			calls = append(calls, InlinedCall{
				Function:     seg.name,
				FuncGenerics: seg.nameGen,
				Raw:          seg.text,
			})
		}
	}
	return calls
}
