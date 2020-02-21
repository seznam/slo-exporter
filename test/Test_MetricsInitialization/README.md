# TestMetricsInitialization

- Test whether all expected metrics have been properly initialized on single log line.
For all of the aggregated metrics, we check that both possible results values have been exposed and that `le` is filled according to the domain configuration file.

- There is also single log line which is supposed to be filtered based on provided status code. We test that by checking the total number of read lines.
- The other single log line which gets processed hits the configured normalizer rule, so that endpoint name is transformed as configured. 
