# Nginx proxy SLO example

This example shows a simple configuration of slo-exporter using
[`tailer`](/docs/modules/tailer.md) to parse
Nginx proxy's logs as a source of data in order to compute SLO.

#### How to run it
In root of the repo
```bash
make build
cd examples/nginx_proxy
../../slo_exporter --config-file slo_exporter.yaml
```
Once started, see http://localhost:8080/metrics.

## How SLO is computed
- [tailer](/docs/modules/tailer.md) is used to parse the logs. Note the `modules.tailer.loglineParseRegexp` configuration which needs to match the used Nginx log format.
- [relabel](/docs/modules/relabel.md) drops the unwanted events (e.g. based on its HTTP status code, userAgent,...), normalize URI and eventually set a new event's metadata key (see `operationName`). Not all of this may be needed in your use case, but we include it to present an example use of this module.
- [dynamicClassifier](/docs/modules/dynamicClassifier.md) classifies generated event based on provided [classification.csv](./classification.csv)

## Observed SLO types
Refer to [slo_rules.yaml](./slo_rules.yaml) for the exact configuration of how SLO events are generated based on input logs/events.

#### `availability`
For every log line which results in classified event in domain `test-domain`, an SLO event is generated. Its result is determined based on statusCode metadata key - with all events with `statusCode > 500` being marked as failed.


#### `latency90`
For every log line which results in classified event in domain `test-domain` and slo_class `critical`, an SLO event is generated. Its result is determined based on requestDuration metadata key - with all events which took more than `0.8s` to process are being marked as failed. This SLO type represents 90th latency percentile - in other words, we expect that more then 90% of events meets this condition.