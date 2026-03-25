// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package gosymname

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleFunction(t *testing.T) {
	s := Parse("encoding/json.Marshal", SourcePclntab)
	assert.Equal(t, "encoding/json", s.Package())
	assert.Equal(t, "Marshal", s.Local())
	assert.Equal(t, ClassFunction, s.Class())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "encoding/json", interps[0].Package)
	assert.Equal(t, "Marshal", interps[0].OuterFunction)
	assert.Equal(t, ReceiverNone, interps[0].OuterReceiverKind)
	assert.Equal(t, float32(1.0), interps[0].Confidence)
	assert.False(t, interps[0].IsMethod())
	assert.False(t, interps[0].HasInlinedCalls())
}

func TestPointerReceiverMethod(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/kv/kvserver.(*raftSchedulerShard).worker", SourceDWARF)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/kv/kvserver", s.Package())
	assert.Equal(t, "(*raftSchedulerShard).worker", s.Local())
	assert.Equal(t, ClassFunction, s.Class())
	assert.True(t, s.HasPointerReceiver())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "raftSchedulerShard", interps[0].OuterReceiver)
	assert.Equal(t, ReceiverPointer, interps[0].OuterReceiverKind)
	assert.Equal(t, "worker", interps[0].OuterFunction)
	assert.Equal(t, float32(1.0), interps[0].Confidence)
	assert.True(t, interps[0].IsMethod())
}

func TestValueReceiverMethodAmbiguous(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/kvfeed.rangefeedFactory.Run", SourceDWARF)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/kvfeed", s.Package())
	assert.Equal(t, "rangefeedFactory.Run", s.Local())
	assert.Equal(t, ClassFunction, s.Class())
	assert.True(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 2)

	// Find the function interpretation (higher confidence because lowercase first segment).
	var funcInterp, methInterp *Interpretation
	for i := range interps {
		if interps[i].OuterReceiverKind == ReceiverNone {
			funcInterp = &interps[i]
		} else {
			methInterp = &interps[i]
		}
	}
	require.NotNil(t, funcInterp)
	require.NotNil(t, methInterp)

	assert.Equal(t, "rangefeedFactory", funcInterp.OuterFunction)
	require.Len(t, funcInterp.InlinedCalls, 1)
	assert.Equal(t, "Run", funcInterp.InlinedCalls[0].Function)
	assert.Equal(t, float32(0.6), funcInterp.Confidence)

	assert.Equal(t, "rangefeedFactory", methInterp.OuterReceiver)
	assert.Equal(t, ReceiverValue, methInterp.OuterReceiverKind)
	assert.Equal(t, "Run", methInterp.OuterFunction)
	assert.Equal(t, float32(0.4), methInterp.Confidence)
}

func TestSimpleClosure(t *testing.T) {
	s := Parse("github.com/getsentry/sentry-go.NewClient.func1", SourceDWARF)
	assert.Equal(t, "github.com/getsentry/sentry-go", s.Package())
	assert.Equal(t, "NewClient.func1", s.Local())
	assert.Equal(t, ClassClosure, s.Class())
	assert.True(t, s.IsClosure())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "NewClient", interps[0].OuterFunction)
	assert.Equal(t, "func1", interps[0].ClosureSuffix)
	assert.Equal(t, 1, interps[0].ClosureDepth)
	assert.Equal(t, float32(1.0), interps[0].Confidence)
}

func TestEscapedImportPath(t *testing.T) {
	s := Parse("gopkg.in/square/go-jose%2ev2.newBuffer", SourceDWARF)
	assert.Equal(t, "gopkg.in/square/go-jose.v2", s.Package())
	assert.Equal(t, "newBuffer", s.Local())
	assert.Equal(t, ClassFunction, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "gopkg.in/square/go-jose.v2", interps[0].Package)
	assert.Equal(t, "newBuffer", interps[0].OuterFunction)
	assert.Equal(t, float32(1.0), interps[0].Confidence)
}

