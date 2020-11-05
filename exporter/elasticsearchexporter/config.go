package elasticsearchexporter

import (
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration settings for the Elasticsearch exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"`
	ClientSettings                ClientConfig `mapstructure:",squash"`
	BulkSettings                  BulkConfig   `mapstructure:",squash"`
}

type ClientConfig struct {
	Hosts      []string                   `mapstructure:"hosts"`
	CloudID    string                     `mapstructure:"cloudid"`
	TLSSetting configtls.TLSClientSetting `mapstructure:",squash"`

	// ReadBufferSize for HTTP client. See http.Transport.ReadBufferSize.
	ReadBufferSize int `mapstructure:"read_buffer_size"`

	// WriteBufferSize for HTTP client. See http.Transport.WriteBufferSize.
	WriteBufferSize int `mapstructure:"write_buffer_size"`

	Authentication Authentication               `mapstructure:",squash"`
	Headers        map[string]string            `mapstructure:"headers,omitempty"`
	Retries        int                          `mapstructure:"retries"`
	Discovery      Discovery                    `mapstructure:"discover"`
	RetrySettings  exporterhelper.RetrySettings `mapstructure:"retry"`
}

type Authentication struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	APIKey   string `mapstructure:"api_key"`
}

type Discovery struct {
	OnStart  bool          `mapstructure:"on_start"`
	Interval time.Duration `mapstructure:"interval"`
}

type BulkConfig struct {
	Workers       int         `mapstructure:"workers"`
	FlushSettings FlushConfig `mapstructure:"flush"`

	Index    string        `mapstructure:"index"`
	Pipeline string        `mapstructure:"pipeline"`
	Routing  string        `mapstructure:"routing"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type FlushConfig struct {
	Bytes    int           `mapstructure:"bytes"`
	Interval time.Duration `mapstructure:"interval"`
}

type RetryConfig struct {
	InitialInterval time.Duration `mapstructure:"initial_interval"`
	MaxInterval     time.Duration `mapstructure:"max_interval"`
}
