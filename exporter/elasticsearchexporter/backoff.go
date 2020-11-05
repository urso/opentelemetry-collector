package elasticsearchexporter

import (
	"time"

	"github.com/cenkalti/backoff"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type backoffFunc func(attempts int) time.Duration

func createBackoffFunc(config exporterhelper.RetrySettings) backoffFunc {
	if !config.Enabled {
		return nil
	}

	expBackoff := backoff.NewExponentialBackOff()
	if config.InitialInterval > 0 {
		expBackoff.InitialInterval = config.InitialInterval
	}
	if config.MaxInterval > 0 {
		expBackoff.MaxInterval = config.MaxInterval
	}
	if config.MaxElapsedTime > 0 {
		expBackoff.MaxElapsedTime = config.MaxElapsedTime
	}
	expBackoff.Reset()

	return func(attempts int) time.Duration {
		if attempts == 1 {
			expBackoff.Reset()
		}

		return expBackoff.NextBackOff()
	}
}