func TestInitFunction(t *testing.T) {
	s := Parse("github.com/klauspost/compress/flate.init.0", SourceDWARF)
	assert.Equal(t, "github.com/klauspost/compress/flate", s.Package())
	assert.Equal(t, "init.0", s.Local())
	assert.Equal(t, ClassInit, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "init.0", interps[0].OuterFunction)
	assert.Equal(t, float32(1.0), interps[0].Confidence)
}

func TestMapInitFunction(t *testing.T) {
	s := Parse("time.map.init.0", SourcePclntab)
	assert.Equal(t, "time", s.Package())
	assert.Equal(t, "map.init.0", s.Local())
	assert.Equal(t, ClassMapInit, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "map.init.0", interps[0].OuterFunction)
}

func TestGlobalClosure(t *testing.T) {
	s := Parse("glob.func1", SourcePclntab)
	assert.Equal(t, "glob", s.Package())
	assert.Equal(t, "func1", s.Local())
	assert.Equal(t, ClassGlobalClosure, s.Class())
	assert.True(t, s.IsClosure())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "glob", interps[0].Package)
	assert.Equal(t, "func1", interps[0].OuterFunction)
}

func TestMethodExpressionWrapper(t *testing.T) {
	s := Parse("encoding/json.arrayEncoder.encode-fm", SourcePclntab)
	assert.Equal(t, "encoding/json", s.Package())
	assert.Equal(t, "arrayEncoder.encode-fm", s.Local())
	assert.Equal(t, ClassFunction, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 2)

	for _, interp := range interps {
		assert.Equal(t, WrapperMethodExpr, interp.Wrapper)
	}

	// Find the value-receiver interpretation.
	var methInterp *Interpretation
	for i := range interps {
		if interps[i].OuterReceiverKind == ReceiverValue {
			methInterp = &interps[i]
		}
	}
	require.NotNil(t, methInterp)
	assert.Equal(t, "arrayEncoder", methInterp.OuterReceiver)
	assert.Equal(t, "encode", methInterp.OuterFunction)
}

func TestBareName(t *testing.T) {
	s := Parse("indexbytebody", SourceNM)
	assert.Equal(t, "", s.Package())
	assert.Equal(t, "indexbytebody", s.Local())
	assert.Equal(t, ClassBareName, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "indexbytebody", interps[0].OuterFunction)
	assert.Equal(t, float32(1.0), interps[0].Confidence)
}

func TestCFunctionSymbol(t *testing.T) {
	s := Parse("ZSTD_DCtx_trace_end.part.13", SourceNM)
	assert.Equal(t, "", s.Package())
	assert.Equal(t, "ZSTD_DCtx_trace_end.part.13", s.Local())
	assert.Equal(t, ClassCFunction, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "ZSTD_DCtx_trace_end.part.13", interps[0].OuterFunction)
}

func TestDeepInliningChainWithClosure(t *testing.T) {
	s := Parse("crypto/tls.keysFromMasterSecret.prfForVersion.prfAndHashForVersion.prf12.func2", SourcePclntab)
	assert.Equal(t, "crypto/tls", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.True(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 2)

	// Find the function interpretation (all inlined).
	var funcInterp *Interpretation
	for i := range interps {
		if interps[i].OuterReceiverKind == ReceiverNone {
			funcInterp = &interps[i]
			break
		}
	}
	require.NotNil(t, funcInterp)
	assert.Equal(t, "keysFromMasterSecret", funcInterp.OuterFunction)
	assert.Equal(t, "func2", funcInterp.ClosureSuffix)
	assert.Equal(t, 1, funcInterp.ClosureDepth)
	require.Len(t, funcInterp.InlinedCalls, 3)
	assert.Equal(t, "prfForVersion", funcInterp.InlinedCalls[0].Function)
	assert.Equal(t, "prfAndHashForVersion", funcInterp.InlinedCalls[1].Function)
	assert.Equal(t, "prf12", funcInterp.InlinedCalls[2].Function)
	assert.Equal(t, float32(0.6), funcInterp.Confidence)
}

func TestClosureNestingWithMidChainPtrReceiver(t *testing.T) {
	s := Parse("crypto/x509.marshalCertificatePolicies.func1.2.(*Builder).AddASN1ObjectIdentifier.1", SourcePclntab)
	assert.Equal(t, "crypto/x509", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "marshalCertificatePolicies", interp.OuterFunction)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "Builder", interp.InlinedCalls[0].Receiver)
	assert.Equal(t, ReceiverPointer, interp.InlinedCalls[0].ReceiverKind)
	assert.Equal(t, "AddASN1ObjectIdentifier", interp.InlinedCalls[0].Function)
	assert.Equal(t, "func1.2.1", interp.ClosureSuffix)
	assert.Equal(t, 3, interp.ClosureDepth)
	assert.Equal(t, float32(1.0), interp.Confidence)
}

