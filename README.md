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

The flow of the processing pipeline can be dynamically set using configuration file so it can be used
for various use cases and event types.

### Module types
There is set of implemented modules to be used and are divided to three basic types based on their input/output.

- `producer` does not read any events but produces them. These modules serve as sources of the events.
- `ingester` reads events but does not produce any. These modules serves for reporting the SLO metrics to some external systems.
- `processor` is combination of `producer` and `ingester`. It reads event and produces new or modified one.


### Pipeline rules
The pipeline can be composed dynamically but there are some basic rules which needs be followed:
  - At the beginning of the pipeline can be only `producer` module.
  - `producer` module can be found only at the beginning of the pipeline.
  - Type of produced event of the preceding module must match the ingested type of the following one.

## Configuration
Slo exporter is configured using one YAML file. Path to this file is configured using the `--config-file` flag.
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
# Should be less or equal to maximumGracefulShutdownDuration
minimumGracefulShutdownDuration: "1s"

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
    - [`eventFilter`](./docs/modules/event_filter.md)
    - [`normalizer`](./docs/modules/normalizer.md)
    - [`dynamicClassifier`](./docs/modules/dynamic_classifier.md)
    - [`sloEventProducer`](./docs/modules/slo_event_producer.md)
- ingesters:
    - [`prometheusExporter`](./docs/modules/prometheus_exporter.md)

Details how they work and ther `moduleConfig` can be found in their own 
linked documentation in the [docs/modules](./docs/modules) folder.

Full documented example can be found in [conf/slo_exporter.yaml](conf/slo_exporter.yaml).

## Build

It is recommended to build slo-exporter with golang 1.13+.

```bash
make
```

## Frequently asked questions

### How to add new normalization replacement rule?

Event normalization is done in [`event normalizer`](pkg/normalizer/normalizer.go).
User can add normalization replacement rule in slo-exporter main config under key [`.modules.normalizer.replaceRules`](conf/slo_exporter.yaml).

Suppose you see a lot of events matching this regular expression `/api/v1/ppchit/rule/[0-9a-fA-F]{5,16}` which you want to normalize, then your normalization replacement rule can look like following snippet:

```yaml
...
modules:
  normalizer:
    replaceRules:
      - regexp: "/api/v1/ppchit/rule/[0-9a-fA-F]{5,16}"
        # Replacement of the matched path
        replacement: "/api/v1/ppchit/rule/0"
```

### How to deal with malformed lines?

Before !87. If you are seeing too many malformed lines then you should inspect [tailer package](pkg/tailer/tailer.go) and seek for variable `lineParseRegexp`.

After !87, slo-exporter main config supports to specify custom regular expression in field `.module.tailer.loglineParseRegexp`.

### How to deploy slo-exporter?

slo-exporter can be deployed as:
 1. sidecar container application tailing local (emptydir) (proxy) logs
     * manifest example can be found in [userproxy repository](https://gitlab.seznam.net/sklik-frontend/Proxies/tree/master/userproxy/kubernetes)
 1. standalone application tailing remote logs using [`htail` web page tailer over http](https://gitlab.seznam.net/Sklik-DevOps/htail)
     * manifest example can be found in [kubernetes directory](kubernetes/)




- [Code coverage](https://sklik-devops.gitlab.seznam.net/slo-exporter/coverage.html)
