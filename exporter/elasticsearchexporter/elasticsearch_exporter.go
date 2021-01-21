package elasticsearchexporter

import (
	"bytes"
	"context"
	"net/http"

	"github.com/elastic/go-elasticsearch/v7"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esutil"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type elasticsearchExporter struct {
	maxRetries int
	index      string
	pipeline   string
	mapping    map[string]string

	logger *zap.Logger

	client      *elasticsearch7.Client
	bulkIndexer esutil.BulkIndexer
}

var retryOnStatus = []int{502, 503, 504, 429}

func newExporter(config Config, params component.ExporterCreateParams) (*elasticsearchExporter, error) {
	client, err := newClient(params.Logger, config.ClientSettings)
	if err != nil {
		return nil, err
	}

	bulkIndexer, err := newBulkIndexer(client, config.BulkSettings)
	if err != nil {
		return nil, err
	}

	return &elasticsearchExporter{
		logger:     params.Logger,
		maxRetries: config.ClientSettings.Retries,
		index:      config.BulkSettings.Index,
		// TODO: add setting to disable ECS auto-mapping
		mapping: ecsConventionsMapping,

		client:      client,
		bulkIndexer: bulkIndexer,
	}, nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	return e.bulkIndexer.Close(ctx)
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld pdata.Logs) (dropped int, err error) {
	e.logger.Debug("Received new log events.")

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		iils := rl.InstrumentationLibraryLogs()
		for j := 0; j < iils.Len(); j++ {
			ils := iils.At(i)
			logs := ils.Logs()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)

				var event []byte
				event, err := encodeLogEvent(lr, e.mapping)
				if err != nil {
					e.logger.Error("Failed to encode log record.", zap.NamedError("reason", err))
					dropped++
					continue
				}

				if err := e.enqueueEvent(ctx, 0, event); err != nil {
					// The bulkdIndexer only returns an error if the context was cancelled.
					return dropped, err
				}
			}
		}
	}

	return dropped, nil
}

func (e *elasticsearchExporter) enqueueEvent(ctx context.Context, attempts int, event []byte) error {
	item := esutil.BulkIndexerItem{
		Index:  e.index,
		Action: "create",
		Body:   bytes.NewReader(event),
		OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, resp esutil.BulkIndexerResponseItem, err error) {
			switch {
			case e.maxRetries < 0 || attempts < e.maxRetries && shouldRetryRequest(resp.Status):
				// TODO: debug log
				e.logger.Debug("Retrying to index event", zap.Int("attempt", attempts), zap.Int("status", resp.Status))
				e.enqueueEvent(ctx, attempts+1, event)
			case resp.Status == 0 && err != nil: // encoding error, we didn't even attempt to send the event
				e.logger.Error("Failed to add event to bulk request.", zap.NamedError("reason", err))
			default:
				e.logger.Error("Failed to index event.", zap.NamedError("reason", err))
			}
		},
	}

	return e.bulkIndexer.Add(ctx, item)
}

func newClient(logger *zap.Logger, config ClientConfig) (*elasticsearch7.Client, error) {
	transport, err := newTransport(config)
	if err != nil {
		return nil, err
	}

	var headers http.Header
	for k, v := range config.Headers {
		headers.Add(k, v)
	}

	// TODO: validate settings:
	//  - try to parse address and validate scheme (address must be a valid URL"
	//  - check if cloud ID is valid

	return elasticsearch.NewClient(elasticsearch7.Config{
		Addresses:             config.Hosts,
		CloudID:               config.CloudID,
		Username:              config.Authentication.User,
		Password:              config.Authentication.Password,
		APIKey:                config.Authentication.APIKey,
		Header:                headers,
		EnableRetryOnTimeout:  true,
		MaxRetries:            config.Retries,
		DiscoverNodesOnStart:  config.Discovery.OnStart,
		DiscoverNodesInterval: config.Discovery.Interval,
		RetryOnStatus:         retryOnStatus,
		RetryBackoff:          createBackoffFunc(config.RetrySettings),
		Logger:                (*clientLogger)(logger),
		Transport:             transport,
	})
}

func newTransport(config ClientConfig) (*http.Transport, error) {
	tlsCfg, err := config.TLSSetting.LoadTLSConfig()
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsCfg != nil {
		transport.TLSClientConfig = tlsCfg
	}
	if config.ReadBufferSize > 0 {
		transport.ReadBufferSize = config.ReadBufferSize
	}
	if config.WriteBufferSize > 0 {
		transport.WriteBufferSize = config.WriteBufferSize
	}

	return transport, nil
}

func newBulkIndexer(client *elasticsearch7.Client, config BulkConfig) (esutil.BulkIndexer, error) {
	// TODO: add debug logger
	return esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		NumWorkers:    config.Workers,
		FlushBytes:    config.FlushSettings.Bytes,
		FlushInterval: config.FlushSettings.Interval,
		Client:        client,
		Index:         config.Index,
		Pipeline:      config.Pipeline,
		Routing:       config.Routing,
		Timeout:       config.Timeout,
	})
}

func shouldRetryRequest(status int) bool {
	for _, retryable := range retryOnStatus {
		if status == retryable {
			return true
		}
	}
	return false
}
