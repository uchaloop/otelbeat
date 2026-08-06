package otelbeat_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/uchaloop/beat"
	"github.com/uchaloop/otelbeat"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestHandler_RecordsDurationAndProcessed(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	h, err := otelbeat.New(mp.Meter("test"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	h.Handle(ctx, beat.Record{Duration: 50 * time.Millisecond, Processed: 5, Mode: beat.ModeCron})
	h.Handle(ctx, beat.Record{Err: errors.New("x"), Mode: beat.ModeCron})
	h.Handle(ctx, beat.Record{Err: &beat.PanicError{Value: "boom"}, Mode: beat.ModeInterval})

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	durations := histogram(t, rm, "beat.run.duration")
	if got := totalCount(durations); got != 3 {
		t.Errorf("total run count = %d, want 3", got)
	}
	if c := countFor(durations, "panic", "interval"); c != 1 {
		t.Errorf("panic/interval count = %d, want 1", c)
	}
	if c := countFor(durations, "error", "cron"); c != 1 {
		t.Errorf("error/cron count = %d, want 1", c)
	}

	processed := sum(t, rm, "beat.processed")
	if v := valueFor(processed, "ok", "cron"); v != 5 {
		t.Errorf("processed ok/cron = %d, want 5", v)
	}
}

func TestHandler_WithStatus_Overrides(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	h, err := otelbeat.New(mp.Meter("test"), otelbeat.WithStatus(func(beat.Record) string {
		return "2"
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	h.Handle(context.Background(), beat.Record{Mode: beat.ModeCron})

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	if c := countFor(histogram(t, rm, "beat.run.duration"), "2", "cron"); c != 1 {
		t.Errorf("custom status count = %d, want 1", c)
	}
}

func TestNew_RequiresMeter(t *testing.T) {
	if _, err := otelbeat.New(nil); err == nil {
		t.Fatal("expected error for nil meter")
	}
}

func TestHandler_ClampsNegativeProcessed(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	h, err := otelbeat.New(mp.Meter("test"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// A Job that violates its contract with a negative count must not corrupt
	// the monotonic counter.
	h.Handle(context.Background(), beat.Record{Processed: -5, Mode: beat.ModeCron})

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	if v := valueFor(sum(t, rm, "beat.processed"), "ok", "cron"); v != 0 {
		t.Errorf("processed = %d, want 0 (negative clamped)", v)
	}
}

func histogram(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Histogram[float64] {
	t.Helper()

	m := findMetric(t, rm, name)
	h, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("%s is %T, want Histogram[float64]", name, m.Data)
	}

	return h
}

func sum(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Sum[int64] {
	t.Helper()

	m := findMetric(t, rm, name)
	s, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("%s is %T, want Sum[int64]", name, m.Data)
	}

	return s
}

func findMetric(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}

	t.Fatalf("metric %q not found", name)

	return metricdata.Metrics{}
}

func totalCount(h metricdata.Histogram[float64]) uint64 {
	var total uint64
	for _, dp := range h.DataPoints {
		total += dp.Count
	}

	return total
}

func countFor(h metricdata.Histogram[float64], status, mode string) uint64 {
	for _, dp := range h.DataPoints {
		if attr(dp.Attributes, "status") == status && attr(dp.Attributes, "mode") == mode {
			return dp.Count
		}
	}

	return 0
}

func valueFor(s metricdata.Sum[int64], status, mode string) int64 {
	for _, dp := range s.DataPoints {
		if attr(dp.Attributes, "status") == status && attr(dp.Attributes, "mode") == mode {
			return dp.Value
		}
	}

	return 0
}

func attr(set attribute.Set, key string) string {
	v, ok := set.Value(attribute.Key(key))
	if !ok {
		return ""
	}

	return v.AsString()
}
