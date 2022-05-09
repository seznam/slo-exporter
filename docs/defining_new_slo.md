# Defining new SLO

> In case you don't know what SLO is we recommend you to read [the SRE workbook](https://sre.google/workbook/implementing-slos/).
> Shortcuts used:
>   - **Service Level Indicator(SLI)**: Measurable phenomenon that describes some crucial characteristic of the observed service(ideally in %).
>   - **Service Level Objective(SLO)**: Desired level of quality for given SLI.
>   - **Service Level Agreement(SLA)**: Agreement between service provider and user about what happens if the SLO is not matched.
>   - **Error budget(EB)**: Acceptable amount of errors over a specified time window based on the SLO.

Each service having any consumers should have defines SLO.
For example if your team manages set of microservices,
most probably you would set and measure SLO for any interaction outside your team components(other team components, end users).
Keep in mind that service cannot have higher SLO than lowest of services it depends on.

# 1. Defining SLI
Do a brainstorming for both developers and product owners to
think of the important functionality and characteristics of your service
and what matters to your customers. These will be your SLIs. 
You should be able to represent those as a % of success or failure for the further computation.
> Hint: Do not try to overcomplicate things right from the beginning and stop it from actually computing at least some SLIs.
>       You can iterate on the accuracy later on.

## SLI computation approaches
There are 2 main approaches to measure the SLI

### Time based
This SLI would be based on some time interval and percentage of the time the service meets the criteria.
Most common use-case would be an uptime computed as percentage of the time the service is operational(it might be tricky to tell what functional means).

##### Advantages
- Easy to calculate
##### Disadvantages
- Does not reflect if the service is used at all (overnight for example)
- Does not reflect the impact of an outage (if only one or thousands of users were affected by an outage)

### Event based
This SLI would be computed per event of the observed phenomenon.
Most common use-case would be observing each HTTP request result or result of some async job triggered by the user.

##### Advantages
- Actually reflects impact of the outage on users (if an outage happens but no one uses the service no one cares)
- Reflects the amount of the outage impact (outage in prime time affects the EB more than in low traffic periods of a day)
##### Disadvantages
- Can be harder to compute and filter
    - Large number of events to aggregate
    - Some events may not be relevant to user experience (external monitoring HTTP request probes for example)
- May be hard to alert on, in case of low frequency phenomenons (if the event occurs twice a day, single failed event might cause 50% failure ratio on short time ranges)
- May lead to accounting repeated events of one user leading to multiplication of the impact (user refreshing the page to see if its operational).

## SLI types
These are the most common types of SLIs we encountered, of course you can think of any others that suit you but
if possible try to stick to these common ones for consistency.
They are based on the SRE workbook, and it makes it easier for others to understand what the SLI means.

For more inspiration see https://sre.google/workbook/implementing-slos/#slis-for-different-types-of-services

### Availability
#### Time based
Also, often referred to as an _uptime_, a ratio of the time when the service is up to the time it is down.
You should be careful what actually means the service _is down_.
Mostly suitable for some continuously running computations or in cases when the event based SLI is hard
or even impossible to evaluate reliably.

#### Event based
Probably most common SLI used mainly for web services computed as a ratio of failed/successful requests served by the service.
Should generally describe ratio of failed events to its total count. 
The Simplest example could be expressed as a PromQL `http_request_duration_seconds_count{service="foo", status_code=~"5.."} / ignoring(status_code) http_request_duration_seconds_count{service="foo"}`.

> REST API requests are quite easy to observe but dealing with for example GraphQL, XmlRPC, gRPC might be challenging
> since the result might be encoded in the request body or in GraphQL case one HTTP request might contain several responses
> thus it's hard to just binary say success or failure.

> Carefully decide what is the failure of your service and what is the client's fault.
> It always depends on the service but a lot of 4xx HTTP statuses might be disputable.

### Latency
Similar to the availability SLI type, but instead of a success or failure we measure a latency of the event.
Sticking to the example of REST API we would observe how long it takes to serve the request.

Latency SLI is observed in the form of percentiles such as "95% of events are served within 1.2s".

> Keep in mind that a failed event might take way less time than a successful one. So you should consider if the failed events should be counted into your latency SLI or not.
> Also be careful for example on user side timeouts. It might be observed in both, the latency and availability SLI type.

### Quality
SLI describing the quality of some events. 
For example if you return a search response to a client and your service is sharded and not all of your shards are available at the moment,
your response might be degraded in quality. This should be expressed again as a % so in this example % of shards that were (un)available.

### Freshness
Should detect if served data is not outdated. 
#### Time based
Ratio of time the data was outdated.
#### Event based
Ratio of events affected by the outdated events.

# 2. Defining SLO
Definition of the SLO should be done by the product owner of the service since he knows what is the desired quality.
In case you have no idea where to start we advise recommend using historical data to determine the current best effort SLO
and iterate on that to make it better.

## Choosing a time window
SLO should be measured over some longer time period. Most common would be a month meaning "We want to have a 98% availability over the last month".
This purely depends on the service nature and possibly on the users and what they expect.

## Documenting SLO
It's good practice to document how the SLO is measured in a human-readable form, so the product owner understands it.
Remember that each change in the SLO computation should be communicated, understood and updated there.

> Example:
> Our availability SLO is 99.5% of successful events over 4 weeks.
> Failed event is an HTTP request from the user resulting with status code 5xx.
> All requests resulting with 4xx status codes are not counted into the total amount of events since those are user-side issues.
> Also all requests made by our external monitoring are excluded from the whole SLO computation.


