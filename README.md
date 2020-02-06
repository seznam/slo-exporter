# SLO exporter

[![pipeline status](https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/badges/master/pipeline.svg)](https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/commits/master)
[![coverage report](https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/badges/master/coverage.svg)](https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/commits/master)

[Here](https://sklik-devops.gitlab.seznam.net/slo-exporter) you can examine which code is not covered by tests.


## Build
```bash
make build
```

## Testing on real-time production logs
Requires credentials to log in to szn-logy

Make sure to have `.env` file in the root of this repository in following format
```bash
SZN_LOGY_USER=xxx
SZN_LOGY_PASSWORD=xxx
```

Then just run
```bash
make compose
```

Address:
 - Prometheus scraping slo-exporter metrics: http://localhost:9090
 - slo-exporter address: http://localhost:8080/metrics

**[Use this link to see the graphs](http://localhost:9090/graph?g0.range_input=5m&g0.stacked=1&g0.expr=increase(slo_exporter_tailer_lines_read_total%5B10s%5D)&g0.tab=0&g1.range_input=5m&g1.stacked=1&g1.expr=sum(increase(slo_exporter_dynamic_classifier_events_processed_total%5B10s%5D))%20by%20(result%2C%20classified_by)&g1.tab=0&g2.range_input=1h&g2.expr=&g2.tab=1&g3.range_input=5m&g3.expr=histogram_quantile(0.99%2Crate(slo_exporter_dynamic_classifier_matcher_operation_duration_seconds_bucket%5B10s%5D))&g3.tab=0&g4.range_input=5m&g4.stacked=1&g4.expr=increase(slo_exporter_tailer_malformed_lines_total%5B10s%5D)&g4.tab=0&g5.range_input=5m&g5.stacked=1&g5.expr=histogram_quantile(0.99%2Crate(slo_exporter_slo_event_producer_evaluation_duration_seconds_bucket%5B10s%5D))&g5.tab=0&g6.range_input=5m&g6.stacked=1&g6.expr=increase(slo_exporter_slo_event_producer_events_not_matching_any_rule%5B10s%5D)&g6.tab=0&g7.range_input=15m&g7.stacked=0&g7.expr=slo_exporter_sqlwriter_batch_size&g7.tab=0&g8.range_input=2d&g8.expr=sum(timescale_slo_events_total)%20by%20(result)&g8.tab=0&g9.range_input=5m&g9.stacked=0&g9.expr=increase(slo_exporter_sqlwriter_batch_writes_total%5B10s%5D)&g9.tab=0&g10.range_input=15m&g10.expr=increase(slo_exporter_event_filter_filtered_events_total%5B10s%5D)&g10.tab=0)**


## Architecture diagram
Written in Go using the [pipeline pattern](https://blog.golang.org/pipelines)

```
                                                      static config
                                                        +-------+
                                                        |       |
                                                        |       |
                                                        |       +----------+
                                                        |       |          |
                                                        +-------+          |
                                                                         +-v------+
                                                                         | cache  |            +--------------+
                                                                         |        |            |              |
+--------------+                                                         +-^----+-+            | 70% critical |
| nginx log    |                                                           |    |              | 50% warning  |
| processor    +-------+ event                                             |    |              |              |
|              |       |                                                   |    |              +------+-------+
+--------------+       |                                                   |    |                     |
                       |                                                   |    |                     |         (classified)                 SLO
+--------------+       |   +--------+       +------------+    event    +---+----v---+   event   +-----v-------+    event     +-----------+   event    +----------------+
| envoy log    |       |   | event  |       | event      |             | dynamic    |           | statistical |              | SLO event |            | Prometheus     |
| receiver     +-----------> filter +-------+ normalizer +-------------> classifier +-----------> classifier  +--------------+ producer  +------------> SLO exporter   |
|              |       |   |        |       |            |             |            |           |             |              |           |            |                |
+--------------+       |   +--------+       +------------+             +------------+           +-------------+              +-----------+            +----------------+
                       |
+--------------+       |
| prometheus   |       |
| query        +-------+ event
| processor    |
+--------------+

```



### RequestEvent classification flow
flow:
1. Pri startu se nahraje do cache pocatecni stav ze staticke konfigurace
1. Do dynamickyho classifieru prichazi event:
   1. je klasifikovany? (ma vyplnene slo_ atributy)
      - ano: 
         1. zapise se do cache spolu s jeho normalized identifikatorem
      - ne: 
         1. dotaze se cache na exact match, matchuje?
            - ano: 
               1. prida eventu slo_ data a posle ho dal
            - ne: 
               1. zkusi match regularama, matchuje?
                  - ne: 
                     1. posle event dal beze zmeny
                  - ano: 
                     1. zapise nalezena slo_ data spolu se svym normalizovanym id do exact matchu
                     1. prida eventu slo_ data a posle ho dal
1. Do statistickeho classifieru prijde event
   1. je klasifikovany?
      - ano: 
         1. inkrementuje pro dane slo_ data statistiky
         1. posle ho dal
      - ne: 
        1. na zaklade statistik priradi slo_ data eventu
        1. inkrementuje metriky ze neco klasifikoval pro dany slo_ data
        1. posle ho dal
