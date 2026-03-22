// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build test

// Package mock provides a mock for the networktracer component.
package mock

import (
	"testing"

	networktracer "github.com/DataDog/datadog-agent/comp/networktracer/def"
	"github.com/DataDog/datadog-agent/pkg/network"
)

// mockTracer is a controllable test implementation of the networktracer Component.
type mockTracer struct {
	activeConnections func(clientID string) (*network.Connections, func(), error)
	registerClient    func(clientID string) error
	getStats          func() map[string]interface{}
	pause             func() error
	resume            func() error
}

// Mock returns a mock networktracer component that returns zero values by default.
// Fields on the returned mock can be overridden for specific test scenarios.
func Mock(_ *testing.T) networktracer.Component {
	return &mockTracer{
		activeConnections: func(_ string) (*network.Connections, func(), error) {
			return &network.Connections{}, func() {}, nil
		},
		registerClient: func(_ string) error { return nil },
		getStats:       func() map[string]interface{} { return nil },
		pause:          func() error { return nil },
		resume:         func() error { return nil },
	}
}

// GetActiveConnections implements Component.
func (m *mockTracer) GetActiveConnections(clientID string) (*network.Connections, func(), error) {
	return m.activeConnections(clientID)
}

// RegisterClient implements Component.
func (m *mockTracer) RegisterClient(clientID string) error {
	return m.registerClient(clientID)
}

// GetStats implements Component.
func (m *mockTracer) GetStats() map[string]interface{} {
	return m.getStats()
}

// Pause implements Component.
func (m *mockTracer) Pause() error {
	return m.pause()
}

// Resume implements Component.
func (m *mockTracer) Resume() error {
	return m.resume()
}
