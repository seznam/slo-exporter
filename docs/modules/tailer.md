# Tailer

|                |             |
|----------------|-------------|
| `moduleName`   | `tailer`    |
| Module type    | `producer`  |
| Output event   | `raw`       |

This module is able to tail file and parse each line using regular expression with named groups.
Those group names are used as metadata keys of the produces event and values are the matching strings.

It persists the last read position to file, so it can continue if restarted.

It can be used for example to tail proxy log and create events from it
so you can calculate SLO for your HTTP servers etc.

`moduleConfig`
```yaml
# Path to file to be processed.
tailedFile: "/logs/access_log"
# If tailed file should be followed for new lines once all current lines are processed.
follow: true
# If tailed file should be reopened.
reopen: true
# Path to file where to persist position of tailing.
positionFile: ""
# How often current position should be persisted to the position file.
positionPersistenceInterval: "2s"
# Defines RE which is used to parse the log line.
# Currently known named groups which are used to extract information for generated Events are:
#   sloDomain - part of SLO classification for the given event.
#   sloApp - part of SLO classification for the given event.
#   sloClass - part of SLO classification for the given event.
# All other named groups will be added to to the request event as event.Metadata.
loglineParseRegexp: '^(?P<ip>[A-Fa-f0-9.:]{4,50}) \S+ \S+ \[(?P<time>.*?)\] "(?P<request>.*?)" (?P<statusCode>\d+) \d+ "(?P<referer>.*?)" uag="(?P<userAgent>[^"]+)" "[^"]+" ua="[^"]+" rt="(?P<requestDuration>\d+(\.\d+)??)".+ignore-slo="(?P<ignoreSloHeader>[^"]*)" slo-domain="(?P<sloDomain>[^"]*)" slo-app="(?P<sloApp>[^"]*)" slo-class="(?P<sloClass>[^"]*)" slo-endpoint="(?P<sloEndpoint>[^"]*)" slo-result="(?P<sloResult>[^"]*)"'    # emptyGroupRE defines RE used to decide whether some of the RE match groups specified in loglineParseRegexp is empty and this its assigned variable should be kept unitialized
# Value, that will be treated as empty value.
emptyGroupRE: '^-$'
# eventIdMetadataKey its value will be used as a unique id for the generated event if present (hint: use a trace ID if possible).
eventIdMetadataKey: <string>
```

