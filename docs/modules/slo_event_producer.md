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
  - <condition>
# Matcher of the SLO classification of the event. Rule will be applied only if matches the event SLO classification.
slo_matcher:
  domain: <value>
  class: <value>
  app: <value>
# Conditions to be checked on the matching event, if any of those results with true, the event is marked as failure, otherwise success.
failure_conditions:
  - <condition>
# Additional metadata that will be exported in the SLO metrics.
additional_metadata:
  <value>: <value>
# If the event already contains information about it's result,
# it will be used and the failure conditions wont apply at all for this event.
#   if evaluated event's sloResult attribute is nonempty, use its content to determine the event's result (ignoring all failure_criteria)
#   if value equals event.Result's success value -> event is considered as successful
#   otherwise, event is considered as failed
#   if evaluated event's sloResult attribute is empty, failure_criteria are evaluated and events's result is set based on them.
honor_slo_result: True
```
*Please note that if multiple types of matchers are used in a rule, all of them has to match the given event.*

`condition`
```yaml
# Name of the operator to be evaluated on the value of the specified key.
operator: <operator_name>
key: <value>
value: <value>
```

Supported operators:

| `operator_name`      | Expected string format        | Description |
|----------------------|-------------------------------|-------------|
| `matchesRegexp`      | Any string                    | Tries if value of the key matches the regexp form value. |
| `numberHigherThan`   | String parsable as float      | Converts the string to float if possible and checks if is higher than the value. |
| `durationHigherThan` | Staring in Go duration format | Converts the string to duration if possible and cmpares it to the duration form value. |

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
