// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build linux

// Package ptracer holds ptracer related files
package ptracer

import (
	"encoding/binary"

	"github.com/DataDog/datadog-agent/pkg/security/proto/ebpfless"
	"github.com/DataDog/datadog-agent/pkg/security/secl/model/utils"
)

// SpanTLS holds the needed informations to retrieve spans on a TLS
type SpanTLS struct {
	format     uint64 // present but not used
	maxThreads uint64
	base       uintptr
}

func isTLSRegisterRequest(req []byte) bool {
	return req[0] == RegisterSpanTLSOp
}

func handleTLSRegister(req []byte) *SpanTLS {
	return &SpanTLS{
		format:     binary.NativeEndian.Uint64(req[1:9]),
		maxThreads: binary.NativeEndian.Uint64(req[9:17]),
		base:       uintptr(binary.NativeEndian.Uint64(req[17:25])),
	}
}

func fillSpanContext(tracer *Tracer, pid int, tid int, span *SpanTLS) *ebpfless.SpanContext {
	if span == nil {
		return nil
	}
	offset := uint64((tid % int(span.maxThreads)) * (16 + 8))

	pSpan, err := tracer.readData(pid, uint64(span.base)+offset, 24 /*sizeof uint128 + sizeof uint64*/)
	if err != nil {
		return nil
	}

	return &ebpfless.SpanContext{
		SpanID: binary.NativeEndian.Uint64(pSpan[0:8]),
		TraceID: utils.TraceID{
			Lo: binary.NativeEndian.Uint64(pSpan[8:16]),
			Hi: binary.NativeEndian.Uint64(pSpan[16:24]),
		},
	}
}

// OTelTLS holds information needed to read OTel Thread Local Context Records
// from native applications using ELF TLSDESC.
// Support for additional runtimes (e.g., Go via pprof labels) will be added later.
type OTelTLS struct {
	tlsOffset int64 // signed TLS offset from thread pointer (discovered via dynsym)
}

func isOTelTLSRegisterRequest(req []byte) bool {
	return req[0] == RegisterOTelTLSOp
}

func handleOTelTLSRegister(req []byte) *OTelTLS {
	if len(req) < 9 {
		return nil
	}
	return &OTelTLS{
		tlsOffset: int64(binary.NativeEndian.Uint64(req[1:9])),
	}
}
