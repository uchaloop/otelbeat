package otelbeat_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/uchaloop/beat"
	"github.com/uchaloop/otelbeat"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// collect builds a handler over a manual reader, hands it every record, and
// returns what the SDK exported.
func collect(t *testing.T, recs []beat.Record, opts ...otelbeat.Option) metricdata.ResourceMetrics {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	h, err := otelbeat.New(mp.Meter("test"), opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	for _, r := range recs {
		h.Handle(ctx, r)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	return rm
}

func TestHandler_RecordsEveryInstrument(t *testing.T) {
	rm := collect(t, []beat.Record{
		{
			Duration: 50 * time.Millisecond, Processed: 5, Missed: 2,
			Mode: beat.ModeFixedRate, Outcome: beat.OutcomeOK,
		},
		{Err: errors.New("x"), Mode: beat.ModeFixedRate, Outcome: beat.OutcomeError},
		{
			Err:  &beat.PanicError{Value: "boom"},
			Mode: beat.ModeFixedDelay, Outcome: beat.OutcomePanic,
		},
	})

	durations := histogram(t, rm, "beat.run.duration")
	if got := totalCount(durations); got != 3 {
		t.Errorf("total run count = %d, want 3", got)
	}
	if c := countFor(durations, "panic", "fixed_delay"); c != 1 {
		t.Errorf("panic/fixed_delay count = %d, want 1", c)
	}
	if c := countFor(durations, "error", "fixed_rate"); c != 1 {
		t.Errorf("error/fixed_rate count = %d, want 1", c)
	}

	if v := valueFor(sum(t, rm, "beat.processed"), "ok", "fixed_rate"); v != 5 {
		t.Errorf("processed ok/fixed_rate = %d, want 5", v)
	}
	if v := valueForMode(sum(t, rm, "beat.missed"), "fixed_rate"); v != 2 {
		t.Errorf("missed fixed_rate = %d, want 2", v)
	}
}

func TestHandler_Saturation(t *testing.T) {
	rm := collect(t, []beat.Record{{
		Duration: time.Second, Period: 2 * time.Second,
		Mode: beat.ModeFixedRate, Outcome: beat.OutcomeOK,
	}})

	if got := sumFor(histogram(t, rm, "beat.run.saturation"), "ok", "fixed_rate"); got != 0.5 {
		t.Errorf("saturation = %v, want 0.5", got)
	}
}

func TestHandler_Lateness(t *testing.T) {
	scheduled := time.Now()

	rm := collect(t, []beat.Record{{
		ScheduledFor: scheduled,
		Start:        scheduled.Add(250 * time.Millisecond),
		Mode:         beat.ModeFixedRate, Outcome: beat.OutcomeOK,
	}})

	if got := sumFor(histogram(t, rm, "beat.run.lateness"), "ok", "fixed_rate"); got != 0.25 {
		t.Errorf("lateness = %v, want 0.25", got)
	}
}

func TestHandler_LatenessNeverGoesNegative(t *testing.T) {
	scheduled := time.Now()

	// A clock stepping back can put the start before the point it aimed at.
	rm := collect(t, []beat.Record{{
		ScheduledFor: scheduled,
		Start:        scheduled.Add(-time.Second),
		Mode:         beat.ModeFixedRate, Outcome: beat.OutcomeOK,
	}})

	if got := sumFor(histogram(t, rm, "beat.run.lateness"), "ok", "fixed_rate"); got != 0 {
		t.Errorf("lateness = %v, want 0", got)
	}
}

func TestHandler_SkipsRatiosItCannotCompute(t *testing.T) {
	// No Period to divide by and no scheduled point to measure from.
	rm := collect(t, []beat.Record{{
		Duration: time.Second, Mode: beat.ModeFixedRate, Outcome: beat.OutcomeOK,
	}})

	for _, name := range []string{"beat.run.saturation", "beat.run.lateness"} {
		if h, ok := optionalHistogram(rm, name); ok && totalCount(h) != 0 {
			t.Errorf("%s recorded %d points, want none", name, totalCount(h))
		}
	}

	// The instruments that always apply still recorded.
	if got := totalCount(histogram(t, rm, "beat.run.duration")); got != 1 {
		t.Errorf("duration count = %d, want 1", got)
	}
}

func TestHandler_OutcomeLabelsEveryInstrument(t *testing.T) {
	rm := collect(t, []beat.Record{{
		Duration: time.Second, Period: 2 * time.Second, Processed: 1, Missed: 1,
		ScheduledFor: time.Now(), Start: time.Now(),
		Mode: beat.ModeFixedRate, Outcome: beat.OutcomeTimeout,
	}})

	for _, name := range []string{"beat.run.duration", "beat.run.saturation", "beat.run.lateness"} {
		if c := countFor(histogram(t, rm, name), "timeout", "fixed_rate"); c != 1 {
			t.Errorf("%s timeout count = %d, want 1", name, c)
		}
	}
	if v := valueFor(sum(t, rm, "beat.processed"), "timeout", "fixed_rate"); v != 1 {
		t.Errorf("beat.processed timeout value = %d, want 1", v)
	}

	// beat.missed is the exception: the points it counts belong to the gap
	// before this run, so labelling them with this run's outcome would
	// attribute the loss to whatever happened to come next.
	missed := sum(t, rm, "beat.missed")
	if v := valueForMode(missed, "fixed_rate"); v != 1 {
		t.Errorf("beat.missed fixed_rate = %d, want 1", v)
	}
	if hasAttr(missed, "outcome") {
		t.Error("beat.missed carries an outcome attribute, want mode only")
	}
}

func TestHandler_WithOutcomeOverrides(t *testing.T) {
	rm := collect(t,
		[]beat.Record{{Mode: beat.ModeFixedRate, Outcome: beat.OutcomeError}},
		otelbeat.WithOutcome(func(beat.Record) string { return "throttled" }),
	)

	if c := countFor(histogram(t, rm, "beat.run.duration"), "throttled", "fixed_rate"); c != 1 {
		t.Errorf("custom outcome count = %d, want 1", c)
	}
}

func TestDefaultOutcome(t *testing.T) {
	if got := otelbeat.DefaultOutcome(beat.Record{Outcome: beat.OutcomeCanceled}); got != "canceled" {
		t.Errorf("DefaultOutcome = %q, want canceled", got)
	}

	// A Record assembled by hand rather than by beat carries no outcome; an
	// empty label would be worse than saying so.
	if got := otelbeat.DefaultOutcome(beat.Record{}); got != "unknown" {
		t.Errorf("DefaultOutcome of a bare Record = %q, want unknown", got)
	}
}

func TestHandler_ClampsNegativeCounts(t *testing.T) {
	// A Job that violates its contract with a negative count must not corrupt
	// the monotonic counters.
	rm := collect(t, []beat.Record{{
		Processed: -5, Missed: -1, Mode: beat.ModeFixedRate, Outcome: beat.OutcomeOK,
	}})

	if v := valueFor(sum(t, rm, "beat.processed"), "ok", "fixed_rate"); v != 0 {
		t.Errorf("processed = %d, want 0 (negative clamped)", v)
	}
	if v := valueForMode(sum(t, rm, "beat.missed"), "fixed_rate"); v != 0 {
		t.Errorf("missed = %d, want 0 (negative clamped)", v)
	}
}

func TestNew_RequiresMeter(t *testing.T) {
	if _, err := otelbeat.New(nil); err == nil {
		t.Fatal("expected error for nil meter")
	}
}

// TestHandler_BucketBoundaries pins the boundaries the package asks for and
// confirms the SDK honours them. They are advisory in the API, so this is the
// test that says the advice is taken.
func TestHandler_BucketBoundaries(t *testing.T) {
	rm := collect(t, []beat.Record{{
		Duration: time.Second, Period: 2 * time.Second,
		ScheduledFor: time.Now(), Start: time.Now(),
		Mode: beat.ModeFixedRate, Outcome: beat.OutcomeOK,
	}})

	want := map[string][]float64{
		"beat.run.duration": {
			0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300, 600,
		},
		"beat.run.saturation": {
			0.1, 0.25, 0.5, 0.7, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 2, 4,
		},
		"beat.run.lateness": {
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60,
		},
	}

	for name, bounds := range want {
		h := histogram(t, rm, name)
		if len(h.DataPoints) != 1 {
			t.Fatalf("%s has %d data points, want 1", name, len(h.DataPoints))
		}
		if got := h.DataPoints[0].Bounds; !slices.Equal(got, bounds) {
			t.Errorf("%s bounds = %v, want %v", name, got, bounds)
		}
	}
}

// TestHandler_SaturationSeparatesTheDecisionRange is the regression the
// boundaries exist for. On the SDK defaults - built for milliseconds and not
// rescaled for a bare ratio - 0.1, 0.8 and 1.2 all landed in (0, 5], so no
// quantile of this histogram could tell headroom from overrun.
func TestHandler_SaturationSeparatesTheDecisionRange(t *testing.T) {
	var recs []beat.Record
	for _, sat := range []float64{0.1, 0.8, 1.2} {
		recs = append(recs, beat.Record{
			Duration: time.Duration(sat * float64(time.Second)),
			Period:   time.Second,
			Mode:     beat.ModeFixedRate, Outcome: beat.OutcomeOK,
		})
	}

	if got := filled(histogram(t, collect(t, recs), "beat.run.saturation")); len(got) != 3 {
		t.Errorf("0.1, 0.8 and 1.2 filled %d buckets (indices %v), want 3", len(got), got)
	}
}

// TestHandler_LatenessSeparatesMillisecondsFromSeconds is the same regression
// on the other seconds-valued histogram: 5ms and 3s used to share a bucket.
func TestHandler_LatenessSeparatesMillisecondsFromSeconds(t *testing.T) {
	base := time.Now()

	var recs []beat.Record
	for _, late := range []time.Duration{5 * time.Millisecond, 3 * time.Second} {
		recs = append(recs, beat.Record{
			ScheduledFor: base, Start: base.Add(late),
			Mode: beat.ModeFixedRate, Outcome: beat.OutcomeOK,
		})
	}

	if got := filled(histogram(t, collect(t, recs), "beat.run.lateness")); len(got) != 2 {
		t.Errorf("5ms and 3s filled %d buckets (indices %v), want 2", len(got), got)
	}
}

// filled reports the indices of the buckets that received anything.
func filled(h metricdata.Histogram[float64]) []int {
	var idx []int
	for _, dp := range h.DataPoints {
		for i, c := range dp.BucketCounts {
			if c > 0 {
				idx = append(idx, i)
			}
		}
	}

	return idx
}

func histogram(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Histogram[float64] {
	t.Helper()

	h, ok := optionalHistogram(rm, name)
	if !ok {
		t.Fatalf("metric %q not found", name)
	}

	return h
}

func optionalHistogram(rm metricdata.ResourceMetrics, name string) (metricdata.Histogram[float64], bool) {
	m, ok := findMetric(rm, name)
	if !ok {
		return metricdata.Histogram[float64]{}, false
	}

	h, ok := m.Data.(metricdata.Histogram[float64])

	return h, ok
}

func sum(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Sum[int64] {
	t.Helper()

	m, ok := findMetric(rm, name)
	if !ok {
		t.Fatalf("metric %q not found", name)
	}

	s, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("%s is %T, want Sum[int64]", name, m.Data)
	}

	return s
}

func findMetric(rm metricdata.ResourceMetrics, name string) (metricdata.Metrics, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m, true
			}
		}
	}

	return metricdata.Metrics{}, false
}

