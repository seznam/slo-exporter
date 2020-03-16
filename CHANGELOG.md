# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
## [Unreleased]
### Added
- Possibility to add additional metadata labels to `events_processed_total` metric of dynamic classifier using `unclassifiedEventMetadataKeys`.


## [2.3.0] - 2020-03-16
### Added
- Slo_rules now support honor_slo_result.
- All unknown named groups parsed by tailer are set as HTTP headers.

### Fixed
- Delayed graceful shutdown.

### Changed
- **BREAKING** Graceful shutdown timeout conf options.
  - gracefulShutdownTimeout replaced with maximumGracefulShutdownDuration.
  - afterGracefulShutdownDelay replaced with minimumGracefulShutdownDuration.

## [2.2.0] - 2020-03-16
### Added
 - app_build_info metric

## [2.1.1] - 2020-03-13
### Fixed
- Fixed hanging shutdown when all modules ended without explicit termination request.

## [2.1.0] - 2020-03-12
### Changed
- Tailer line matching regular expression is now part of configuration.
- Tailer is able to initialize event with SloClassification, if provided within log line.

### Fixed
- Update dynamic classifier cache with data from already classified event.
- E2e-tests' run\_tests.sh is now checking whether slo\_exporter is running, before test will proceed.

## [2.0.0] - 2020-03-09
### Added
 - Implemented prometheus ingester.
 - Optional gracefulShutdownTimeout configuration.

### Changed
 - **BREAKING** Request event normalizer now uses regular expressions for filtering.
     - Config option `filteredHttpStatusCodes` is now `filteredHttpStatusCodeMatchers` and is list of regular expressions instead of integers.
     - Config option `filteredHttpHeaders` is now `filteredHttpHeaderMatchers` and is map of regular expression matching header name to regular expression matching header value.

### Fixed
 - Refactored loading of event normalizer configuration.
 - StringMap.Without fixed logic and added tests.
 - StringMap.Merge do not return empty map when merged with nil.

## [1.4.0] - 2020-02-28

### Added
 - Implemented graceful shutdown for each pipeline processor.

## [1.3.0] - 2020-02-28

### Added
 - Process multiple domains from single data source.

### Fixed
 - Typo in label failedToClassify.

## [1.2.3] - 2020-02-27

### Added
  - Normalize also `.ico` files as `:image`.

## [1.2.2] - 2020-02-20

### Config
  - Frontend API GraphQL endpoints without `operationName` are now classified as `no_slo`.

## [1.2.1] - 2020-02-19
