# Event metadata renamer

*Module status is _experimental_, it may be modified or removed even in non-major release.*

|                |                        |
|----------------|------------------------|
| `moduleName`   | `eventMetadataRenamer` |
| Module type    | `processor`            |
| Input event    | `raw`                  |
| Output event   | `raw`                  |

This module allows you to modify the event metadata by renaming its keys. Refusals of overriding an already existing _destination_ are reported as a Warning log as well as within exposed Prometheus' metric.

`moduleConfig`
```yaml
eventMetadataRenamerConfigs:
    - source: keyX
      destination: keyY
```
