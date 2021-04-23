# Envoy proxy SLO example

This example shows a simple configuration of slo-exporter using
[`envoy access-log-server module`](/docs/modules/envoy_access_log_server.md).

#### How to run it
In root of the repo
```bash
make docker
cd examples/envoy_proxy
docker-compose up -d
```
Once started, see http://localhost:8001/metrics.

## How SLO is computed
- [envoyAccessLogServer module](/docs/modules/envoy_access_log_server.md) is used to receive envoy's logs via grpc.
- [relabel module](/docs/modules/relabel.md) drops the unwanted events (e.g. based on its HTTP status code, userAgent,...).
- [metadataClassifier module](/docs/modules/metadata_classifier.md) classifies generated event based HTTP headers sent by a client

## Observed SLO types
Refer to [slo_rules.yaml](./slo-exporter/slo_rules.yaml) for the exact configuration of how SLO events are generated based on input logs/events.

#### `availability`
For every log line which results in classified event in domain `test-domain`, an SLO event is generated. Its result is determined based on statusCode metadata key - with all events with `statusCode > 500` being marked as failed.

#### `latency90`, `latency99`
For every log line which results in classified event in domain `test-domain` and slo_class `critical`, an SLO event is generated. Its result is determined based on `timeToLastDownstreamTxByte` metadata key.
