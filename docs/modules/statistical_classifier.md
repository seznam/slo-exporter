# Statistical classifier

|                |                         |
|----------------|-------------------------|
| `moduleName`   | `statisticalClassifier` |
| Module type    | `processor`             |
| Input event    | `raw`                   |
| Output event   | `raw`                   |

This module watches observes statistical distribution of all incoming already classified events.
This distribution is then used to classify incoming unclassified events.
It produces only classified events, if any error or issue is encountered, the event is dropped.

This module allows you to ensure no events will be dropped just because they were not classified.
Of course the precision is based on the previously observed data but it is still better than drop the events completely.

Applicable for example in the following cases:

 - Application usually sends its event identifier within HTTP headers. 
   In cases where communication is interrupted in a way that this header is not sent 
   (e.g. HTTP 5xx or 499 status codes), we have no way how to identify (and thus classify) the event.


`moduleConfig`
```yaml
# Time interval from which calculate the distribution used for classification.
historyWindowSize: "30m"
# How often the weights calculated over the historyWindowSize will be updated.
historyWeightUpdateInterval: "1m"
```

