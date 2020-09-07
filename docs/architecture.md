# Architecture
SLO-exporter is written in Go and built using [the pipeline pattern](https://blog.golang.org/pipelines).

The processed event is passed from one module to another to allow its modification or filtering
for the final state to be reported as an SLI event.

The flow of the processing pipeline can be dynamically set using configuration file, so it can be used
for various use cases and event types.

### Event Types
Slo-exporter differentiates between two event types:

##### Raw
This is an event which came from the data source, it has metadata and quantity
and you can set its event key which will be in the resulting metrics and can be used for classification of the event.

##### SLO event
 Final event generated from the raw event. This event has already evaluated result and classification
 an is then reported to output metrics.

### Module types
There is set of implemented modules to be used and are divided to three basic types based on their input/output.

##### `producer`
Does not read any events but produces them. These modules serve as sources of the events.

##### `ingester`
Reads events but does not produce any. These modules serves for reporting the SLO metrics to some external systems.

##### `processor`
Combination of `producer` and `ingester`. It reads an event and produces new or modified one.
