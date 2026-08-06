package otelbeatfx_test

import (
	"testing"

	"github.com/uchaloop/beat"
	"github.com/uchaloop/otelbeat/otelbeatfx"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
)

func TestModule_ProvidesHandler(t *testing.T) {
	var h beat.Handler

	app := fx.New(
		fx.Provide(func() metric.MeterProvider { return sdkmetric.NewMeterProvider() }),
		otelbeatfx.Module(),
		fx.Populate(&h),
		fx.NopLogger,
	)

	if err := app.Err(); err != nil {
		t.Fatalf("graph: %v", err)
	}
	if h == nil {
		t.Fatal("no beat.Handler provided")
	}
}
