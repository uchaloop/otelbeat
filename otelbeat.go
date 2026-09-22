// Package otelbeat implements optional beat.Handler instrumentation with OTel.
// It records job execution through oteljob exactly once and adds scheduling
// metrics. Do not deliver the same Result separately to oteljob.
//
// beat.run.lateness (seconds) records max(0, Start - ScheduledFor) when both
// timestamps exist. beat.missed ({point}) counts missed scheduled points. Both
// carry mode alone, normalized to fixed_rate, fixed_delay or unknown. Missed
// points describe the gap before this attempt, not its outcome. Intentional
// backoff is excluded by beat. Losses arrive with the next record and can be
// lost on shutdown or when work blocks. uint64 increments above MaxInt64 are
// capped at MaxInt64 before recording in OTel's signed counter.
//
// The application owns the SDK, Resource, Views, exporters and shutdown. The
// handler makes no network calls itself and does not log errors or create traces.
package otelbeat

import (
	"context"
	"errors"
	"math"

	"github.com/uchaloop/beat"
	"github.com/uchaloop/job"
	"github.com/uchaloop/oteljob"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MakeHandler builds execution and scheduling instrumentation on meter.
// Install it explicitly with beat.WithHandler, or compose it with custom
// observers using beat.MultiHandler. Omitting it leaves beat uninstrumented.
func MakeHandler(meter metric.Meter) (beat.Handler, error) {
	if meter == nil {
		return nil, errors.New("meter is required")
	}

	execution, err := oteljob.MakeHandler(meter)
	if err != nil {
		return nil, err
	}

	lateness, err := meter.Float64Histogram("beat.run.lateness",
		metric.WithDescription("Delay between the scheduled and actual start of a run."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 300))
	if err != nil {
		return nil, err
	}

	missed, err := meter.Int64Counter("beat.missed",
		metric.WithDescription("Scheduled points that passed without a run, excluding intentional backoff."),
		metric.WithUnit("{point}"))
	if err != nil {
		return nil, err
	}

	return &handler{execution: execution, lateness: lateness, missed: missed}, nil
}

type handler struct {
	execution job.Handler
	lateness  metric.Float64Histogram
	missed    metric.Int64Counter
}

func (h *handler) Handle(ctx context.Context, record beat.Record) {
	h.execution.Handle(ctx, record.Result)

	mode := string(record.Mode)
	if record.Mode != beat.ModeFixedRate && record.Mode != beat.ModeFixedDelay {
		mode = "unknown"
	}

	attrs := metric.WithAttributes(attribute.String("mode", mode))
	h.missed.Add(ctx, int64(min(record.Missed, uint64(math.MaxInt64))), attrs)

	if !record.Result.Start.IsZero() && !record.ScheduledFor.IsZero() {
		h.lateness.Record(ctx, max(0, record.Result.Start.Sub(record.ScheduledFor)).Seconds(), attrs)
	}
}
