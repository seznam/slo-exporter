# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased
### Added
- [#77](https://github.com/seznam/slo-exporter/pull/77) *Experimental* slo-rules-generator tool

### Changed
- [#78](https://github.com/seznam/slo-exporter/pull/78) Upgrade dependencies mainly to avoid reported CVEs in grafana/loki
- [#80](https://github.com/seznam/slo-exporter/pull/80) prometheus-ingester: Add staleness support and possibility to have shorter query interval

## [v6.10.0] 2021-09-17
### Added
- [#69](https://github.com/seznam/slo-exporter/pull/69) *Experimental* Exemplars support for the `prometheus_exporter` module.

### Fixed
- [#71](https://github.com/seznam/slo-exporter/pull/71) Fix corner cases in StringMap.Merge(), StringMap.Without()

## [v6.9.0] 2021-07-14
### Added
- [#60](https://github.com/seznam/slo-exporter/pull/60) New module eventMetadataRenamer

### Changed
- [#54](https://github.com/seznam/slo-exporter/pull/54) Use strict unmarshal in relabel module
- [#57](https://github.com/seznam/slo-exporter/pull/57) Upgrade to Go 1.16

## [v6.8.0] 2021-03-24
### Added
- [#48](https://github.com/seznam/slo-exporter/pull/48) New ingester module [`kafkaIngester`](docs/modules/kafka_ingester.md)

## [v6.7.1] 2021-02-15
### Fixed
- [#44](https://github.com/seznam/slo-exporter/issues/44) Install missing ca-certificates to docker base image

## [v6.7.0] 2021-01-29
### Changed
- [#43](https://github.com/seznam/slo-exporter/pull/40) slo-event-producer: `slo_matcher` values are now regular expressions.

## [v6.6.0] 2021-01-19
### Added
- [#37](https://github.com/seznam/slo-exporter/pull/37) New ingester module [`envoyAccessLogServer`](docs/modules/envoy_access_log_server.md) to receive remote [access logs from an Envoy proxy over the gRPC](https://www.envoyproxy.io/docs/envoy/latest/api-v3/data/accesslog/v3/accesslog.proto).

### Fixed
- [#39](https://github.com/seznam/slo-exporter/pull/39) Prometheus-exporter: Additional SLO event metadata does not get overwritten.

### Changed
- [#40](https://github.com/seznam/slo-exporter/pull/40) Upgrade to Go 1.15

## [v6.5.0] 2020-11-11
### Changed
- [#28](https://github.com/seznam/slo-exporter/pull/28) Abstracted statistical classifier history to generic interface in new storage package.
- [#34](https://github.com/seznam/slo-exporter/pull/34) Prometheus recording rules: Unified computation for SLO classes uptime, uptime-alt (slo-expoter's SLO)
- [#34](https://github.com/seznam/slo-exporter/pull/34) Prometheus alerts: missing data alert now explicitly matches only enabled SLO domains
- [#32](https://github.com/seznam/slo-exporter/pull/32) Prometheus alerts, recording rules: now fully reflects our internal setup
- [#31](https://github.com/seznam/slo-exporter/pull/31) Grafana dashboard: SLO Drilldown shows current increase of events on background of error budget change

### Fixed
- [#33](https://github.com/seznam/slo-exporter/pull/33) prometheus-ingester now always expose query_fails_total

### Added
- [#32](https://github.com/seznam/slo-exporter/pull/32) Grafana dashboard: SLO Effective Burn-rate

## [v6.4.1] 2020-08-25
### Fixed
- [#27](https://github.com/seznam/slo-exporter/pull/27) Empty configuration is now evaluated as invalid

## [v6.4.0] 2020-08-07
### Added
- [#25](https://github.com/seznam/slo-exporter/pull/25) `--version` command-line flag showing only the build version

## [v6.3.0] 2020-08-05
### Added
- [#24](https://github.com/seznam/slo-exporter/pull/24) DynamicClassifier: Allow use of comments in the CSV files, see [the docs](./docs/modules/dynamic_classifier.md#csv-comments).

### Fixed
- [#24](https://github.com/seznam/slo-exporter/pull/24) DynamicClassifier: Panic on CSV with unexpected number of fields.

## [v6.3.0-rc1] 2020-08-04
### Changed
- [#18](https://github.com/seznam/slo-exporter/pull/18) Tailer: Upgraded hpcloud/tail package to latest commit.

### Added
- [#20](https://github.com/seznam/slo-exporter/pull/20) CI:
     - Release additional Docker image tags:
        - `latest` Latest released version.
        - `vX` Latest released version for the major version.
        - `vX.Y` Latest released version for the minor version.
     - Prebuilt binaries for:
        - `slo-exporter_darwin_386`
        - `slo-exporter_darwin_amd64`
        - `slo-exporter_linux_386`
        - `slo-exporter_linux_amd64`
        - `slo-exporter_windows_386`
        - `slo-exporter_windows_amd64`

## [v6.2.0] 2020-07-29
### Added
- CI pipeline
  - build
  - on release, publish docker image and github release with binaries

## [v6.1.0] 2020-06-30
### Changed
- Dockerfile labels
- Dockerfile src image

### Added
- SLO computation recording rules, alerts
- slo-exporter grafana dashboard

### Fixed
- PrometheusIngester: Fixed unit of the `slo_exporter_prometheus_ingester_query_duration_seconds_bucket` metric.

## [v6.0.0] 2020-06-12
### Changed
- **BREAKING** Dropped the `normalizer` module in favour of the `relabel` and `eventKeyGenerator` modules.
  Those can be used to sanitize metadata values and compose the event key from any metadata keys.
- **BREAKING** Dropped the `eventFilter` module in favour of the `relabel` module.
- **BREAKING** sloEventProducer: dropped `honor_slo_result` configuration option, same behavior can be now achieved using failure conditions and metadata filters.
- **BREAKING** sloEventProducer: all operators renamed
- Dynamic classifier no longer exposes status_code label to unclassified events metric, you have to explicitly specify it in the `unclassifiedEventMetadataKeys` configuration option.
- **BREAKING** Tailer: no longer sets the event key, use the `eventKeyGenerator` module.
- **BREAKING** Tailer: no longer sets the SLO classification of the event, use the `metadataClassifier` module.

## [v5.6.0] 2020-06-10
### Added
- New module [`relabel`](/docs/modules/relabel.md) allowing to modify event metadata using Prometheus relabel config.
- New flag `--check-config` to verify if configuration is ok.
- Enabled mutex and block profiling.

### Changed
- Upgraded to Go 1.14
- PrometheusIngester: added `query_type` label for `slo_exporter_prometheus_ingester_query_fails_total` metric

### Fixed
- PrometheusIngester: fixed isolation of `histogram_increase` query type causing invalid computation of increase.

## [v5.5.0] 2020-05-28
### Added
- sloEventProducer: notEqualTo, notMatchesRegexp, numberNotEqualTo operators

## [v5.4.1] 2020-05-27
### Fixed
- PrometheusIngester: last defined query shadows the previous ones

## [v5.4.0] 2020-05-22
### Changed
- DynamicClassifier: Log entire event on unsuccessful classification.

### Added
- sloEventProducer: Expose metrics based on slo_rules configuration (`exposeRulesAsMetrics` slo_event_producer module option).

## [v5.3.0] 2020-05-05
### Added
- New Prometheus-ingester's query configuration option 'resultAsQuantity'

## [v5.2.0] 2020-05-05
### Added
- New module `metadataClassifier` to classify event based on it's metadata. See [the docs](docs/modules/metadata_classifier.md) for more info.

## [v5.1.1] 2020-05-04
### Changed
- prometheus_exporter's LabelNames.sloApp default value now matches documented one (slo_app)

## [v5.1.0] 2020-05-4
### Added
- sloEventProducer: Added new operators `equalTo`, `numberEqualTo`, `numberEqualOrHigherThan` and `numberEqualOrLessThan`.

## [v5.0.0] 2020-04-30
### Changed
- **BREAKING** prometheusIngester: Renamed `increase` query type to `counter_increase`.
- **BREAKING** sloEventProducer: Dropped unused configuration key `event_type` in rules file.

### Added
- Prometheus ingester: New query type `histogram_increase` to generate events with for each bucket.

## [v4.4.0] 2020-04-27
### Added
- Prometheus ingester 'increase' query type
- New module `eventKeyGenerator` to generate event key from its metadata. See the [docs](docs/modules/event_key_generator.md).
- Generic metadata matcher for slo_event_producer

### Changed
- Prometheus ingester query types. (Existing named as 'simple')
- Generalize eventKey access/usage for all ingesters by putting it to event.Metadata

## [v4.3.0] 2020-04-08
### Added
- Possibility to dynamically set the log level using HTTP endpoint, [see the docs](./README.md#debugging).
- StatisticalClassifier module now allows to set default weights.

### Fixed
- Fixed logging in reporting metrics from prometheus exporter module.

## [v4.2.1] 2020-04-07
### Fixed
- Roll back mutex and blocking goroutine profiling in `pprof` beacause of [an issue in go 1.14.1](https://github.com/golang/go/issues/37967).

## [v4.2.0] 2020-04-07
### Added
- New module `statisticalClassifier` to classify events based on previous events classification statistical distribution.
  Read more in [the module documentation](docs/modules/statistical_classifier.md).
- Enabled mutex and blocking goroutine profiling in `pprof`.

## [v4.1.0] 2020-03-30
### Fixed
- Fixed CPU usage burst caused by leaking timers.

### Added
- Added Go debugging handlers to web interface on `/debug/pprof/`. For usage see [https://blog.golang.org/pprof](https://blog.golang.org/pprof).

## [v4.0.3] 2020-03-30
### Fixed
- Removed redundant namespacing for dynamic classifier metric `events_processed_total`.

## [v4.0.2] 2020-03-29
### Fixed
- Kubernetes manifests fixed to use `/liveness` probe endpoint.

## [v4.0.1] 2020-03-29
### Fixed
- Kubernetes manifests fixed to match the breaking changes in 4.0.0.

## [v4.0.0] 2020-03-26
### Changed
- **BREAKING** Pipeline structure is now defined using the `pipeline` configuration option.
    For more information see [the architecture documentation](README.md#architecture).
- **BREAKING** The `log_level` configuration option was removed and replaced with the `--log-level` command line flag.
    Also it can be still configured with the ENV variable `SLO_EXPORTER_LOGLEVEL`.
- **BREAKING** The `--disable-timescale-exporter` and `--disable-prometheus-exporter` flags were dropped
    in favor of dynamic pipeline structure configuration.
- **BREAKING** The timescale exporter was dropped.
- **BREAKING** The `minimumGracefulShutdownDuration` configuration option was replaced with `afterPipelineShutdownDelay` to be more intuitive.

## [v3.2.0] - 2020-03-24
### Refactored
- HttpRequest.Headers, HttpRequest.Metadata is now filled only with data not matching conf.tailer.emptyGroupRE.
- Drop frpcStatus as a dedicated attribute for HttpRequest.

## [v3.1.0] - 2020-03-20

### Refactored
- If eventKey matching group in tailer RE is nonempty, its value is propagated to HttpRequest.EventKey.

## [v3.0.0] - 2020-03-20
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

## [v2.4.0] - 2020-03-16
### Added
- Possibility to add additional metadata labels to `events_processed_total` metric of dynamic classifier using `unclassifiedEventMetadataKeys`.

## [v2.3.0] - 2020-03-16
### Added
- Slo_rules now support honor_slo_result.
- All unknown named groups parsed by tailer are set as HTTP headers.

### Fixed
- Delayed graceful shutdown.

### Changed
- **BREAKING** Graceful shutdown timeout conf options.
  - gracefulShutdownTimeout replaced with maximumGracefulShutdownDuration.
  - afterGracefulShutdownDelay replaced with minimumGracefulShutdownDuration.

## [v2.2.0] - 2020-03-16
### Added
 - app_build_info metric

## [v2.1.1] - 2020-03-13
### Fixed
- Fixed hanging shutdown when all modules ended without explicit termination request.

## [v2.1.0] - 2020-03-12
### Changed
- Tailer line matching regular expression is now part of configuration.
- Tailer is able to initialize event with SloClassification, if provided within log line.

### Fixed
- Update dynamic classifier cache with data from already classified event.
- E2e-tests' run\_tests.sh is now checking whether slo\_exporter is running, before test will proceed.

## [v2.0.0] - 2020-03-09
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

## [v1.4.0] - 2020-02-28

### Added
 - Implemented graceful shutdown for each pipeline processor.

## [v1.3.0] - 2020-02-28

### Added
 - Process multiple domains from single data source.

### Fixed
 - Typo in label failedToClassify.

## [v1.2.3] - 2020-02-27

### Added
  - Normalize also `.ico` files as `:image`.

## [v1.2.2] - 2020-02-20

### Config
  - Frontend API GraphQL endpoints without `operationName` are now classified as `no_slo`.

## [v1.2.1] - 2020-02-19
