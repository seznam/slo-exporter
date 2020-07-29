# Prometheus SLO example

This example shows simple configuration of slo-exporter using as a source
of events Prometheus API to compute SLO of Prometheus itself.

#### How to run it
In root of the repo
```bash
make build
cd examples/prometheus
../../slo_exporter --config-file slo_exporter.yaml
```
Once started see http://localhost:8080/metrics.

## How is SLO computed

Prometheus has few basic functionalities which should be covered by its SLOs.

## Observed SLO types
#### `api_availability`
Uses Prometheus counter `prometheus_http_requests_total` which has label `handler` and `code`.
It generates event for every request with the corresponding metadata using
[the `prometheus_ingester` modules `counter_increase` query type](../../docs/modules/prometheus_ingester.md#type-counter_increase).
Label `handler` holds information which endpoint has been called. That is used as an event key to classify its importance (SLO class).
Label `code` contains resulting status code of the request. That is used to decide if the event was success (<500) or fail(>=500).

#### `api_latency`
Uses Prometheus histogram `prometheus_http_request_duration_seconds_bucket` describing response latency distribution of all requests.
It generates event for every request with the corresponding metadata and additional values from the `le` labels about
upper and lower boundary of the bucket the event falls into using
[the `prometheus_ingester` modules `histogram_increase` query type](../../docs/modules/prometheus_ingester.md#type-histogram_increase).
Then the lower boundary, holding the minimum duration the request took, is compared with the latency threshold to decide is request was success or fail.
The latency has different thresholds for distinct SLO classes, so the rules are separate for each of them.
