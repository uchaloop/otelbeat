# otelbeat

[![CI](https://github.com/uchaloop/otelbeat/actions/workflows/ci.yml/badge.svg)](https://github.com/uchaloop/otelbeat/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/uchaloop/otelbeat.svg)](https://pkg.go.dev/github.com/uchaloop/otelbeat)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

OpenTelemetry metrics for [`beat`](https://github.com/uchaloop/beat), the
background-job scheduler. `otelbeat` implements `beat.Handler` and records a
minimal, attribute-driven set of instruments per run on a meter the application
supplies.

The `beat` core has no metrics - it hands each run to a `Handler`. `otelbeat` is
that handler when you want OpenTelemetry. The dependency is one-directional
(`otelbeat` -> `beat`); the core never imports OpenTelemetry.

## Install

```bash
go get github.com/uchaloop/otelbeat
```

## Instruments

Two instruments, recorded per run:

| Instrument | Type | Typical Prometheus series |
|---|---|---|
| `beat.run.duration` | Float64Histogram (s) | `beat_run_duration_seconds_bucket` / `_sum` / `_count` |
| `beat.processed` | Int64Counter (`{item}`) | `beat_processed_total` |

The histogram's `_count` also gives the number of runs, so run/error/panic counts
and latency come from one instrument.

## Attributes

Both instruments carry:

- `status` - `ok` / `error` / `panic` (or your own codes, see `WithStatus`)
- `mode` - `interval` / `cron`

`status`(3) × `mode`(2) = 6 series per instrument - slice in Grafana, no metric
explosion.

The `panic` status requires beat's `recovery` middleware. Without it a panicking
Job crashes the process and records nothing.

## Usage

The application owns the `MeterProvider` and its exporter (Prometheus, OTLP, …);
`otelbeat` only records.

With Fx, `otelbeatfx.Module` provides the `beat.Handler` from the container's
`metric.MeterProvider`:

```go
import "github.com/uchaloop/otelbeat/otelbeatfx"

fx.New(
    // ... your MeterProvider ...
    otelbeatfx.Module(),
    beatfx.Module(),
)
```

Without Fx (or to customize), call `otelbeat.New` directly:

```go
handler, err := otelbeat.New(meterProvider.Meter("beat"))
```

The core package `github.com/uchaloop/otelbeat` has no Fx dependency; the Fx
integration lives in `github.com/uchaloop/otelbeat/otelbeatfx`.

Map domain errors to your own status codes with `WithStatus`, building on
`otelbeat.DefaultStatus`:

```go
otelbeat.New(meter, otelbeat.WithStatus(func(r beat.Record) string {
    var pe *beat.PanicError
    switch {
    case errors.As(r.Err, &pe):
        return "panic"
    case errors.Is(r.Err, ErrThrottled):
        return "2"
    case r.Err != nil:
        return "1"
    default:
        return "0"
    }
}))
```

## Example queries

```promql
rate(beat_run_duration_seconds_count[5m])                                   # run rate
rate(beat_run_duration_seconds_count{status="error"}[5m])                   # error rate
histogram_quantile(0.95, sum by(le)(rate(beat_run_duration_seconds_bucket[5m])))  # p95 latency
rate(beat_processed_total[5m])                                              # throughput
```

## Acknowledgements

`otelbeat` builds on [OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go).
Thanks to its authors and maintainers.

## License

MIT.