func totalCount(h metricdata.Histogram[float64]) uint64 {
	var total uint64
	for _, dp := range h.DataPoints {
		total += dp.Count
	}

	return total
}

func countFor(h metricdata.Histogram[float64], outcome, mode string) uint64 {
	for _, dp := range h.DataPoints {
		if matches(dp.Attributes, outcome, mode) {
			return dp.Count
		}
	}

	return 0
}

func sumFor(h metricdata.Histogram[float64], outcome, mode string) float64 {
	for _, dp := range h.DataPoints {
		if matches(dp.Attributes, outcome, mode) {
			return dp.Sum
		}
	}

	return -1
}

// valueForMode finds a data point by mode alone, for the counter that carries
// nothing else.
func valueForMode(s metricdata.Sum[int64], mode string) int64 {
	for _, dp := range s.DataPoints {
		if attr(dp.Attributes, "mode") == mode {
			return dp.Value
		}
	}

	return -1
}

func hasAttr(s metricdata.Sum[int64], key string) bool {
	for _, dp := range s.DataPoints {
		if _, ok := dp.Attributes.Value(attribute.Key(key)); ok {
			return true
		}
	}

	return false
}

func valueFor(s metricdata.Sum[int64], outcome, mode string) int64 {
	for _, dp := range s.DataPoints {
		if matches(dp.Attributes, outcome, mode) {
			return dp.Value
		}
	}

	return 0
}

func matches(set attribute.Set, outcome, mode string) bool {
	return attr(set, "outcome") == outcome && attr(set, "mode") == mode
}

func attr(set attribute.Set, key string) string {
	v, ok := set.Value(attribute.Key(key))
	if !ok {
		return ""
	}

	return v.AsString()
}
