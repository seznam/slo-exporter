## Glossary
Here we describe some of the terms used through the repository. We assume that you have read chapters on SLO from Google's [SRE book](https://landing.google.com/sre/sre-book/toc/) and [SRE workbook](https://landing.google.com/sre/workbook/toc/), so the main focus here is to describe

### locality, namespace
We use this labels internally to differentiate between individual K8S clusters (`locality`) and K8S namespaces (`namespace`).

### slo-domain
This label groups slo-types and slo-classes into single entity which shares the same error budget policy and stakeholders. SLO domain usually contains multiple error budgets (equal to no. of slo-types * number of slo-classes for individual slo-types).

### slo-type
Differentiates individual SLIs - e.g. freshness, availability, etc. Some of the SLIs may be represented by multiple slo-types, multiple percentiles for latency SLI as slo-types latency90, latency99 as an example.

### slo-class
Label which enable to group events from the same slo-domain and slo-type. It may serve multiple purposes, e.g. to
- group events to the same classes of importance
- group events which share the same SLO thresholds
- group events with similar frequency of occurrence

### event_key
The last level of SLO event's grouping. Its content depends on desired level of SLO drilldown accuracy. It may contain name of RPC method, or normalized path of HTTP request together with HTTP method (e.g. `GET:/campaigns/list`). See [architecture](./architecture.md) for details on SLO event's structure.

### Error budget policy
A formal document which specifies actions which are to be triggered based on current state of error budget. Stopping all rollouts and shifting developers' focus on service's stability when error budget is depleted is the most common example. See [example error budget policy as published by Google](https://landing.google.com/sre/workbook/chapters/error-budget-policy/)