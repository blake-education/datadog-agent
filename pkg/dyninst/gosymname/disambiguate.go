// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package gosymname

// WithKnownTypes returns a disambiguator that keeps interpretations where the
// outer receiver type is in the given set of unqualified type names. Non-method
// interpretations are filtered out.
func WithKnownTypes(types map[string]struct{}) func(*Interpretation) bool {
	return func(i *Interpretation) bool {
		if i.OuterReceiverKind == ReceiverNone {
			return false
		}
		_, ok := types[i.OuterReceiver]
		return ok
	}
}

// WithKnownPackageTypes returns a disambiguator that keeps interpretations
// where the fully-qualified receiver type (package + "." + type) is in the
// given set. Non-method interpretations are filtered out.
func WithKnownPackageTypes(qualifiedTypes map[string]struct{}) func(*Interpretation) bool {
	return func(i *Interpretation) bool {
		if i.OuterReceiverKind == ReceiverNone {
			return false
		}
		qualified := i.Package + "." + i.OuterReceiver
		_, ok := qualifiedTypes[qualified]
		return ok
	}
}

// WithoutInlined returns a disambiguator that keeps only interpretations
// without inlined calls.
func WithoutInlined() func(*Interpretation) bool {
	return func(i *Interpretation) bool {
		return !i.HasInlinedCalls()
	}
}
