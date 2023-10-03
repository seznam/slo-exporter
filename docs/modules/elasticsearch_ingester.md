# Elasticsearch ingester

|                |                         |
|----------------|-------------------------|
| `moduleName`   | `elasticSearchIngester` |
| Module type    | `producer`              |
| Output event   | `raw`                   |

This module allows you to real time follow all new documents using Elasticsearch query and compute SLO based on those.

Most common use case would be, if running in Kubernetes for example and already collecting logs using the ELK stack. You
can simply hook to those data and compute SLO based on those application logs.

### Elastic search versions and support

Currently, only v7 is supported.

### How does it work

The module periodically(interval is configurable) queries(you can pass in custom Lucene query)
the Elasticsearch index(needs to be specified) and for every hit creates a new event from the document. All the
documents needs to have a field with a timestamp(field name and format configurable), so the module can sort them and
store the last queried document timestamp. In next iteration it will use this timestamp as lower limit for the range
query, so it does not miss any entries. Each query is limited by maximum batch size(configurable) co the requests are
not huge.

In case you do not use structured logging and your logs are not indexed, you can specify name of the field with the raw
log entry and regular expression with named groups which, if matched, will be propagated to the event metadata.

### moduleConfig

```yaml
# OPTIONAL Debug logging
debug: false
# REQUIRED Version of the Elasticsearch API to be used, possible values: 7 
apiVersion: "v7"
# REQUIRED List of addresses pointing to the Elasticsearch API endpoint to query
addresses:
  - "https://foo.bar:4433"
# OPTIONAL Skip verification of the server certificate 
insecureSkipVerify: false
# OPTIONAL Timeout for the Elasticsearch API call
timeout: "5s"
# Enable/disable sniffing, autodiscovery of other nodes in Elasticsearch cluster
sniffing: true
# Enable/disable healtchecking of the Elasticsearch nodes 
healthchecks: true

# OPTIONAL username to use for authentication
username: "foo"
# OPTIONAL password to use for authentication
password: "bar"
# OPTIONAL Client certificate to be used for authentication
clientCertFile: "./client.pem"
# OPTIONAL Client certificate key to be used for authentication
clientKeyFile: "./client-key.pem"
# OPTIONAL Custom CA certificate to verify the server certificate
clientCaCertFile: "./ca.cert"

# OPTIONAL Interval how often to check for new documents from the Elasticsearch API.
# If the module was falling behind fo the amount of documents in the Elaseticsearch, it will
# query it more often.
interval: 5s
# REQUIRED Name of the index to be queried 
index: "my-index"
# OPTIONAL Additional Lucene query to filter the results 
query: "app_name: nginx AND namespace: test"
# OPTIONAL Maximum number of documents to be read in one batch
maxBatchSize: 100

# REQUIRED Document filed name containing a timestamp of the event
timestampField: "@timestamp"
# REQUIRED Golang time parse format to parse the timestamp from the timestampField
timestampFormat: "2006-01-02T15:04:05Z07:00" # See # https://www.geeksforgeeks.org/time-formatting-in-golang/ for common examples
# OPTIONAL Name of the field in document containing the raw log message you want to parse
rawLogField: "log"
# OPTIONAL Regular expression to be used to parse the raw log message,
# each matched named group will be propagated to the new event metadata
rawLogParseRegexp: '^(?P<ip>[A-Fa-f0-9.:]{4,50}) \S+ \S+ \[(?P<time>.*?)\] "(?P<httpMethod>[^\s]+)?\s+(?P<httpPath>[^\?\s]+).*'
# OPTIONAL If content of the named group match this regexp, it will be considered as an empty value.
rawLogEmptyGroupRegexp: '^-$'
```