func TestRepeatedPtrReceiverWithClosuresAndDeferwrap(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl.(*AggMetrics).getOrCreateScope.(*AggMetrics).getOrCreateScope.func1.func4.deferwrap", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "AggMetrics", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "getOrCreateScope", interp.OuterFunction)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "AggMetrics", interp.InlinedCalls[0].Receiver)
	assert.Equal(t, ReceiverPointer, interp.InlinedCalls[0].ReceiverKind)
	assert.Equal(t, "getOrCreateScope", interp.InlinedCalls[0].Function)
	assert.Equal(t, "func1.func4.deferwrap", interp.ClosureSuffix)
	assert.Equal(t, 2, interp.ClosureDepth)
	assert.Equal(t, WrapperDeferWrap, interp.Wrapper)
}

func TestPtrReceiverChainWithRangeIterator(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/resolvedspan.(*AggregatorFrontier).All.(*resolvedSpanFrontier).All.func1-range1", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/resolvedspan", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "AggregatorFrontier", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "All", interp.OuterFunction)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "resolvedSpanFrontier", interp.InlinedCalls[0].Receiver)
	assert.Equal(t, ReceiverPointer, interp.InlinedCalls[0].ReceiverKind)
	assert.Equal(t, "All", interp.InlinedCalls[0].Function)
	assert.Equal(t, "func1-range1", interp.ClosureSuffix)
	assert.Equal(t, 1, interp.ClosureDepth)
}

func TestFunctionWithInlinedPtrReceiverMethodAndClosure(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl.reconcileJobStateWithLocalState.(*cachedState).AggregatorFrontierSpans.func3", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "reconcileJobStateWithLocalState", interp.OuterFunction)
	assert.Equal(t, ReceiverNone, interp.OuterReceiverKind)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "cachedState", interp.InlinedCalls[0].Receiver)
	assert.Equal(t, ReceiverPointer, interp.InlinedCalls[0].ReceiverKind)
	assert.Equal(t, "AggregatorFrontierSpans", interp.InlinedCalls[0].Function)
	assert.Equal(t, "func3", interp.ClosureSuffix)
	assert.Equal(t, 1, interp.ClosureDepth)
	assert.Equal(t, float32(1.0), interp.Confidence)
}

func TestGenericMethodWithComplexTypeParams(t *testing.T) {
	input := `github.com/cockroachdb/cockroach/pkg/jobs/jobspb.(*TimestampSpansMap).MinTimestamp.Keys[go.shape.struct { WallTime int64 "protobuf:\"varint,1,opt,name=wall_time,json=wallTime,proto3\" json:\"wall_time,omitempty\""; Logical int32 "protobuf:\"varint,2,opt,name=logical,proto3\" json:\"logical,omitempty\"" },go.shape.[]github.com/cockroachdb/cockroach/pkg/roachpb.Span].func2-range1`
	s := Parse(input, SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/jobs/jobspb", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.True(t, s.IsGeneric())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "TimestampSpansMap", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "MinTimestamp", interp.OuterFunction)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "Keys", interp.InlinedCalls[0].Function)
	require.NotNil(t, interp.InlinedCalls[0].FuncGenerics)
	assert.Contains(t, interp.InlinedCalls[0].FuncGenerics.Raw, "go.shape.struct")
	assert.Contains(t, interp.InlinedCalls[0].FuncGenerics.Raw, "roachpb.Span")
	assert.Equal(t, "func2-range1", interp.ClosureSuffix)
	assert.Equal(t, 1, interp.ClosureDepth)
}

func TestGenericPtrReceiverInlinedMethod(t *testing.T) {
	input := "github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord.(*rangeFeedRegistry).ForEachPartialRangefeed.(*Set[go.shape.*github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord.activeRangeFeed]).Range.func3"
	s := Parse(input, SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.True(t, s.IsGeneric())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "rangeFeedRegistry", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "ForEachPartialRangefeed", interp.OuterFunction)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "Set", interp.InlinedCalls[0].Receiver)
	assert.Equal(t, ReceiverPointer, interp.InlinedCalls[0].ReceiverKind)
	require.NotNil(t, interp.InlinedCalls[0].ReceiverGenerics)
	assert.Contains(t, interp.InlinedCalls[0].ReceiverGenerics.Raw, "activeRangeFeed")
	assert.Equal(t, "Range", interp.InlinedCalls[0].Function)
	assert.Equal(t, "func3", interp.ClosureSuffix)
	assert.Equal(t, 1, interp.ClosureDepth)
}

