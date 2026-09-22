package otelbeat_test

import (
	"context"
	"time"

	"github.com/uchaloop/beat"
	"github.com/uchaloop/job"
	"github.com/uchaloop/otelbeat"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func ExampleMakeHandler() {
	// The application supplies its reader/exporter. ManualReader needs explicit
	// collection and is useful in tests; it does not export over the network.
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewManualReader()))
	observer, err := otelbeat.MakeHandler(provider.Meter("example/worker"))
	if err != nil {
		panic(err)
	}
	runner, err := job.MakeRunner(job.Config{Timeout: 4 * time.Minute}, func(context.Context) (int64, error) { return 1000, nil })
	if err != nil {
		panic(err)
	}
	scheduler, err := beat.MakeBeat(beat.Config{Period: 5 * time.Minute, Jitter: 5 * time.Minute}, runner, beat.WithHandler(observer))
	if err != nil {
		panic(err)
	}
	if err := scheduler.Start(context.Background()); err != nil {
		panic(err)
	}
	// A real application waits for its shutdown signal here.
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStop()
	if err := scheduler.Stop(stopCtx); err != nil {
		panic(err)
	}
	flushCtx, cancelFlush := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFlush()
	if err := provider.Shutdown(flushCtx); err != nil {
		panic(err)
	}
	// Output:
}
