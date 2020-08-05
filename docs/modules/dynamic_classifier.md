# Dynamic classifier

|                |                     |
|----------------|---------------------|
| `moduleName`   | `dynamicClassifier` |
| Module type    | `processor`         |
| Input event    | `raw`               |
| Output event   | `raw`               |

The SLO calculation is based on some domains and classes which group together
events by their functionality but also priority or demands on their quality.

This is called classification and for the SLO calculation you need to assign those events
to their domains and classes. These information how to classify them
can be specified using CSV files or they can come along with the event.

This module checks if the incoming event isn't already classified and if it isn't, it checks
the CSV file specifications if they can classify the event and adds the classification if possible.

The motivation behind this is that application itself can have the classification defined in it's code.
Then it just passes it along with the event (HTTP request in headers for example) and there is no need
to have the classification held centrally somewhere.

There is one issue, for example when generating SLO events from proxy log which proxies traffic to web
server sending those classification along. If the application stops working, it won't send the
classification, so we wouldn't know how to classify it. To mitigate this issue this module also
caches all the classifications of input events which are already classified.
This way it can classify the events even if the application goes down if they were called before.

Also, this cache can be initialized with defined values on startup, so that we can correctly classify events even for application which does not provide us with the classification by themselves.


#### `moduleConfig`
```yaml
# Paths to CSV files containing exact match classification rules.
exactMatchesCsvFiles: []
# Paths to CSV files containing regexp match classification rules.
regexpMatchesCsvFiles:
  - "conf/userportal.csv"
# Metadata key names of the event which will be added to the `events_processed_total` metric if the event cannot be classified.
# Name of the resulting label will be converted to snake case and prefixed with `metadata_`
unclassifiedEventMetadataKeys:
  - "userAgent"
```

##### Example of the CSV with exact classification:
```csv
test-domain,test-app,test-class,"GET:/testing-endpoint"
```

##### Example of the CSV with regexp classification:
```csv
test-domain,test-app,test-class,"/api/test/.*"
test-domain,test-app,test-class-all,"/api/.*"
```

##### CSV comments
CSV configuration files support single line comments. Comment has to start with the `#` character with no leading whitespaces.
Example:
```csv
# Example of comment
test-domain,test-app,test-class,"/api/test/.*"
```


