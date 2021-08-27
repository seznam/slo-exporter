# Prometheus exporter

|                |                      |
|----------------|----------------------|
| `moduleName`   | `prometheusExporter` |
| Module type    | `ingester`           |
| Input event    | `SLO`                |

This module exposes the SLO metrics in Prometheus format, so they can be
scraped, computed, visualized and alerted on.

SLO is often computed over long time ranges such as 4 weeks.
But on the other hand, for debugging it is essential to be able to distinct what event type
caused the issue. To allow this, this exporter exposes cascade of aggregated metrics (see the example below).
From the highest level over whole slo domain to the lowest granularity of each event type.

This way the alerting and usual visualization can use the high level metrics, but in case of issues
it's possible to drill down right to the root cause.

The `normalizer` module is intended to mitigate possible issues witch exploding of event type cardinality.
But to make sure, if any unique event type slips through, to avoid the cardinality explosion,
 the module allows to set maximum limit of exposed event types. any other new will be replaces with configured placeholder.

`moduleConfig`
```yaml
# Name of the resulting counter metric to be exposed representing counter of slo events by it's classification and result.
metricName: "slo_events_total"
# Limit of unique event keys, when exceeded, the event key in the label is replaced with placeholder.
maximumUniqueEventKeys: 1000
# Placeholder to replace new event keys when the limit is hit.
ExceededKeyLimitPlaceholder: "cardinalityLimitExceeded"
# Names of labels to be used for specific event information.
labelNames:
  # Contains information about the event result (success, fail, ...).
  result: "result"
  # Domain of the SLO event.
  sloDomain: "slo_domain"
  # SLO class of the event.
  sloClass: "slo_class"
  # Application, to which the event belongs.
  sloApp: "slo_app"
  # Unique identifier of the event.
  # This label holds value of Key attribute of the input SLO event
  eventKey: "event_key"
```

## Exposed metrics example
Given the default configuration as specified above, the resulting exposed metrics will be as follows:
```
slo_domain:slo_events_total{result=~"success|fail",slo_domain="__domain_name__"}
slo_domain_slo_class:slo_events_total{result=~"success|fail",slo_domain="__domain_name__",slo_class="__slo_class__"}
slo_domain_slo_class_slo_app:slo_events_total{result=~"success|fail",slo_domain="__domain_name__",slo_class="__slo_class__",slo_app="__slo_app__"}
slo_domain_slo_class_slo_app_event_key:slo_events_total{result=~"success|fail",slo_domain="__domain_name__",slo_class="__slo_class__",slo_app="__slo_app__",event_key="__event_key__"}
```

Each of the timeseries will have additional labels which are (optionally) specified in [sloEventProducer](./slo_event_producer.md) rules configuration (as `additional_metadata`) - for example slo_version, slo_type,... 
