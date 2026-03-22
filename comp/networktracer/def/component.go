// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package networktracer defines the component interface for network connection tracing.
package networktracer

import "github.com/DataDog/datadog-agent/pkg/network"

// team: network-monitoring

// Component provides network connection tracing capabilities.
// It wraps the underlying eBPF-based tracer on Linux and the driver-based tracer on Windows.
type Component interface {
	// GetActiveConnections returns active network connections for the given clientID.
	// The returned cleanup function MUST be called after the caller is done with the connections.
	GetActiveConnections(clientID string) (*network.Connections, func(), error)

	// RegisterClient registers a new client for connection delta tracking.
	RegisterClient(clientID string) error

	// GetStats returns internal tracer statistics.
	GetStats() map[string]interface{}

	// Pause suspends eBPF-based connection tracking.
	Pause() error

	// Resume resumes eBPF-based connection tracking.
	Resume() error
}
