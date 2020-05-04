# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [5.1.0] 2020-05-4
### Added
- sloEventProducer: Added new operators `equalTo`, `numberEqualTo`, `numberEqualOrHigherThan` and `numberEqualOrLessThan`.

## [5.0.0] 2020-04-30
### Changed
- **BREAKING** prometheusIngester: Renamed `increase` query type to `counter_increase`.
- **BREAKING** sloEventProducer: Dropped unused configuration key `event_type` in rules file.

### Added
- Prometheus ingester: New query type `histogram_increase` to generate events with for each bucket.

## [4.4.0] 2020-04-27
### Added
- Prometheus ingester 'increase' query type
- New module `eventKeyGenerator` to generate event key from its metadata. See the [docs](docs/modules/event_key_generator.md).
- Generic metadata matcher for slo_event_producer

### Changed
- Prometheus ingester query types. (Existing named as 'simple')
- Generalize eventKey access/usage for all ingesters by putting it to event.Metadata

## [4.3.0] 2020-04-08
### Added
- Possibility to dynamically set the log level using HTTP endpoint, [see the docs](./README.md#debugging).
- StatisticalClassifier module now allows to set default weights.

### Fixed
- Fixed logging in reporting metrics from prometheus exporter module.

## [4.2.1] 2020-04-07
### Fixed
- Roll back mutex and blocking goroutine profiling in `pprof` beacause of [an issue in go 1.14.1](https://github.com/golang/go/issues/37967).

## [4.2.0] 2020-04-07
### Added
- New module `statisticalClassifier` to classify events based on previous events classification statistical distribution.
  Read more in [the module documentation](docs/modules/statistical_classifier.md).
- Enabled mutex and blocking goroutine profiling in `pprof`.

## [4.1.0] 2020-03-30
### Fixed
- Fixed CPU usage burst caused by leaking timers.

### Added
- Added Go debugging handlers to web interface on `/debug/pprof/`. For usage see [https://blog.golang.org/pprof](https://blog.golang.org/pprof).

## [4.0.3] 2020-03-30
### Fixed
- Removed redundant namespacing for dynamic classifier metric `events_processed_total`.

## [4.0.2] 2020-03-29
### Fixed
- Kubernetes manifests fixed to use `/liveness` probe endpoint.

## [4.0.1] 2020-03-29
### Fixed
- Kubernetes manifests fixed to match the breaking changes in 4.0.0.

## [4.0.0] 2020-03-26
### Changed
- **BREAKING** Pipeline structure is now defined using the `pipeline` configuration option.
    For more information see [the architecture documentation](README.md#architecture).
- **BREAKING** The `log_level` configuration option was removed and replaced with the `--log-level` command line flag.
    Also it can be still configured with the ENV variable `SLO_EXPORTER_LOGLEVEL`.
- **BREAKING** The `--disable-timescale-exporter` and `--disable-prometheus-exporter` flags were dropped
    in favor of dynamic pipeline structure configuration.
- **BREAKING** The timescale exporter was dropped.
- **BREAKING** The `minimumGracefulShutdownDuration` configuration option was replaced with `afterPipelineShutdownDelay` to be more intuitive.

## [3.2.0] - 2020-03-24
### Refactored
- HttpRequest.Headers, HttpRequest.Metadata is now filled only with data not matching conf.tailer.emptyGroupRE.
- Drop frpcStatus as a dedicated attribute for HttpRequest.

## [3.1.0] - 2020-03-20

### Refactored
- If eventKey matching group in tailer RE is nonempty, its value is propagated to HttpRequest.EventKey.

## [3.0.0] - 2020-03-20
### Fixed
- Inconsistencies in aggregated SLO metrics exposed to Prometheus.
- Normalizer now does not drop event if eventKey already set.

### Refactored
- Refactored prometheus exporter to implement Collector interface and to not require known labels beforehand.

### Changed
- **BREAKING** Event filter module now filters using metadata using `metadataFilter`. Old `filteredHttpStatusCodeMatchers` and `filteredHttpHeaderMatchers` were dropped.
- **BREAKING** Failure criteria configuration synatx of sloEventProducer module has changed.
    - `failure_criteria` is now `failure_conditions`
    - `criterium` is now called `operator`
    - Operators are evaluated on event metadata. `key` field was added in order to specify on which metadata is the given operator to be evaluated.
    - Old criteria were dropped and newly available operators are `matchesRegexp`, `numberHigherThan` and `durationHigherThan`.
    - Example of new failure conditions syntax:
      ```yaml
      failure_conditions:
          - operator: matchesRegexp
            key: "metadataKey"
            value: ".*"
      ```

## [2.4.0] - 2020-03-16
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
