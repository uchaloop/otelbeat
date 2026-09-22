package otelbeat_test

import (
	"context"
	"math"
	"testing"
	"testing/synctest"
	"time"

	"github.com/uchaloop/beat"
	"github.com/uchaloop/job"
	"github.com/uchaloop/otelbeat"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func collect(t *testing.T, records ...beat.Record) map[string]metricdata.Metrics {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	h, err := otelbeat.MakeHandler(provider.Meter("test"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		h.Handle(context.Background(), r)
	}
	return read(t, reader)
}

func read(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var resource metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &resource); err != nil {
		t.Fatal(err)
	}
	metrics := make(map[string]metricdata.Metrics)
	for _, scope := range resource.ScopeMetrics {
		for _, m := range scope.Metrics {
			metrics[m.Name] = m
		}
	}
	return metrics
}

func TestExecutionRecordedOnce(t *testing.T) {
	now := time.Now()
	metrics := collect(t, beat.Record{Result: job.Result{Start: now, Duration: time.Second, WorkDuration: time.Second, Processed: 1000, Outcome: job.OutcomeError}, ScheduledFor: now.Add(-250 * time.Millisecond), Mode: beat.ModeFixedRate, Missed: 2, Iteration: 42})
	if len(metrics) != 4 {
		t.Fatalf("instruments: %v", metrics)
	}
	for _, p := range metrics["job.run.duration"].Data.(metricdata.Histogram[float64]).DataPoints {
		if p.Count != 1 || p.Sum != 1 {
			t.Fatal(p)
		}
	}
	processed := metrics["job.run.processed"].Data.(metricdata.Histogram[int64]).DataPoints
	if len(processed) != 1 || processed[0].Count != 1 || processed[0].Sum != 1000 {
		t.Fatal(processed)
	}
	late := metrics["beat.run.lateness"].Data.(metricdata.Histogram[float64]).DataPoints
	if len(late) != 1 || late[0].Sum != .25 || late[0].Attributes.Len() != 1 {
		t.Fatal(late)
	}
	missed := metrics["beat.missed"].Data.(metricdata.Sum[int64])
	if !missed.IsMonotonic || len(missed.DataPoints) != 1 || missed.DataPoints[0].Value != 2 || missed.DataPoints[0].Attributes.Len() != 1 {
		t.Fatal(missed)
	}
}

func TestLatenessMissingAndClockRollback(t *testing.T) {
	now := time.Now()
	metrics := collect(t,
		beat.Record{ScheduledFor: now},
		beat.Record{Result: job.Result{Start: now}},
		beat.Record{Result: job.Result{Start: now}, ScheduledFor: now.Add(time.Second)},
	)
	points := metrics["beat.run.lateness"].Data.(metricdata.Histogram[float64]).DataPoints
	if len(points) != 1 || points[0].Count != 1 || points[0].Sum != 0 {
		t.Fatal(points)
	}
}

func TestMissedConversion(t *testing.T) {
	for _, n := range []uint64{0, 1, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64} {
		metrics := collect(t, beat.Record{Missed: n, Mode: beat.ModeFixedRate})
		points := metrics["beat.missed"].Data.(metricdata.Sum[int64]).DataPoints
		if len(points) != 1 || points[0].Value != int64(min(n, uint64(math.MaxInt64))) {
			t.Fatalf("%d: %+v", n, points)
		}
		if _, ok := metrics["job.run.duration"]; ok {
			t.Fatal("work never ran")
		}
	}
}

func TestModesAreBounded(t *testing.T) {
	metrics := collect(t, beat.Record{Mode: beat.ModeFixedRate}, beat.Record{Mode: beat.ModeFixedDelay}, beat.Record{Mode: "custom"}, beat.Record{})
	points := metrics["beat.missed"].Data.(metricdata.Sum[int64]).DataPoints
	if len(points) != 3 {
		t.Fatal(points)
	}
}

func TestBeatIntegration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		defer provider.Shutdown(context.Background())
		h, err := otelbeat.MakeHandler(provider.Meter("test"))
		if err != nil {
			t.Fatal(err)
		}
		runner, err := job.MakeRunner(job.Config{}, func(context.Context) (int64, error) { time.Sleep(1500 * time.Millisecond); return 1000, nil })
		if err != nil {
			t.Fatal(err)
		}
		completed := make(chan struct{}, 1)
		count := 0
		scheduler, err := beat.MakeBeat(beat.Config{Period: time.Second}, runner, beat.WithHandler(beat.MultiHandler(h, beat.HandlerFunc(func(context.Context, beat.Record) {
			count++
			if count == 2 {
				completed <- struct{}{}
			}
		}))))
		if err != nil {
			t.Fatal(err)
		}
		if err := scheduler.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		<-completed
		if err := scheduler.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		metrics := read(t, reader)
		for _, p := range metrics["job.run.duration"].Data.(metricdata.Histogram[float64]).DataPoints {
			if p.Count != 2 {
				t.Fatalf("duplicate or missing delivery: %+v", p)
			}
		}
		points := metrics["beat.missed"].Data.(metricdata.Sum[int64]).DataPoints
		if len(points) != 1 || points[0].Value != 1 {
			t.Fatal(points)
		}
	})
}

func TestNilMeter(t *testing.T) {
	if _, err := otelbeat.MakeHandler(nil); err == nil {
		t.Fatal("nil accepted")
	}
}