func TestInlinedPtrReceiverMethodWithClosure(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvstreamer.(*workerCoordinator).getMaxNumRequestsToIssue.(*IntPool).ApproximateQuota.func2", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvstreamer", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.False(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "workerCoordinator", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "getMaxNumRequestsToIssue", interp.OuterFunction)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "IntPool", interp.InlinedCalls[0].Receiver)
	assert.Equal(t, ReceiverPointer, interp.InlinedCalls[0].ReceiverKind)
	assert.Equal(t, "ApproximateQuota", interp.InlinedCalls[0].Function)
	assert.Equal(t, "func2", interp.ClosureSuffix)
	assert.Equal(t, 1, interp.ClosureDepth)
	assert.Equal(t, float32(1.0), interp.Confidence)
}

func TestDeepFunctionInliningChainWithClosure(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/kv/kvserver.newConsistencyQueue.makeRateLimitedTimeoutFunc.makeRateLimitedTimeoutFuncByPermittedSlowdown.func2", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/kv/kvserver", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.True(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 2)

	// Find function interpretation.
	var funcInterp *Interpretation
	for i := range interps {
		if interps[i].OuterReceiverKind == ReceiverNone {
			funcInterp = &interps[i]
			break
		}
	}
	require.NotNil(t, funcInterp)
	assert.Equal(t, "newConsistencyQueue", funcInterp.OuterFunction)
	require.Len(t, funcInterp.InlinedCalls, 2)
	assert.Equal(t, "makeRateLimitedTimeoutFunc", funcInterp.InlinedCalls[0].Function)
	assert.Equal(t, "makeRateLimitedTimeoutFuncByPermittedSlowdown", funcInterp.InlinedCalls[1].Function)
	assert.Equal(t, "func2", funcInterp.ClosureSuffix)
	assert.Equal(t, 1, funcInterp.ClosureDepth)
	assert.Equal(t, float32(0.6), funcInterp.Confidence)
}

func TestAmbiguousFirstSegmentWithInlinedPtrReceiver(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/kv/kvserver.raftLogQueue.MaybeAddAsync.(*baseQueue).MaybeAddAsync.func1", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/kv/kvserver", s.Package())
	assert.Equal(t, ClassClosure, s.Class())
	assert.True(t, s.IsAmbiguous())

	interps := s.Interpretations()
	require.Len(t, interps, 2)

	// Find function interpretation.
	var funcInterp, methInterp *Interpretation
	for i := range interps {
		if interps[i].OuterReceiverKind == ReceiverNone {
			funcInterp = &interps[i]
		} else {
			methInterp = &interps[i]
		}
	}
	require.NotNil(t, funcInterp)
	require.NotNil(t, methInterp)

	assert.Equal(t, "raftLogQueue", funcInterp.OuterFunction)
	require.Len(t, funcInterp.InlinedCalls, 2)
	assert.Equal(t, "MaybeAddAsync", funcInterp.InlinedCalls[0].Function)
	assert.Equal(t, "baseQueue", funcInterp.InlinedCalls[1].Receiver)
	assert.Equal(t, ReceiverPointer, funcInterp.InlinedCalls[1].ReceiverKind)
	assert.Equal(t, "MaybeAddAsync", funcInterp.InlinedCalls[1].Function)

	assert.Equal(t, "raftLogQueue", methInterp.OuterReceiver)
	assert.Equal(t, ReceiverValue, methInterp.OuterReceiverKind)
	assert.Equal(t, "MaybeAddAsync", methInterp.OuterFunction)
	require.Len(t, methInterp.InlinedCalls, 1)
	assert.Equal(t, "baseQueue", methInterp.InlinedCalls[0].Receiver)
}

