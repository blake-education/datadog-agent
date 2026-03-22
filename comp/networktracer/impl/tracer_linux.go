// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build linux_bpf

// Package networktracerImpl implements the networktracer component.
package networktracerImpl

import (
	"context"
	"fmt"

	ddgostatsd "github.com/DataDog/datadog-go/v5/statsd"

	"github.com/DataDog/datadog-agent/comp/core/telemetry"
	compdef "github.com/DataDog/datadog-agent/comp/def"
	networktracer "github.com/DataDog/datadog-agent/comp/networktracer/def"
	"github.com/DataDog/datadog-agent/pkg/network"
	networkconfig "github.com/DataDog/datadog-agent/pkg/network/config"
	"github.com/DataDog/datadog-agent/pkg/network/tracer"
)

// Requires defines the fx dependencies for the networktracer component.
type Requires struct {
	Lifecycle compdef.Lifecycle

	Telemetry telemetry.Component
	Statsd    ddgostatsd.ClientInterface
}

// Provides defines the output of the networktracer component.
type Provides struct {
	Comp networktracer.Component
}

// tracerImpl is the private implementation of the networktracer component.
type tracerImpl struct {
	t *tracer.Tracer
}

// NewComponent creates a new networktracer component backed by the eBPF tracer.
func NewComponent(reqs Requires) (Provides, error) {
	ncfg := networkconfig.New()

	t, err := tracer.NewTracer(ncfg, reqs.Telemetry, reqs.Statsd)
	if err != nil {
		return Provides{}, fmt.Errorf("could not create network tracer: %w", err)
	}

	impl := &tracerImpl{t: t}

	reqs.Lifecycle.Append(compdef.Hook{
		OnStart: func(context.Context) error { return nil },
		OnStop: func(context.Context) error {
			impl.t.Stop()
			return nil
		},
	})

	return Provides{Comp: impl}, nil
}

// GetActiveConnections returns active network connections for the given clientID.
func (ti *tracerImpl) GetActiveConnections(clientID string) (*network.Connections, func(), error) {
	return ti.t.GetActiveConnections(clientID)
}

// RegisterClient registers a new client for connection delta tracking.
func (ti *tracerImpl) RegisterClient(clientID string) error {
	return ti.t.RegisterClient(clientID)
}

// GetStats returns internal tracer statistics.
func (ti *tracerImpl) GetStats() map[string]interface{} {
	stats, _ := ti.t.GetStats()
	return stats
}

// Pause suspends eBPF-based connection tracking.
func (ti *tracerImpl) Pause() error {
	return ti.t.Pause()
}

// Resume resumes eBPF-based connection tracking.
func (ti *tracerImpl) Resume() error {
	return ti.t.Resume()
}
