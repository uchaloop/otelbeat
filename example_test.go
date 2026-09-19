package otelbeat_test

import (
	"context"
	"errors"
	"time"

	"github.com/uchaloop/beat"
	"github.com/uchaloop/otelbeat"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func ExampleNew() {
	// ManualReader permits explicit collection. A production application can
	// instead supply its configured reader/exporter to the MeterProvider.
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	handler, err := otelbeat.New(provider.Meter("beat"))
	if err != nil {
		panic(err)
	}
	runner, err := beat.MakeBeat(beat.Config{
		Period: 5 * time.Minute, JobTimeout: 4 * time.Minute,
		Jitter: 5 * time.Minute,
	}, func(ctx context.Context) (int, error) { return 0, ctx.Err() }, handler)
	if err != nil {
		panic(err)
	}
	if err := runner.Start(context.Background()); err != nil {
		panic(err)
	}
	// The application runs until its shutdown signal.
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := runner.Stop(stopCtx); err != nil {
		panic(err)
	}
	exportCtx, exportCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer exportCancel()
	if err := provider.Shutdown(exportCtx); err != nil {
		panic(err)
	}
	// Output:
}

func ExampleWithOutcome() {
	errThrottled := errors.New("throttled")
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer provider.Shutdown(context.Background())
	handler, err := otelbeat.New(provider.Meter("beat"),
		otelbeat.WithOutcome(func(r beat.Record) string {
			if r.Outcome == beat.OutcomeError && errors.Is(r.Err, errThrottled) {
				return "throttled"
			}
			return otelbeat.DefaultOutcome(r)
		}),
	)
	if err != nil {
		panic(err)
	}
	_ = handler // Supply this handler to beat.MakeBeat.
	// Output:
}
