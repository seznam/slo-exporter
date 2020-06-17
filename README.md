# SLO exporter

[![pipeline status](https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/badges/master/pipeline.svg)](https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/commits/master)
[![coverage report](https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/badges/master/coverage.svg)](https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/commits/master)
[![godoc badge](https://godoc.org/github.com/prometheus/prometheus?status.svg)](https://sklik-devops.gitlab.seznam.net/slo-exporter/godoc/pkg/gitlab.seznam.net/sklik-devops/slo-exporter/)

[CHANGELOG](./CHANGELOG.md)

slo-exporter is golang tool used for
 * reading events from different sources (a log file, prometheus metrics, ...)
 * processing the events (filtering, normalization and classification)
 * exporting SLO metrics based on the processed events

## Architecture
It is built using [the pipeline pattern](https://blog.golang.org/pipelines).
The processed event is passed from one module to another to allow it's modification or filtering
for the final state to be reported as an SLI event.

The flow of the processing pipeline can be dynamically set using configuration file, so it can be used
for various use cases and event types.

### Module types
There is set of implemented modules to be used and are divided to three basic types based on their input/output.

- `producer` does not read any events but produces them. These modules serve as sources of the events.
- `ingester` reads events but does not produce any. These modules serves for reporting the SLO metrics to some external systems.
- `processor` is combination of `producer` and `ingester`. It reads an event and produces new or modified one.


### Pipeline rules
The pipeline can be composed dynamically but there are some basic rules it needs to follow:
  - `ingester` module cannot be at the beginning of pipeline.
  - `ingester` module can only be linked to preceding `producer` module.
  - Type of produced event by the preceding module must match the ingested type of the following one.

## Configuration
Slo exporter itself is configured using one YAML file. Path to this file is configured using the `--config-file` flag.
Additional configuration files might be needed by some pipeline modules depending on their needs and if they are used in the pipeline at all.
```bash
slo_exporter --config-file=config.yaml
```

This yaml has basic structure of:
```yaml
# Address where the web interface should listen on.
webServerListenAddress: "0.0.0.0:8080"
# Maximum time to wait for all events to be processed after receiving SIGTERM or SIGINT.
maximumGracefulShutdownDuration: "10s"
# How long to wait after processing pipeline has been shutdown before stopping http server w metric serving.
# Useful to make sure metrics are scraped by Prometheus. Ideally set it to Prometheus scrape interval + 1s or more.
# Should be less or equal to afterPipelineShutdownDelay
afterPipelineShutdownDelay: "1s"

# Defines architecture of the pipeline.
pipeline: [<moduleType>]

# Contains configuration for distinct pipeline module.
modules:
  <moduleType>: <moduleConfig>
```

Possible `moduleType`:

- producers:
    - [`tailer`](./docs/modules/tailer.md)
    - [`prometheusIngester`](./docs/modules/prometheus_ingester.md)
- processors:
    - [`eventKeyGenerator`](./docs/modules/event_key_generator.md)
    - [`metadataClassifier`](./docs/modules/metadata_classifier.md)
    - [`dynamicClassifier`](./docs/modules/dynamic_classifier.md)
    - [`statisticalClassifier`](./docs/modules/statistical_classifier.md)
    - [`sloEventProducer`](./docs/modules/slo_event_producer.md)
- ingesters:
    - [`prometheusExporter`](./docs/modules/prometheus_exporter.md)

Details how they work and their `moduleConfig` can be found in their own
linked documentation in the [docs/modules](./docs/modules) folder.

#### Configuration examples
Actual examples of usage with full configuration can be found in the [`examples/`](examples) directory.

## Build

It is recommended to build slo-exporter with golang 1.13+.

```bash
make
```

## Debugging
If you need to dynamically change the log level of the application, you can use the `/logging` HTTP endpoint.
To set the log level use the `POST` method with URL parameter `level` of value `error`, `warning`, `info` or `debug`.

Example using `cURL`
```bash
# Use GET to get current log level.
$ curl -s http://0.0.0.0:8080/logging
current logging level is: debug

# Use POST to set the log level.
$ curl -XPOST -s http://0.0.0.0:8080/logging?level=info
logging level set to: info
```

#### Profiling
In case of issues with leaking resources for example, slo-exporter supports the
Go profiling using pprof on `/debug/pprof/` web interface path. For usage see the official [docs](https://golang.org/pkg/net/http/pprof/).


## Frequently asked questions

### How to add new normalization replacement rule?
Event normalization can be done using the `relabel` module, see [its documentation](docs/modules/relabel.md).

### How to deal with malformed lines?
Before !87. If you are seeing too many malformed lines then you should inspect [tailer package](pkg/tailer/tailer.go) and seek for variable `lineParseRegexp`.
After !87, slo-exporter main config supports to specify custom regular expression in field `.module.tailer.loglineParseRegexp`.

- [Code coverage](https://sklik-devops.gitlab.seznam.net/slo-exporter/coverage.html)
