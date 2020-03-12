# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## UNRELEASED
### Changed
- tailer line matching regular expression is now part of configuration
- tailer is able to initialize event with SloClassification, if provided within log line
- getEventKey returns SloEndpoint if set

### Fixed
- update dynamic classifier cache with data from already classified event
- e2e-tests' run_tests.sh is now checking whether slo_exporter is running, before test will proceed

### Added
 - Add `status_code` label to dynamic classifier metric `events_processed_total`.

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
