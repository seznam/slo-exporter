# Relabel

|                |              |
|----------------|--------------|
| `moduleName`   | `relabel` |
| Module type    | `processor`  |
| Input event    | `raw`        |
| Output event   | `raw`        |

This module allows you to modify the event metadata or drop the event at all.
It uses native Prometheus `relabel_config` syntax. In this case metadata is referred as labels.
See [the upstream documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config)
for more info.


`moduleConfig`
```yaml
eventRelabelConfigs:
    - <relabel_config>
```

You can find some [examples here](/examples)
