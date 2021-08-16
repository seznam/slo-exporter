# Configuration
Slo exporter itself is configured using one [base YAML file](#base-config). 
Path to this file is configured using the `--config-file` flag.
Additional configuration files might be needed by some modules
depending on their needs and if they are used in the pipeline at all.

#### ENV variables
Every configuration option in the base YAML file can be overridden by using ENV variable.
The schema of ENV variable naming is `SLO_EXPORTER_` prefix and than in uppercase any key of the YAML
structure in uppercase without any underscores. Underscores are used for separating nested structures.
Example: `SLO_EXPORTER_WEBSERVERLISTENADDRESS=0.0.0.0:8080` or for module configuration `SLO_EXPORTER_TAILER_TAILEDFILE=access.log`

#### CMD flags
```bash
$ ./slo_exporter --help-long
usage: slo_exporter --config-file=CONFIG-FILE [<flags>]

Flags:
  --help                     Show context-sensitive help (also try --help-long and --help-man).
  --config-file=CONFIG-FILE  SLO exporter configuration file.
  --log-level="info"         Log level (error, warn, info, debug,trace).
  --check-config             Only check config file and exit with 0 if ok and other status code if not.
```

#### Processing pipeline
Slo-exporter allows to dynamically compose the pipeline structure,
but there is few basic rules it needs to follow:
  - [`ingester`](architecture.md#ingester) module cannot be at the beginning of pipeline.
  - [`ingester`](architecture.md#ingester) module can only be linked to preceding [`producer`](architecture.md#producer) module.
  - Type of produced event by the preceding module must match the ingested type of the following one.


### Base config
```yaml
# Address where the web interface should listen on.
webServerListenAddress: "0.0.0.0:8080"
# Maximum time to wait for all events to be processed after receiving SIGTERM or SIGINT.
maximumGracefulShutdownDuration: "10s"
# How long to wait after processing pipeline has been shutdown before stopping http server w metric serving.
# Useful to make sure metrics are scraped by Prometheus. Ideally set it to Prometheus scrape interval + 1s or more.
# Should be less or equal to afterPipelineShutdownDelay
afterPipelineShutdownDelay: "1s"

# Defines architecture of the pipeline how the event will be processed by the modules.
pipeline: [<moduleType>]

# Contains configuration for distinct pipeline module.
modules:
  <moduleType>: <moduleConfig>
```

### `moduleType`:

##### Producers:
Only produces new events from the specified data source.
  - [`envoy_access_log_server`](modules/envoy_access_log_server.md)
  - [`tailer`](modules/tailer.md)
  - [`prometheusIngester`](modules/prometheus_ingester.md)
  - [`envoyAccessLogServer`](modules/envoy_access_log_server.md)
  - [`kafkaIngester`](modules/kafka_ingester.md)
  
##### Processors:
Reads input events, does some processing based in the module type and produces modified event.
  - [`eventKeyGenerator`](modules/event_key_generator.md)
  - [`metadataClassifier`](modules/metadata_classifier.md)
  - [`relabel`](modules/relabel.md)
  - [`dynamicClassifier`](modules/dynamic_classifier.md)
  - [`statisticalClassifier`](modules/statistical_classifier.md)
  - [`sloEventProducer`](modules/slo_event_producer.md)
  
##### Ingesters:
Only reads input events but does not produce any.
  - [`prometheusExporter`](modules/prometheus_exporter.md)

Details how they work and their `moduleConfig` can be found in their own
linked documentation in the [docs/modules](modules) folder.

#### Configuration examples
Actual examples of usage with full configuration can be found in the [`examples/`](examples) directory.

#### Configuration testing
If you want to verify that your configuration is valid, use the `--check-config` flag.
Slo-exporter then just verifies if the configuration is valid and exits with status 0 if ok and 1 if not.
