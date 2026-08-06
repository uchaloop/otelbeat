// Package otelbeat records OpenTelemetry metrics for a beat.Beat. It implements
// beat.Handler and, per run, records two instruments on a meter the application
// supplies:
//
//   - beat.run.duration (histogram, seconds) - its count also gives the number
//     of runs;
//   - beat.processed (counter) - items processed, summed.
//
// Both carry the attributes "status" and "mode", so a single query sliced by
// those labels covers success/error/panic counts, latency and throughput per
// scheduling mode. The application owns the MeterProvider and its
// exporter/reader; otelbeat records, it does not export.
package otelbeat

import (
	"context"
	"errors"

	"github.com/uchaloop/beat"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// StatusFunc classifies a run into the value of the "status" attribute. Use it
// to map domain errors to your own codes; DefaultStatus gives "ok", "error" or
// "panic".
type StatusFunc func(beat.Record) string

type settings struct {
	status StatusFunc
}

// Option configures the handler built by New.
type Option func(*settings)

// WithStatus overrides how a run is classified into the "status" attribute. The
// default is DefaultStatus. A nil func is ignored. Keep the set of returned
// values small - each distinct value is a separate metric series.
func WithStatus(f StatusFunc) Option {
	return func(s *settings) {
		if f != nil {
			s.status = f
		}
	}
}

type handler struct {
	duration  metric.Float64Histogram
	processed metric.Int64Counter
	status    StatusFunc
}

// New builds a beat.Handler that records the instruments described in the
// package doc on meter. The application owns the meter and its exporter.
func New(meter metric.Meter, opts ...Option) (beat.Handler, error) {
	if meter == nil {
		return nil, errors.New("meter is required")
	}

	s := settings{status: DefaultStatus}
	for _, o := range opts {
		if o != nil {
			o(&s)
		}
	}

	duration, err := meter.Float64Histogram(
		"beat.run.duration",
		metric.WithDescription("Duration of a single beat run."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	processed, err := meter.Int64Counter(
		"beat.processed",
		metric.WithDescription("Items processed per run, summed."),
		metric.WithUnit("{item}"),
	)
	if err != nil {
		return nil, err
	}

	return &handler{duration: duration, processed: processed, status: s.status}, nil
}

func (h *handler) Handle(ctx context.Context, r beat.Record) {
	attrs := metric.WithAttributes(
		attribute.String("status", h.status(r)),
		attribute.String("mode", string(r.Mode)),
	)

	h.duration.Record(ctx, r.Duration.Seconds(), attrs)

	// Clamp to guard the monotonic counter against a Job that violates its
	// contract with a negative count.
	h.processed.Add(ctx, int64(max(0, r.Processed)), attrs)
}

// DefaultStatus classifies a run as "ok", "panic" (a recovered panic - a
// *beat.PanicError) or "error". Build on it in a custom StatusFunc.
//
// The "panic" status only appears when beat's recovery middleware is in use;
// without it a panicking Job crashes the process and produces no Record.
func DefaultStatus(r beat.Record) string {
	switch {
	case r.Err == nil:
		return "ok"
	case isPanic(r.Err):
		return "panic"
	default:
		return "error"
	}
}

func isPanic(err error) bool {
	var pe *beat.PanicError

	return errors.As(err, &pe)
}
