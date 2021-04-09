// Copyright Sabre

package prometheusremotewritereceiver

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config - remote write
type Config struct {
	config.ReceiverSettings       `mapstructure:",squash"`
	confighttp.HTTPServerSettings `mapstructure:",squash"`
}

var _ config.Receiver = (*Config)(nil)
