# Changelog

All notable changes to this module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-09-01

### Changed

- The package documentation carries what each instrument means, how a run is
  classified into its status, and why the attributes stop at two; the README is a
  landing page.
- The module is built with Go 1.27. A module that depends on this one has to
  declare 1.27 as well.

## [0.1.1] - 2026-08-06

### Changed

- Reworked the README as concise, user-focused documentation.

## [0.1.0] - 2026-08-06

### Added

- Initial release: `otelbeat` implements `beat.Handler` and records OpenTelemetry metrics for a `beat` scheduler.
- `beat.run.duration` histogram (seconds) - its count also gives the number of runs.
- `beat.processed` counter - items processed, summed.
- `status` (`ok`/`error`/`panic`) and `mode` (`interval`/`cron`) attributes on both instruments.
- `WithStatus` option and exported `DefaultStatus` to map runs to custom status codes.
- Fx integration in `otelbeat/otelbeatfx`: `otelbeatfx.Module` provides the `beat.Handler` from the container's `metric.MeterProvider`; the core `otelbeat` package has no Fx dependency.

[Unreleased]: https://github.com/uchaloop/otelbeat/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/uchaloop/otelbeat/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/uchaloop/otelbeat/releases/tag/v0.1.0
