# Event filter

|                |               |
|----------------|---------------|
| `moduleName`   | `eventFilter` |
| Module type    | `processor`   |
| Input event    | `raw`         |
| Output event   | `raw`         |

This module allows you to drop events based on it's metadata.
This can be done using regular expressions, if any of them matches the event,
it won't be passed to next module and will be dropped.

It is useful for example to drop HTTP requests with status code `404`,
those you probably don't want to take into account in your SLO calsulations.

`moduleConfig`
```yaml
# Map of Go regular expression matching event metadata key to Go regular expression matching metadata value. Any event with metadata matching any of those will be dropped.
metadataFilter:
  "(?i)statusCode": "30[12]|40[045]|411"
  "(?i)userAgent": "(?i)(?:sentry|blackbox-exporter|kube-probe)"
```

