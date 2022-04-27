# SLO exporter
[![CircleCI](https://img.shields.io/circleci/build/github/seznam/slo-exporter/master) ](https://app.circleci.com/pipelines/github/seznam/slo-exporter?branch=master)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/seznam/slo-exporter) ](https://github.com/seznam/slo-exporter/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/seznam/slo-exporter) ](https://hub.docker.com/repository/docker/seznam/slo-exporter/general)
[![GitHub All Releases](https://img.shields.io/github/downloads/seznam/slo-exporter/total?label=release%20binary%20downloads) ](https://github.com/seznam/slo-exporter/releases/latest)

Tool [slo-exporter](https://github.com/seznam/slo-exporter/blob/master/README.md) computes standardized Service Level Indicator (SLI) and Service Level Objectives (SLO) metrics
based on events coming from various data sources. It follows principles from
[the SRE Workbook](https://landing.google.com/sre/workbook).
With already prepared [examples](examples), [Grafana dashboards](grafana_dashboards/README.md), [Prometheus recording rules and alerts](prometheus/), you can easily
start to alert on SLO in your infrastructure.

**If you want to start with computing SLOs, make sure to check out [this guide](./docs/defining_new_slo.md)!**

## Motivation
After more than year of experience of maintaining SLO alerting based on application metrics
just from Prometheus, number of issues showed up which made it very difficult and unbearable.
Few among others:
 - High cardinality of metrics if we wanted to easily find out which event type caused the alert or affected the error budget.
 - Classification of events ending up as huge regular expressions in the PromQLs.
 - Issues with default values for the computation if no events happened.
 - Need to filter out some events based on high cardinality metadata which cannot be added to metrics.

 This lead us to decision that we need to process the events separately and in
 Prometheus do just the final computation and alerting.

 We describe our journey towards SLO based alerting more in detail in the two articles:
 - [Implementing SRE workbook alerting with Prometheus only](https://medium.com/@sklik.devops/our-journey-towards-slo-based-alerting-bd8bbe23c1d6)
 - [Advanced SLO infrastructure based on slo-exporter](https://medium.com/@sklik.devops/our-journey-towards-slo-based-alerting-d23d4f6f620e)

## How it works
Every ingested event has metadata which are used to classify it to specific SLO domain and class
as described in [the SRE Workbook chapter `Alerting on SLOs`](https://landing.google.com/sre/workbook/chapters/alerting-on-slos/).
Additionally, name of the application where the event happened and identifier of the event is also added to ease the debugging of possible SLO violation.
Finally, you decide based on the metadata if the event was a successful or failed one.
Slo-exporter then exposes Prometheus metric `slo_domain_slo_class:slo_events_total{slo_domain="...", slo_class="...", result="..."}`.
This gives you number of successful or failed events which is all you need to calculate the error budget, burn rate etc.

## Installing
#### Build
In the root of the repository run
```bash
make
```

#### Docker
Prebuilt docker images can be found at [Docker Hub](https://hub.docker.com/repository/docker/seznam/slo-exporter).
```
docker run -it seznam/slo-exporter:<version> --help
```


#### Prebuilt binaries
See the [the latest release page](https://github.com/seznam/slo-exporter/releases) for the prebuilt binaries.


## Configuration and examples
Detailed configuration documentation you can find here [docs/configuration](docs/configuration.md).

To see some real use-cases and examples you can look at the [examples/](examples).

## Operating
Some advices on operating the slo-exporter, debugging and profiling can be found here [docs/operating.md](docs/operating.md).

Please note that features marked as *Experimental* are not considered stable and may be removed or changed even in [minor or patch release](https://semver.org/).

## Community
* Slack: [#slo-exporter](https://join.slack.com/t/slo-exporter/shared_invite/zt-mnqxqv1s-1zaJtDiYbuVoOCCAMQi4Kg)
* User mailing list: [slo-exporter](https://groups.google.com/g/slo-exporter)
* Issue Tracker: [GitHub Issues](https://github.com/seznam/slo-exporter/issues)
