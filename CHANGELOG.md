# Changelog

## [0.4.0] - 2026-09-22

### Breaking changes

- Replace `New` and custom outcome options with `MakeHandler`.
- Delegate execution measurements to oteljob's `job.run.duration` and
  `job.run.processed`; replace the processed counter with a batch-size histogram.
  Count completed attempts using the duration count with `phase="total"`.
- Remove `beat.run.saturation`. Keep `beat.run.lateness` and `beat.missed`
  with `mode` alone, without attributing scheduling losses to the current outcome.
- Move the Fx adapter to the independent `github.com/uchaloop/otelbeatfx` module; remove Fx from this module.

### Added

- Record work, error processing and total duration, with independent error-handler
  outcomes through oteljob. Instrumentation remains explicitly opt-in.
- Add an English Beat Grafana dashboard with configurable datasource, import
  instructions, Prometheus queries and metric interpretation guidance.

### Fixed

- Cap unsigned missed-point increments at MaxInt64 before recording them in OTel.
- Omit execution measurements for unstarted work and negative processed counts;
  omit lateness when either required timestamp is missing.
- Normalize unrecognized mode and outcome values to `unknown` to bound cardinality.

## [0.3.0] - 2026-09-19

Requires beat 0.4.0.

### Breaking changes

- Replaced `status` with `outcome`, using `Record.Outcome`.
- Renamed `StatusFunc`, `WithStatus` and `DefaultStatus` to their Outcome equivalents.
- Mode values are `fixed_rate` and `fixed_delay`.

### Added

- `beat.run.saturation` and `beat.run.lateness` histograms.
- `beat.missed` counter with only the mode attribute; excludes intentional backoff.
- Explicit advisory boundaries for all histograms.
- Provider wiring examples, telemetry diagram and metric interpretation guide.

### Fixed

- Empty outcomes use `unknown`. Negative counters and lateness are clamped to zero.
- Missing Period or ScheduledFor omits the corresponding derived measurement.

## [0.2.0] - 2026-09-01

- Required Go 1.27 and expanded package documentation.

## [0.1.1] - 2026-08-06

- Updated README.

## [0.1.0] - 2026-08-06

- Initial release: duration histogram, processed counter, custom status
  classification and Fx integration.

[0.3.0]: https://github.com/uchaloop/otelbeat/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/uchaloop/otelbeat/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/uchaloop/otelbeat/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/uchaloop/otelbeat/releases/tag/v0.1.0

[0.4.0]: https://github.com/uchaloop/otelbeat/compare/v0.3.0...v0.4.0
