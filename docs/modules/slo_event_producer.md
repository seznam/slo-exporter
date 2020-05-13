# SLO event producer

|                |                     |
|----------------|---------------------|
| `moduleName`   | `sloEventProducer`  |
| Module type    | `processor`         |
| Input event    | `raw`               |
| Output event   | `SLO`               |

The SLO is being calculated based on a ratio of successful and failed events.
The input events has some metadata, but we need to decide if it was a success of failure.

This module allows setting rules for evaluating the events.
Number of those rules can be higher, so they can be loaded from separate YAML files.

`moduleConfig` (in the root slo exporter config file)
```yaml
# Paths to files containing rules for evaluation of SLO event result and it's metadata.
rulesFiles:
  - <path>
```

Structure of the rules file referenced from the root config of slo exporter:

`rulesConfig`
```yaml
# List of rules to be evaluated on the input events.
rules:
  - <rule>
```

`rule`
```yaml
# Matcher of the events metadata. Rule will be applied only if all of them matches.
metadata_matcher:
  - <metadata_matcher_condition>
# Matcher of the SLO classification of the event. Rule will be applied only if matches the event SLO classification.
slo_matcher:
  domain: <value>
  class: <value>
  app: <value>
# Conditions to be checked on the matching event, if any of those results with true, the event is marked as failure, otherwise success.
failure_conditions:
  - <failure_condition>
# Additional metadata that will be exported in the SLO metrics.
additional_metadata:
  <value>: <value>
# If the event already contains information about it's result,
# it will be used and the failure conditions wont apply at all for this event.
#   if evaluated event's sloResult attribute is nonempty, use its content to determine the event's result (ignoring all failure_criteria)
#   if value equals event.Result's success value -> event is considered as successful
#   otherwise, event is considered as failed
#   if evaluated event's sloResult attribute is empty, failure_criteria are evaluated and events's result is set based on them.
honor_slo_result: False
```
*Please note that if multiple types of matchers are used in a rule, all of them has to match the given event.*

`metadata_matcher_condition`, `failure_condition`
```yaml
# Name of the operator to be evaluated on the value of the specified key.
operator: <operator_name>
key: <value>
value: <value>
# If set to True, the failure_condition will be exposed as a Prometheus metrics (named 'slo_exporter_slo_event_producer_slo_rules_threshold').
# - All metadata_matchers of the given slo rule which contain equality operator are added as labels to the resulting metric.
# - All failure conditions which evaluate against number (e.g. numberEqualTo, numberHigherThan) will result in single metric with operator name set in 'operator' label. At least one such failure condition has be found within the rule.
# - Additional_metadata are added as labels to resulting metric.
#
# Valid for `failure_conditions` only.
#
# See below for an example.
expose_as_metric: false
```

Supported operators:

| `operator_name`             | Expected string format        | Description |
|-----------------------------|-------------------------------|-------------|
| `equalTo      `             | Any string                    | Compares the string with the given value. Qualifies as an equality operator. |
| `matchesRegexp`             | Any string                    | Tries if value of the key matches the regexp form value. |
| `numberEqualTo`             | String parsable as float      | Converts the string to float if possible and checks if is equal to the value. Qualifies as an equality operator. |
| `numberHigherThan`          | String parsable as float      | Converts the string to float if possible and checks if is higher than the value. |
| `numberEqualOrHigherThan`   | String parsable as float      | Converts the string to float if possible and checks if is equal or higher than the value. |
| `numberEqualOrLessThan`     | String parsable as float      | Converts the string to float if possible and checks if is equal or less than the value. |
| `durationHigherThan`        | Staring in Go duration format | Converts the string to duration if possible and compares it to the duration from value. |

---

Example of the whole rules file:
```yaml
rules:
  - slo_matcher:
      domain: test-domain
    failure_conditions:
      - operator: numberHigherThan
        key: statusCode
        value: 499
    additional_metadata:
      slo_type: availability
      slo_version: 6
    honor_slo_result: True

  - metadata_matcher:
      - operator: matchesRegexp
        key: statusCode
        value: 200
    slo_matcher:
      domain: test-domain
      class: critical
    failure_conditions:
      - operator: numberHigherThan
        key: requestDuration
        value: 1
    additional_metadata:
      slo_version: 6
      slo_type: latency90
      percentile: 90
      le: 0.8
```

`exposing individual failure conditions as a Prometheus metric:`
```
  - metadata_matcher:
      - key: name
        operator: equalTo
        value: ad.advisual
      - key: cluster
        operator: matchesRegexp
        value: tt-k8s1.+
    failure_conditions:
      - key: prometheusQueryResult
        operator: numberHigherThan
        value: 6300
        expose_as_metric: true
    additional_metadata:
      slo_version: 6
      slo_type: freshness

  - metadata_matcher:
      - key: name
        operator: equalTo
        value: ad.banner
    failure_conditions:
      - key: prometheusQueryResult
        operator: numberHigherThan
        value: 6300
      - key: prometheusQueryResult
        operator: numberEqualOrLessThan
        value: 1
        expose_as_metric: true
    additional_metadata:
      slo_version: 6
      slo_type: freshness
      foo: bar
```



The configuration above will result in the following metrics:

```
# HELP slo_exporter_slo_event_producer_slo_rules_threshold Threshold exposed based on information from slo_event_producer's slo_rules configuration
# TYPE slo_exporter_slo_event_producer_slo_rules_threshold gauge
slo_exporter_slo_event_producer_slo_rules_threshold{foo="",name="ad.advisual",operator="numberHigherThan",slo_type="freshness",slo_version="6"} 6300
slo_exporter_slo_event_producer_slo_rules_threshold{foo="bar",name="ad.banner",operator="numberEqualOrLessThan",slo_type="freshness",slo_version="6"} 1
```
