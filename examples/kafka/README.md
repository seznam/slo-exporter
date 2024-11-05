# Kafka ingester SLO example

This example shows a simple configuration of slo-exporter using
[`kafka_ingester`](/docs/modules/kafka_ingester.md)
as a source of data in order to compute SLO of a server which publishes events through Kafka.

#### How to run it
In root of the repo
```bash
make build
cd examples/kafka
docker-compose up -d
```
Once started see http://localhost:8080/metrics.

## How SLO is computed
Kafkacat is used to publish events to Kafka on behalf of an imaginary server. Each event contains its SLO classification together with metadata.

## Observed SLO types
#### `availability`
All events whose "result" metadata's key equals to "OK" are considered successful.

#### `quality`
All events whose all quality degradation tracking metadata's key(s) equals to 0 are considered successful.  