func TestCompilerGeneratedItab(t *testing.T) {
	input := "go:itab.*cloud.google.com/go/iam/apiv1/iampb.TestIamPermissionsResponse,google.golang.org/protobuf/reflect/protoreflect.ProtoMessage"
	s := Parse(input, SourceNM)
	assert.Equal(t, "", s.Package())
	assert.Equal(t, ClassCompilerGenerated, s.Class())
	assert.True(t, s.IsCompilerGenerated())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, input, interps[0].OuterFunction)
}

func TestCompilerGeneratedStruct(t *testing.T) {
	input := "go:struct { github.com/AzureAD/microsoft-authentication-library-for-go/apps/public.AcquireInteractiveOption; github.com/AzureAD/microsoft-authentication-library-for-go/apps/internal/options.CallOption }.Do"
	s := Parse(input, SourceNM)
	assert.Equal(t, ClassCompilerGenerated, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, input, interps[0].OuterFunction)
}

func TestSplitPackage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		source SymbolSource
		pkg    string
		local  string
	}{
		{"simple", "encoding/json.Marshal", SourcePclntab, "encoding/json", "Marshal"},
		{"escaped", "gopkg.in/square/go-jose%2ev2.newBuffer", SourceDWARF, "gopkg.in/square/go-jose.v2", "newBuffer"},
		{"bare", "indexbytebody", SourceNM, "", "indexbytebody"},
		{"runtime", "runtime.gcMarkDone", SourcePclntab, "runtime", "gcMarkDone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, local := SplitPackage(tt.input, tt.source)
			assert.Equal(t, tt.pkg, pkg)
			assert.Equal(t, tt.local, local)
		})
	}
}

func TestInterpretationQualifiedName(t *testing.T) {
	tests := []struct {
		name   string
		interp Interpretation
		want   string
	}{
		{
			"simple function",
			Interpretation{Package: "encoding/json", OuterFunction: "Marshal"},
			"encoding/json.Marshal",
		},
		{
			"pointer receiver",
			Interpretation{
				Package:           "pkg",
				OuterReceiver:     "Type",
				OuterReceiverKind: ReceiverPointer,
				OuterFunction:     "Method",
			},
			"pkg.(*Type).Method",
		},
		{
			"value receiver",
			Interpretation{
				Package:           "pkg",
				OuterReceiver:     "Type",
				OuterReceiverKind: ReceiverValue,
				OuterFunction:     "Method",
			},
			"pkg.Type.Method",
		},
		{
			"generic receiver",
			Interpretation{
				Package:               "pkg",
				OuterReceiver:         "Set",
				OuterReceiverKind:     ReceiverPointer,
				OuterReceiverGenerics: &GenericParams{Raw: "K,V"},
				OuterFunction:         "Get",
			},
			"pkg.(*Set[K,V]).Get",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.interp.QualifiedName())
		})
	}
}

func TestInlinedCallQualifiedFunction(t *testing.T) {
	ic := InlinedCall{
		Package:      "pkg",
		Receiver:     "Builder",
		ReceiverKind: ReceiverPointer,
		Function:     "AddASN1",
	}
	assert.Equal(t, "pkg.(*Builder).AddASN1", ic.QualifiedFunction())
}

