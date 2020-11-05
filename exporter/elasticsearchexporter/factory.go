package elasticsearchexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr = "elasticsearch"
)

// NewFactory creates an Elasticsearch exporter factory
func NewFactory() component.ExporterFactory {
	f := &elasticsearchExporterFactory{}

	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithLogs(f.createLogsExporter))
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		ClientSettings: ClientConfig{
			Retries: 3,
			RetrySettings: exporterhelper.RetrySettings{
				InitialInterval: 1 * time.Second,
				MaxInterval:     60 * time.Second,
				MaxElapsedTime:  5 * time.Minute,
			},
		},
		BulkSettings: BulkConfig{
			Timeout: 60 * time.Second,
			FlushSettings: FlushConfig{
				Interval: 1 * time.Second,
			},
		},
	}
}

type elasticsearchExporterFactory struct{}

func (f *elasticsearchExporterFactory) createLogsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.LogsExporter, error) {
	oCfg := cfg.(*Config)

	exporter, err := newExporter(*oCfg, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		cfg,
		params.Logger,
		exporter.pushLogsData,
		exporterhelper.WithShutdown(exporter.Shutdown),
	)
}
