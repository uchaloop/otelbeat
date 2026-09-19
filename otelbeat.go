// Package otelbeat implements beat.Handler using OpenTelemetry metrics.
// New accepts an application-provided Meter. The application owns the SDK,
// Resource, Views, reader/exporter and shutdown; stop beat before the provider.
//
// # Instruments
//
// beat.run.duration records Job duration in seconds; its count counts completed
// runs. beat.run.saturation records Duration / Period. beat.run.lateness records
// nonnegative Start minus ScheduledFor in seconds. beat.processed accumulates
// Job-reported items. These instruments carry mode and outcome attributes.
// DefaultOutcome uses Record.Outcome, or "unknown" when it is empty. WithOutcome
// supplies a custom classifier; keep its output to a small fixed set.
//
// beat.missed accumulates unintentional grid losses and carries mode alone.
// Intentional backoff is excluded. Losses arrive with the next completed Job,
// so shutdown or a stuck Job can prevent their delivery. Missed is zero in
// fixed-delay mode. The current run's outcome cannot identify the loss's cause.
//
// All histograms have explicit advisory boundaries, overridable by SDK Views.
// Nonpositive Period omits saturation; zero ScheduledFor omits lateness.
// Negative processed and missed counts are clamped to zero.
//
// # Interpretation
//
// Metrics describe execution, not queue capacity. Scaling decisions also need
// queue depth, oldest-item age, arrival rate and dependency capacity. A slow
// Handler can cause missed points without lateness. An unchanged completion
// count suggests a stuck Job only with a known maximum interval between
// completions; backoff and telemetry failure can produce the same signal.
//
// Put service, instance and cluster identity on the application's Resource and
// configure the export pipeline to retain the distinctions needed in queries.
// otelbeat does not modify Resource attributes. The otelbeatfx package provides
// an Fx integration. See the README for examples, boundaries and queries.
package otelbeat

import (
	"context"
	"errors"

	"github.com/uchaloop/beat"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Bucket boundaries for the three histograms. They are advisory - the SDK
// applies them unless the application overrides the aggregation with a View -
// and they exist because the SDK's own defaults assume milliseconds and are not
// rescaled for an instrument's unit.
var (
	// durationBounds spans the range a beat job plausibly takes, from a few
	// milliseconds through ten minutes; larger values enter the overflow bucket.
	durationBounds = []float64{
		0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300, 600,
	}

	// saturationBounds are dense around one, with coarser buckets outside
	// the 0.7 to 1.25 range.
	saturationBounds = []float64{
		0.1, 0.25, 0.5, 0.7, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 2, 4,
	}

	// latenessBounds start at a millisecond, because a healthy process sits
	// near zero and the interesting news is how far above it a run has drifted.
	latenessBounds = []float64{
		0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60,
	}
)

// OutcomeFunc classifies a run into the value of the "outcome" attribute. Use
// it to map domain errors to your own codes; DefaultOutcome passes through what
// beat already decided.
type OutcomeFunc func(beat.Record) string

type settings struct {
	outcome OutcomeFunc
}

// Option configures the handler built by New.
type Option func(*settings)

// WithOutcome overrides how a run is classified into the "outcome" attribute.
// The default is DefaultOutcome. A nil func is ignored. Keep the set of returned
// values small - each distinct value is a separate metric series.
func WithOutcome(f OutcomeFunc) Option {
	return func(s *settings) {
		if f != nil {
			s.outcome = f
		}
	}
}

type metricsHandler struct {
	duration   metric.Float64Histogram
	saturation metric.Float64Histogram
	lateness   metric.Float64Histogram
	processed  metric.Int64Counter
	missed     metric.Int64Counter

	outcome OutcomeFunc
}

// New builds a beat.Handler that records the instruments described in the
// package doc on meter. The application owns the meter and its exporter.
func New(meter metric.Meter, opts ...Option) (beat.Handler, error) {
	if meter == nil {
		return nil, errors.New("meter is required")
	}

	s := settings{outcome: DefaultOutcome}
	for _, o := range opts {
		if o != nil {
			o(&s)
		}
	}

	h := metricsHandler{outcome: s.outcome}

	var err error
	if h.duration, err = meter.Float64Histogram(
		"beat.run.duration",
		metric.WithDescription("Duration of a single beat run."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBounds...),
	); err != nil {
		return nil, err
	}

	if h.saturation, err = meter.Float64Histogram(
		"beat.run.saturation",
		metric.WithDescription("Run duration as a fraction of the configured period."),
		metric.WithUnit("1"),
		metric.WithExplicitBucketBoundaries(saturationBounds...),
	); err != nil {
		return nil, err
	}

	if h.lateness, err = meter.Float64Histogram(
		"beat.run.lateness",
		metric.WithDescription("How long after its scheduled point a run began."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latenessBounds...),
	); err != nil {
		return nil, err
	}

	if h.processed, err = meter.Int64Counter(
		"beat.processed",
		metric.WithDescription("Items processed per run, summed."),
		metric.WithUnit("{item}"),
	); err != nil {
		return nil, err
	}

	if h.missed, err = meter.Int64Counter(
		"beat.missed",
		metric.WithDescription("Scheduled points that passed without a run."),
		metric.WithUnit("{point}"),
	); err != nil {
		return nil, err
	}

	return &h, nil
}

func (h *metricsHandler) Handle(ctx context.Context, record beat.Record) {
	mode := attribute.String("mode", string(record.Mode))
	runAttributes := metric.WithAttributes(attribute.String("outcome", h.outcome(record)), mode)

	h.duration.Record(ctx, record.Duration.Seconds(), runAttributes)

	// Clamp to guard the monotonic counters against a Job that violates its
	// contract with a negative count.
	h.processed.Add(ctx, int64(max(0, record.Processed)), runAttributes)

	// Missed points carry the mode and nothing else: they belong to the gap
	// before this run rather than to the run itself, so this run's outcome would
	// attribute the loss to the wrong thing.
	h.missed.Add(ctx, int64(max(0, record.Missed)), metric.WithAttributes(mode))

	// Saturation needs a period to divide by, and lateness needs the point the
	// run aimed at. beat fills both; a Record assembled by hand may not.
	if record.Period > 0 {
		h.saturation.Record(ctx, record.Duration.Seconds()/record.Period.Seconds(), runAttributes)
	}

	if !record.ScheduledFor.IsZero() {
		// A run that started early - a clock stepping back - is not negatively
		// late, and a histogram of seconds should not carry that.
		h.lateness.Record(ctx, max(0, record.Start.Sub(record.ScheduledFor)).Seconds(), runAttributes)
	}
}

// DefaultOutcome reports the classification beat made in Record.Outcome: "ok",
// "error", "panic", "timeout" or "canceled". Build on it in a custom
// OutcomeFunc.
//
// The "panic" outcome only appears when beat's recovery middleware is in use;
// without it a panicking Job crashes the process and produces no Record. A
// Record that carries no outcome at all - one assembled by hand rather than by
// beat - reads as "unknown" rather than as an empty label.
func DefaultOutcome(r beat.Record) string {
	if len(r.Outcome) == 0 {
		return "unknown"
	}

	return string(r.Outcome)
}