func TestDisambiguateWithKnownTypes(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/kvfeed.rangefeedFactory.Run", SourceDWARF)
	require.True(t, s.IsAmbiguous())

	// Disambiguate by saying rangefeedFactory is a known type.
	types := map[string]struct{}{"rangefeedFactory": {}}
	result := s.Disambiguate(WithKnownTypes(types))
	require.Len(t, result, 1)
	assert.Equal(t, ReceiverValue, result[0].OuterReceiverKind)
	assert.Equal(t, "rangefeedFactory", result[0].OuterReceiver)
}

func TestDisambiguateWithKnownPackageTypes(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/kvfeed.rangefeedFactory.Run", SourceDWARF)
	require.True(t, s.IsAmbiguous())

	types := map[string]struct{}{
		"github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/kvfeed.rangefeedFactory": {},
	}
	result := s.Disambiguate(WithKnownPackageTypes(types))
	require.Len(t, result, 1)
	assert.Equal(t, ReceiverValue, result[0].OuterReceiverKind)
}

func TestDisambiguateWithoutInlined(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/kvfeed.rangefeedFactory.Run", SourceDWARF)
	require.True(t, s.IsAmbiguous())

	result := s.Disambiguate(WithoutInlined())
	require.Len(t, result, 1)
	assert.Equal(t, ReceiverValue, result[0].OuterReceiverKind)
}

func TestInitPlain(t *testing.T) {
	s := Parse("internal/bytealg.init", SourceDWARF)
	assert.Equal(t, "internal/bytealg", s.Package())
	assert.Equal(t, "init", s.Local())
	assert.Equal(t, ClassInit, s.Class())
}

func TestDeeplyNestedFunction(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/server.(*apiV2Server).execSQL.func8.1.3.2", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/server", s.Package())
	assert.Equal(t, ClassClosure, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "apiV2Server", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "execSQL", interp.OuterFunction)
	assert.Equal(t, "func8.1.3.2", interp.ClosureSuffix)
	assert.Equal(t, 4, interp.ClosureDepth)
}

func TestDeeplyNestedDeferwrap(t *testing.T) {
	s := Parse("github.com/foo/logical.(*logicalReplicationWriterProcessor).flushBuffer.Group.GoCtx.func7.1.deferwrap1", SourcePclntab)
	assert.Equal(t, "github.com/foo/logical", s.Package())
	assert.Equal(t, ClassClosure, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "logicalReplicationWriterProcessor", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "flushBuffer", interp.OuterFunction)
	// Group and GoCtx are inlined calls.
	require.Len(t, interp.InlinedCalls, 2)
	assert.Equal(t, "Group", interp.InlinedCalls[0].Function)
	assert.Equal(t, "GoCtx", interp.InlinedCalls[1].Function)
	assert.Equal(t, WrapperDeferWrap, interp.Wrapper)
}

func TestFunkyMethodCalledByInlinedFunction(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/server.(*topLevelServer).startPersistingHLCUpperBound.func1.(*Node).SetHLCUpperBound.1", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/server", s.Package())
	assert.Equal(t, ClassClosure, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "topLevelServer", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "startPersistingHLCUpperBound", interp.OuterFunction)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "Node", interp.InlinedCalls[0].Receiver)
	assert.Equal(t, ReceiverPointer, interp.InlinedCalls[0].ReceiverKind)
	assert.Equal(t, "SetHLCUpperBound", interp.InlinedCalls[0].Function)
	assert.Equal(t, "func1.1", interp.ClosureSuffix)
	assert.Equal(t, 2, interp.ClosureDepth)
}

func TestRepeatedPtrReceiverInlinedMethod(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/crosscluster/producer.(*spanConfigEventStream).startStreamProcessor.(*spanConfigEventStream).startStreamProcessor.func1.func6", SourcePclntab)
	assert.Equal(t, "github.com/cockroachdb/cockroach/pkg/crosscluster/producer", s.Package())
	assert.Equal(t, ClassClosure, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 1)

	interp := interps[0]
	assert.Equal(t, "spanConfigEventStream", interp.OuterReceiver)
	assert.Equal(t, ReceiverPointer, interp.OuterReceiverKind)
	assert.Equal(t, "startStreamProcessor", interp.OuterFunction)
	require.Len(t, interp.InlinedCalls, 1)
	assert.Equal(t, "spanConfigEventStream", interp.InlinedCalls[0].Receiver)
	assert.Equal(t, "startStreamProcessor", interp.InlinedCalls[0].Function)
	assert.Equal(t, "func1.func6", interp.ClosureSuffix)
	assert.Equal(t, 2, interp.ClosureDepth)
}

