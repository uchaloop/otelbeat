# otelbeat

[![CI](https://github.com/uchaloop/otelbeat/actions/workflows/ci.yml/badge.svg)](https://github.com/uchaloop/otelbeat/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/uchaloop/otelbeat.svg)](https://pkg.go.dev/github.com/uchaloop/otelbeat)
[![License: MIT](https://img.shields.io/badge/github/license/uchaloop/otelbeat)](LICENSE)

OpenTelemetry metrics for scheduled jobs running with
[`beat`](https://github.com/uchaloop/beat).

## Installation

```bash
go get github.com/uchaloop/otelbeat
```

## Fx

Provide a `metric.MeterProvider`, then install the metrics handler before the
beat module:

```go
fx.New(
	// Provide your metric.MeterProvider and exporter.
	otelbeatfx.Module(),
	beatfx.Module(),
)
```

## Without Fx

```go
handler, err := otelbeat.New(
	meterProvider.Meter("beat"),
)
```

The returned value implements `beat.Handler`.

## Metrics

| Instrument | Type | Description |
|---|---|---|
| `beat.run.duration` | Float64 histogram, seconds | Job duration and run count |
| `beat.processed` | Int64 counter, items | Number of processed items |

Both instruments include:

- `status`: `ok`, `error`, or `panic`;
- `mode`: `interval` or `cron`.

The `panic` status is available when beat's recovery middleware converts the
panic into `*beat.PanicError`.

## Custom status

Map application errors to custom status values:

```go
handler, err := otelbeat.New(
	meter,
	otelbeat.WithStatus(func(record beat.Record) string {
		switch {
		case errors.Is(record.Err, ErrThrottled):
			return "throttled"
		default:
			return otelbeat.DefaultStatus(record)
		}
	}),
)
```

Keep the number of status values small to avoid high-cardinality metrics.

## Prometheus queries

Run rate:

```promql
rate(beat_run_duration_seconds_count[5m])
```

Error rate:

```promql
rate(beat_run_duration_seconds_count{status="error"}[5m])
```

P95 duration:

```promql
histogram_quantile(0.95, sum by (le) (rate(beat_run_duration_seconds_bucket[5m])))
```

Processing throughput:

```promql
rate(beat_processed_total[5m])
```

## Acknowledgements

I am grateful to the authors of
[OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go). Their
work made this library possible.

## License

[MIT](LICENSE)
