# Kafka ingester

|                |                         |
|----------------|-------------------------|
| `moduleName`   | `kafkaIngester`         |
| Module type    | `producer`              |
| Output event   | `raw`                   |

Kafka ingester generates events from Kafka messages.

`moduleConfig`
```yaml
# Allow verbose logging of events within Kafka library. Global logger with its configured log level is used.
logKafkaEvents: false
# Allow logging of errors within Kafka library. Global logger with its configured log level is used.
logKafkaErrors: true
# List of Kafka brokers
brokers:
  - <string> # e.g. kafka-1.example.com:9092
topic: ""
groupId: ""
# commitInterval indicates the interval at which offsets are committed to the broker.
# If 0 (default), commits will be handled synchronously.
commitInterval: <duration> # e.g. 0, 5s, 10m
# retentionTime optionally sets the length of time the consumer group will be saved by the broker.
# Default: 24h
retentionTime: <duration>
# fallbackStartOffset determines from whence the consumer group should begin consuming when it finds a partition without a committed offset.
# Default: FirstOffset
fallbackStartOffset: <LastOffset|FirstOffset>
# eventIdMetadataKey its value will be used as a unique id for the generated event if present (hint: use a trace ID if possible).
eventIdMetadataKey: <string>
```


For every received message from Kafka:
- data in Key is ignored
- data in Value is unmarshalled according to the schema version specified in Kafka message header `slo-exporter-schema-version` (defaults to `v1` if none specified).

### Supported data schemas
#### `v1`
```
{
    "metadata": {
        "name": "eventName"
        ...
    },
    # Defaults to 1 if none specified
    "quantity": "10",
    "slo_classification": {
        "app": "testApp",
        "class": "critical",
        "domain": "testDomain"
    }
}
```

Strictly speaking, none of the keys is mandatory, however please note that:
- Event with explicitly set quantity=0 is basically noop for Producer module. To give an example, prometheusExporter does not increment any SLO metric for such events.
- Event with empty Metadata does not allow much logic in following modules.
- In case you want to allow ingesting events without SLO classification, you need to make sure that all events are classified within rest of the SLO exporter pipeline.
