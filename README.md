# otelbeat

OpenTelemetry metrics for [beat](https://github.com/uchaloop/beat). The package
implements `beat.Handler`; the application supplies a Meter and owns the SDK,
Resource, exporter and shutdown. Requires Go 1.27 or later.

## Usage

Given an application-configured `metric.MeterProvider` and a `beat.Job`:

```go
handler, err := otelbeat.New(provider.Meter("beat"))
if err != nil {
    return err
}
runner, err := beat.MakeBeat(beat.Config{
    Period: 5 * time.Minute, JobTimeout: 4 * time.Minute,
    Jitter: 5 * time.Minute,
}, job, handler)
if err != nil {
    return err
}
```

Start and stop the runner through its lifecycle. Stop beat before shutting down
the MeterProvider so the final completion can be recorded and exported.
To add logging, supply `beat.MultiHandler(handler, logHandler)`.

```mermaid
flowchart LR
    J[Job returns] --> R[beat.Record]
    R --> H[otelbeat Handler]
    H --> M[Meter: record measurements]
    M --> S[Application SDK and Views]
    S --> E[Reader and exporter]
    E --> B[Metrics backend]
```

Compilable examples of provider wiring and custom outcomes are in
[example_test.go](example_test.go).

## Instruments

| Name | Type / unit | Measurement | Attributes |
|---|---|---|---|
| `beat.run.duration` | Histogram / seconds | Job elapsed time, including middleware | `mode`, `outcome` |
| `beat.run.saturation` | Histogram / ratio | Duration / Period | `mode`, `outcome` |
| `beat.run.lateness` | Histogram / seconds | max(0, Start − ScheduledFor) | `mode`, `outcome` |
| `beat.processed` | Counter / items | Nonnegative Job-reported processed count | `mode`, `outcome` |
| `beat.missed` | Counter / points | Previously computed unintentional grid losses | `mode` |

The duration histogram count counts **completed** runs. No completion is recorded
while a Job is still running. Duration excludes Handler and backoff time.

Default outcomes are `ok`, `error`, `panic`, `timeout`, and `canceled`, taken from
`Record.Outcome`; an empty outcome becomes `unknown`. Modes are `fixed_rate` and
`fixed_delay`. Panic records require beat's recovery middleware.

`beat.missed` excludes points bypassed by intentional backoff. Its count is
reported with the next completed Job, so shutdown or a stuck Job can prevent
its delivery. It has no outcome attribute because the reporting run's outcome
does not describe the lost points. In fixed-delay mode Missed is always zero,
and saturation compares work duration with the configured rest interval.

A slow Handler can cause grid losses without increasing the next run's lateness.
Neither saturation nor Missed alone determines whether to add replicas: also
observe queue depth, oldest-item age, arrival rate and dependency capacity.
An unchanged completion count can suggest a stuck Job only when the expected
maximum interval without completions is known; backoff and telemetry failure
can produce the same signal. Cumulative histogram samples may still be exported.

## Histogram boundaries

Boundaries are advisory; an SDK View can override them.

| Histogram | Boundaries |
|---|---|
| Duration (s) | 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300, 600 |
| Saturation | 0.1, 0.25, 0.5, 0.7, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 2, 4 |
| Lateness (s) | 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60 |

Values above the last boundary still contribute to the overflow bucket.
For manually constructed Records, a nonpositive Period omits saturation and a
zero ScheduledFor omits lateness. Negative processed and missed counts are
clamped to zero.

## Custom outcomes

Keep labels to a small, fixed set. Preserve timeout, cancellation and panic
classifications when adding domain-specific error categories:

```go
handler, err := otelbeat.New(meter,
    otelbeat.WithOutcome(func(r beat.Record) string {
        if r.Outcome == beat.OutcomeError && errors.Is(r.Err, errThrottled) {
            return "throttled"
        }
        return otelbeat.DefaultOutcome(r)
    }),
)
```

Here `errThrottled` is an application sentinel. Do not use error messages or item
identifiers as outcome labels.

## Replicas and Fx

Set service and instance identity on the application's OTel Resource, for example
`service.name`, `service.instance.id` and `k8s.cluster.name`. Ensure the exporter
or collector preserves the identity needed by your backend; Resource attributes
are not necessarily copied into every metric's labels. otelbeat does not modify
the Resource. Aggregate processed rates across the intended service's replicas.

Given `provider` with static type `metric.MeterProvider`, `cfg` and `job`:

```go
fx.New(
    fx.Provide(func() metric.MeterProvider { return provider }),
    fx.Supply(cfg),
    fx.Provide(func() beat.Job { return job }),
    otelbeatfx.Module(),
    beatfx.Module(),
)
```

The application must start/stop the Fx app and arrange provider shutdown after
beat. `otelbeatfx.Module` supplies a Handler on the `"beat"` meter.

## Example queries

For a Prometheus pipeline using the names below, filter to the intended service
and cluster. Exporter naming and resource-label mappings can differ.

```promql
# Completed runs and processed items per second.
sum(rate(beat_run_duration_seconds_count[30m]))
sum(rate(beat_processed_total[30m]))

# Completed runs classified as failures.
sum(rate(beat_run_duration_seconds_count{outcome=~"error|panic|timeout"}[30m]))

# p95 ratio of Job duration to Period, grouped by scheduling mode.
histogram_quantile(0.95,
  sum by (le, mode) (rate(beat_run_saturation_bucket[30m])))

# Unintentional grid losses per second.
sum(rate(beat_missed_total[30m]))
```

Choose a query window with enough completions for your configured period.

[API reference](https://pkg.go.dev/github.com/uchaloop/otelbeat) ·
[MIT license](LICENSE)
