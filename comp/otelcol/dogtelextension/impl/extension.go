// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package dogtelextensionimpl

import (
	"context"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"google.golang.org/grpc"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface"
	ipc "github.com/DataDog/datadog-agent/comp/core/ipc/def"
	log "github.com/DataDog/datadog-agent/comp/core/log/def"
	secrets "github.com/DataDog/datadog-agent/comp/core/secrets/def"
	tagger "github.com/DataDog/datadog-agent/comp/core/tagger/def"
	"github.com/DataDog/datadog-agent/comp/core/telemetry"
	workloadmeta "github.com/DataDog/datadog-agent/comp/core/workloadmeta/def"
	"github.com/DataDog/datadog-agent/comp/metadata/runner"
	dogtelmetrics "github.com/DataDog/datadog-agent/comp/otelcol/dogtelextension/impl/metrics"
	agentmetrics "github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-agent/pkg/serializer"
)

// dogtelExtension implements the dogtelextension.Component interface
type dogtelExtension struct {
	config     *Config
	log        log.Component
	coreConfig coreconfig.Component

	// Core components injected from FX
	serializer   serializer.MetricSerializer
	hostname     hostnameinterface.Component
	workloadmeta workloadmeta.Component
	tagger       tagger.Component
	ipc          ipc.Component
	telemetry    telemetry.Component
	secrets      secrets.Component

	// Build info for metric tags
	buildInfo component.BuildInfo

	// Metadata components (created by extension)
	metadataRunner runner.Component

	// Tagger gRPC server
	taggerServer     *grpc.Server
	taggerServerPort int
	taggerListener   net.Listener
}

// Start implements extension.Extension
func (e *dogtelExtension) Start(_ context.Context, _ component.Host) error {
	// Check if running in standalone mode
	standalone := e.coreConfig.GetBool("otel_standalone")
	if !standalone {
		e.log.Warn("dogtelextension is enabled but DD_OTEL_STANDALONE is false")
		e.log.Warn("This extension should only be used in standalone mode (DD_OTEL_STANDALONE=true)")
		e.log.Warn("In connected mode, the core Datadog Agent provides these functionalities")
		e.log.Info("dogtelextension disabled (not in standalone mode)")
		return nil
	}

	e.log.Info("Starting dogtelextension in standalone mode")

	// Start tagger gRPC server if enabled
	if e.config.EnableTaggerServer {
		if err := e.startTaggerServer(); err != nil {
			e.log.Errorf("Failed to start tagger server: %v", err)
			return err
		}
	}

	// Start metadata collection if enabled
	if e.config.EnableMetadataCollection && e.metadataRunner != nil {
		e.log.Info("Metadata collection is enabled")
	}

	e.log.Infof("dogtelextension started successfully (tagger_port=%d, metadata_enabled=%t)",
		e.taggerServerPort, e.config.EnableMetadataCollection)

	// Send liveness metric to indicate the extension is running
	if err := e.sendLivenessMetric(context.Background()); err != nil {
		e.log.Warnf("Failed to send liveness metric: %v", err)
	}

	return nil
}

// sendLivenessMetric sends a gauge metric indicating the extension is running.
func (e *dogtelExtension) sendLivenessMetric(ctx context.Context) error {
	hostname := e.hostname.GetSafe(ctx)
	now := pcommon.NewTimestampFromTime(time.Now())
	buildTags := dogtelmetrics.TagsFromBuildInfo(e.buildInfo)
	serie := dogtelmetrics.CreateLivenessSerie(hostname, uint64(now), buildTags)

	var serieErr error
	agentmetrics.Serialize(
		agentmetrics.NewIterableSeries(func(_ *agentmetrics.Serie) {}, 200, 4000),
		agentmetrics.NewIterableSketches(func(_ *agentmetrics.SketchSeries) {}, 200, 4000),
		func(seriesSink agentmetrics.SerieSink, _ agentmetrics.SketchesSink) {
			seriesSink.Append(serie)
		},
		func(serieSource agentmetrics.SerieSource) {
			serieErr = e.serializer.SendIterableSeries(serieSource)
		},
		func(_ agentmetrics.SketchesSource) {},
	)
	return serieErr
}

// Shutdown implements extension.Extension
func (e *dogtelExtension) Shutdown(_ context.Context) error {
	e.log.Info("Shutting down dogtelextension")

	// Stop tagger server gracefully
	e.stopTaggerServer()

	// Stop metadata collection
	if e.metadataRunner != nil {
		// Metadata runner stops via its own lifecycle hooks
		e.log.Debug("Metadata runner will stop via FX lifecycle")
	}

	e.log.Info("dogtelextension shutdown complete")
	return nil
}

// GetTaggerServerPort implements dogtelextension.Component
func (e *dogtelExtension) GetTaggerServerPort() int {
	return e.taggerServerPort
}
