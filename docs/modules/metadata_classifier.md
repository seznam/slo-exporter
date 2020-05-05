# Event key generator

|                |                              |
|----------------|------------------------------|
| `moduleName`   | `metadataClassifier` |
| Module type    | `processor`                  |
| Input event    | `raw`                        |
| Output event   | `raw`                        |

This module allows you to classify an event using its metadata.
Specify keys which values will be used as according slo classification items.
If the key cannot be found, original value of classification will be left intact.
By default, the module will override event classification. 
This can be disabled to classify it only if it wasn't classified before.

`moduleConfig`
```yaml
# Key of metadata value to be used as classification slo domain.
sloDomainMetadataKey: <metadata_key>
# Key of metadata value to be used as classification slo domain.
sloClassMetadataKey: <metadata_key>
# Key of metadata value to be used as classification slo domain.
sloAppMetadataKey: <metadata_key>
# If classification of already classified event should be overwritten. 
overrideExistingValues: true
```

