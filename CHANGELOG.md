# Changelog

## [Unreleased]

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

[Unreleased]: https://github.com/uchaloop/otelbeat/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/uchaloop/otelbeat/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/uchaloop/otelbeat/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/uchaloop/otelbeat/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/uchaloop/otelbeat/releases/tag/v0.1.0
