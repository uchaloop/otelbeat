# otelbeat

[![CI](https://github.com/uchaloop/otelbeat/actions/workflows/ci.yml/badge.svg)](https://github.com/uchaloop/otelbeat/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/uchaloop/otelbeat.svg)](https://pkg.go.dev/github.com/uchaloop/otelbeat)
[![License: MIT](https://img.shields.io/github/license/uchaloop/otelbeat)](LICENSE)

OpenTelemetry metrics for scheduled jobs running with
[beat](https://github.com/uchaloop/beat). It implements `beat.Handler`, which
beat calls once per run.

- **Two instruments, two attributes** - enough to answer rate, errors, latency
  and throughput, few enough that cardinality stays constant.
- **The application owns the pipeline** - otelbeat records, it does not
  configure, aggregate or export.

```bash
go get github.com/uchaloop/otelbeat
```

## Quick start

```go
fx.New(
	// Provide your metric.MeterProvider and exporter.
	otelbeatfx.Module(),
	beatfx.Module(),
)
```

Without Fx:

```go
handler, err := otelbeat.New(meterProvider.Meter("beat"))
```

## What it records

| Instrument | Type | What it answers |
|---|---|---|
| `beat.run.duration` | float64 histogram, seconds | Duration, and run count through its own count |
| `beat.processed` | int64 counter, items | Throughput |

Both carry `status` (`ok`, `error`, `panic`) and `mode` (`interval`, `cron`).

```promql
rate(beat_run_duration_seconds_count[5m])                    # runs
rate(beat_run_duration_seconds_count{status="error"}[5m])    # failures
histogram_quantile(0.95, sum by (le) (rate(beat_run_duration_seconds_bucket[5m])))
rate(beat_processed_total[5m])                               # items
```

## Documentation

What each instrument means, how a run is classified, and why the attributes stop
at two are in the package documentation:
**[pkg.go.dev/github.com/uchaloop/otelbeat](https://pkg.go.dev/github.com/uchaloop/otelbeat)**.

## Acknowledgements

I am grateful to the authors of
[OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go). Their
work made this library possible.

## License

[MIT](LICENSE)
