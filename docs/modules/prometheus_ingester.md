# Prometheus ingester


|                |                         |
|----------------|-------------------------|
| `moduleName`   | `prometheusIngester`    |
| Module type    | `producer`              |
| Output event   | `raw`                   |

# TODO

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
# How often to execute the query.
interval: <go_duration>
# Names of the labels that should be dropped from the result.
dropLabels:
  - <label_name>
# Labels and its values to be added to the results labels. Will overwrite conflicting labels.
additionalLabels:
  <label_name>: <label_value>
```
