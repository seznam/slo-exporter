# Prometheus ingester


|                |                         |
|----------------|-------------------------|
| `moduleName`   | `prometheusIngester`    |
| Module type    | `producer`              |
| Output event   | `raw`                   |

Prometheus ingester generates events based on results of provided Prometheus queries.
For its usage example see [the prometheus example](../../examples/prometheus).

`moduleConfig`
```yaml
# URL of the API for Prometheus queries such as https://foo.bar:9090
apiUrl: <URL>
# Timeout for executing the query
queryTimeout: <Go_duration>
# List of queries to be periodically executed.
queries:
  - <query>
```

`query`
```yaml
# PromQL that should be executed
query: 'time() - last_successful_run_timestamp - on(app) group_left() min(alerting_threshold:last_successful_run_timestamp) by (app) > 0'
type: '<query_type>'
# resultAsQuantity determines whether result of the query should be used to set Quantity attribute of the new Event. If 'false', Quantity will be set to 1.
# default value is based on query type: 'counter_increase' and 'histogram_increase' defaults to true, 'simple' default to false
resultAsQuantity: false
# How often to execute the query.
interval: <go_duration>
# Names of the labels that should be dropped from the result.
dropLabels:
  - <label_name>
# Labels and its values to be added to the results labels. Will overwrite conflicting labels.
additionalLabels:
  <label_name>: <label_value>
```

Currently, we recognize two kinds of `<query_type>`:
#### type: `simple`
Supported results for this type of query: Matrix, Scalar, Vector. [See Prometheus API documentation on details about these](https://prometheus.io/docs/prometheus/latest/querying/api/)
Mapping to resulting event(s): For every metric and every of its returned samples, a new event is created with the following values:
```
event.Metadata["prometheusQueryResult"] - sample value
event.Metadata["unixTimestamp"] - sample timestamp
```
The rest of the Metadata map contains values of the returned metrics, while taking into account the `dropLabels` and `additionalLabels` configuration.

Example of intended use case(s):
- keeping track of ratio between two time series and generating slo events based on a certain threshold.
- generating slo events based on metric which record result of last batch job run, or its timestamp (e.g. `last_successful_run_timestamp`)

```
  - query: "max(barrel_creation_time{app="contextserver-slave", cluster="tt-k8s1.ko", name!=""}) by (app, hostname, name)"
    interval: 20s
    type: "simple"
```

#### type: `counter_increase`
Supported result for this type of query: Matrix. [See Prometheus API documentation on details about these](https://prometheus.io/docs/prometheus/latest/querying/api/)
Please note that the query is to be provided exactly as it would be passed to increase function, thus the following requirements applies:
- **no range selector is allowed (e.g. `[5m]`)**. Range selector is automatically added by prometheus_ingester - for the first query it equals to configured `query.interval`, for all other, it equals to rounded difference of current timestamp and timestamp of the last sample (this should roughly to equal to `query.interval` as well).

- **no sum()**. The query should result to matrix - list of time series. Do not apply sum function on the configured query for the same [reason why this is not a good idea to perform `increase(sum(...))`](https://www.robustperception.io/rate-then-sum-never-sum-then-rate).

Mapping to resulting event(s):
For every metric in the result:
* If no previous sample for given metric is found, no event is generated and sample is just stored to local cache (please note that cache is not persisted upon restarts and is local to every individual instance of slo-exporter).
* If previous sample of given metric is found, difference between new sample and previous sample is computed, and a single new event is generated with the following mapping:
```
event.Metadata["unixTimestamp"] - sample timestamp
event.Metadata["prometheusQueryResult"] - increase against the last seen value
```
The rest of the Metadata map contains values of the returned metrics, while taking into account the `dropLabels` and `additionalLabels` configuration.

Example of intended use case(s):
- generating SLO events based on metric of type `counter`

```
  - query: "job_duration_seconds_count{namespace=~'production', app='export-manager'}"
    interval: 20s
    type: "counter_increase"
    dropLabels:
      - instance
```

#### type: `histogram_increase`
Supported result for this type of query: Matrix. [See Prometheus API documentation on details about these](https://prometheus.io/docs/prometheus/latest/querying/api/)
This query type is special case of the previous `counter_increase`. It uses the prometheus `histogram` metric
which is composed by a set of counters which serves to observe distribution of some event observed value.
Prometheus ingester then generates events with values of max and min possible value based on the `le`
bucket distribution of the queried histogram.

```
  - query: "request_duration_seconds_bucket{app='export-manager'}"
    interval: 20s
    type: "histogram_increase"
    dropLabels:
      - instance
```


### Terminology used
* **Metric** - unique set of consisting of metric name and its labels. Associated with list of **Samples** forms a **Time series**.
* **Sample** - a value and timestamp pair. Associated to single Metric.

https://prometheus.io/docs/concepts/data_model/
