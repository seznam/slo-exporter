# Event key generator

|                |                     |
|----------------|---------------------|
| `moduleName`   | `eventKeyGenerator` |
| Module type    | `processor`         |
| Input event    | `raw`               |
| Output event   | `raw`               |

This module allows you to generate an identifier of the event type.
It will join all values of specified event metadata keys (if found) using the separator
and use it as the new identifier.

`moduleConfig`
```yaml
# Separator to be used to join the selected metadata values.
filedSeparator: ":"
# If the event key should be overwritten if it's already set for the input event.
overrideExistingEventKey: true
# Keys which values will be joined as the resulting eventKey in specified order
metadataKeys:
  - <metadata_key>
```