func TestRangeIteratorFunction(t *testing.T) {
	s := Parse("github.com/redis/rueidis/internal/cmds.HmsetFieldValue.FieldValueIter-range1", SourcePclntab)
	assert.Equal(t, "github.com/redis/rueidis/internal/cmds", s.Package())
	// This has -range1 so it's a closure.
	assert.Equal(t, ClassClosure, s.Class())

	interps := s.Interpretations()
	require.Len(t, interps, 2) // Ambiguous: HmsetFieldValue could be type or function.

	// Find the value-receiver interpretation.
	var methInterp *Interpretation
	for i := range interps {
		if interps[i].OuterReceiverKind == ReceiverValue {
			methInterp = &interps[i]
		}
	}
	require.NotNil(t, methInterp)
	assert.Equal(t, "HmsetFieldValue", methInterp.OuterReceiver)
	assert.Equal(t, "FieldValueIter", methInterp.OuterFunction)
}

func TestIsGeneric(t *testing.T) {
	s := Parse("pkg.(*bucket[go.shape.uint64,go.shape.*uint8]).add", SourceDWARF)
	assert.True(t, s.IsGeneric())
}

func TestHasCrossPackageInlining(t *testing.T) {
	// For pclntab/DWARF source, splitPkg scans the entire name because all
	// characters are valid import path chars. It finds the last '/' (in
	// "sem/tree") and splits there, producing package="...tree" and
	// local="walkStmt". This is a known limitation — the parser cannot
	// distinguish the outer package from the inlined package without external
	// context.
	//
	// HasCrossPackageInlining checks for '/' in the local part. Since splitPkg
	// over-scans, local has no '/', so this returns false for pclntab.
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/cdceval.NormalizedSelectClause.github.com/cockroachdb/cockroach/pkg/sql/sem/tree.walkStmt", SourcePclntab)
	assert.False(t, s.HasCrossPackageInlining())

	// But if we construct a local name that retains the '/' (e.g. a
	// manually-split symbol), we can detect it.
	s2 := Symbol{
		raw:   "test",
		local: "Foo.github.com/other/pkg.Bar",
	}
	assert.True(t, s2.HasCrossPackageInlining())
}

func TestCachingBehavior(t *testing.T) {
	s := Parse("encoding/json.Marshal", SourcePclntab)
	interps1 := s.Interpretations()
	interps2 := s.Interpretations()
	// Same slice (cached).
	assert.Equal(t, len(interps1), len(interps2))
	assert.Equal(t, interps1[0], interps2[0])
}

func TestBest(t *testing.T) {
	s := Parse("github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/kvfeed.rangefeedFactory.Run", SourceDWARF)
	best := s.Best()
	require.NotNil(t, best)
	// Best should be the function interpretation (higher confidence for lowercase).
	assert.Equal(t, "rangefeedFactory", best.OuterFunction)
	assert.Equal(t, ReceiverNone, best.OuterReceiverKind)
}

func TestParseInto(t *testing.T) {
	var s Symbol
	ParseInto(&s, "encoding/json.Marshal", SourcePclntab)
	assert.Equal(t, "encoding/json", s.Package())
	assert.Equal(t, "Marshal", s.Local())
}

func TestABISuffix(t *testing.T) {
	s := Parse("runtime.goexit.abi0", SourceNM)
	assert.Equal(t, "runtime", s.Package())
	assert.Equal(t, "goexit", s.Local())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "abi0", interps[0].ABISuffix)
}

func TestABIInternalSuffix(t *testing.T) {
	s := Parse("runtime.morestack.abiinternal", SourceNM)
	assert.Equal(t, "runtime", s.Package())
	assert.Equal(t, "morestack", s.Local())

	interps := s.Interpretations()
	require.Len(t, interps, 1)
	assert.Equal(t, "abiinternal", interps[0].ABISuffix)
}
