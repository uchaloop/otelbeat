// Package otelbeatfx wires otelbeat into an Uber Fx application: it provides a
// beat.Handler that records OpenTelemetry metrics, built from the container's
// metric.MeterProvider. The otelbeat core has no Fx dependency; this package is
// the Fx integration, mirroring beat/beatfx.
package otelbeatfx

import (
	"github.com/uchaloop/beat"
	"github.com/uchaloop/otelbeat"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"
)

// Module provides a beat.Handler that records otelbeat's metrics on the "beat"
// meter from the container's metric.MeterProvider. opts are forwarded to
// otelbeat.New.
func Module(opts ...otelbeat.Option) fx.Option {
	return fx.Module(
		"otelbeat",

		fx.Provide(
			func(mp metric.MeterProvider) (beat.Handler, error) {
				return otelbeat.New(mp.Meter("beat"), opts...)
			},
		),
	)
}