# 3. Implementing SLO
The SRE workbook contains examples how to implement SLO using Prometheus metrics.
Initially we went this path and in some cases it can be the easiest way to start and eventually sufficient.
But in some cases you can encounter issues that are hard to overcome and that was the reason we created the slo-exporter.
If you want to read more about it take a look at [this series of blog posts we wrote](https://medium.com/@sklik.devops/our-journey-towards-slo-based-alerting-bd8bbe23c1d6).

Ideally everyone in company should use the same approach how to compute or at least visualize SLOs. 
In case slo-exporter is too much of a complexity for your trivial SLO computation,
we advise you to at least try to stick to the metric naming used by slo-exporter and possibly using
at least its alerting rules and dashboards.

## Using Prometheus only
This can be easy to implement, and you can find number of upstream
projects that can help you with that such as [this SLO generator](https://promtools.dev/alerts/errors), 
[Sloth](https://sloth.dev/) or [the Pyrra project](https://github.com/pyrra-dev/pyrra) having even a nice visualization.

#### Advantages
- No need for application changes, you can use already existing Prometheus metrics you already collect
- No need for other infrastructure changes

#### Disadvantages
- Limited by cardinality because of the Prometheus architecture and purpose
  (if needed for filtering of events or having more precise thresholds for some events)
- A need to use histograms and its buckets to match the thresholds for latency and quality SLI types
- Computing SLO based on high cardinality metrics over 4w can be enormously demanding for the Prometheus cluster

## Using slo-exporter
Slo exporter was built to solve more complex SLO setups and to unify SLO computation.
It has support to compute SLO based on querying Prometheus data, tailing logs from file, receiving logs from Envoy proxy, following Kafka topic or reading logs from Elasticsearch.

### 1. Choose source of the events
#### Querying Prometheus
This mode is mostly just for unification of the SLO computation so everyone uses the same tooling.
It could be replaced by Prometheus recording rules and just hooking into the alerting by using the same metric naming schema as slo-exporter.
It also does not support HA setup, so it's not advised to use this and avoid it if possible.
Slo-exporter documentation can be found [here](https://github.com/seznam/slo-exporter/blob/master/docs/modules/prometheus_ingester.md) and complete example with whole configuration and kubernetes manifests
[here](https://github.com/seznam/slo-exporter/tree/master/examples/prometheus).

> Warning: in this setup, it is important to not duplicate observed events - for example by running slo-exporter in more instances. In is adviced to run slo-exporter with prometheus ingester module as a single global replica or to shard the queries so that each slo-exporter instance handles different subset of events. Once you decide what is going to be your deployment model, create alerts to cover this.

#### From Envoy access logs
If your service is accessed by users via Envoy proxy, you can configure it to send the access logs using gRPC to the slo-exporter.
Documentation about the slo-exporter gRPC logging module is [here](https://github.com/seznam/slo-exporter/blob/master/docs/modules/envoy_access_log_server.md),
documentation how to configure Envoy to send the access logs over gRPC can be found [here](https://www.envoyproxy.io/docs/envoy/latest/api-v3/data/accesslog/v3/accesslog.proto).
Complete example of the setup is [here](https://github.com/seznam/slo-exporter/tree/master/examples/envoy_proxy).

#### Elasticsearch
> Currently in experimental mode

In case you want to compute the SLO based on your application logs and use an ELK stack, you can hook the slo-exporter to it.

> Warning: in this setup, the slo-exporter cannot run in multiple replicas because it would lead to duplicating the observed events. Make sure to be alert on that.

#### Tailing file logs
Similar to those previous but requires the slo-exporter to be running as a sidecar to each observed service instance and tail its application logs.
slo-exporter documentation [here](https://github.com/seznam/slo-exporter/blob/master/docs/modules/tailer.md)
and a full example [here](https://github.com/seznam/slo-exporter/tree/master/examples/nginx_proxy).

#### Reading Kafka topic
In some cases if you want to avoid logging and need high cardinality data you can send each event to a Kafka topic and slo-exporter would read them from there.
Slo-exporter expects the messages to be in the particular format described [here](https://github.com/seznam/slo-exporter/blob/master/docs/modules/kafka_ingester.md).
Full example can be found [here](https://github.com/seznam/slo-exporter/tree/master/examples/kafka).

### 2. Configuring and deploying the slo-exporter
Based on the decisions made, configure the slo-exporter according to [the documentation](https://github.com/seznam/slo-exporter/blob/master/docs/configuration.md)
and deploy it for example in kubernetes. Example k8s manifests can be found in [../kubernetes](../kubernetes).

### 3. Setting up alerts
Slo-exporter comes with a set of prepared Prometheus recording rules and alerts to be used for the alerting. Those can be found [here](https://github.com/seznam/slo-exporter/tree/master/prometheus). Also you can use [the slo-rule-generator tool](https://github.com/seznam/slo-exporter/tree/master/tools/slo-rules-generator) to generate recording rules needed to activate each domain - those specify domain's ownership metadata, SLO thresholds, burn-rate alerts thresholds.

### 4. Setup Grafana dashboards
Last thing is to set up the Grafana dashboards to visualize your SLOs, those are also [in the slo-exporter repository](https://github.com/seznam/slo-exporter/tree/master/grafana_dashboards).



