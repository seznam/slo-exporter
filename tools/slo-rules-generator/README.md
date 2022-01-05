slo-rules-generator is a tool which generates Prometheus recording rules based on input defintion of SLO domain. The generated rules are necessary for SLO based alerting - error budget exhaustion and burn-rate based alerts.

See [./slo-domains.yaml.example](./slo-domains.yaml.example) for commented example of input configuration.

## Usage

 1. Run `go version` to verify that you have `Go` installed - if not, refer to [golang website](https://golang.org/doc/install)
 1. Build `slo-rules-generator` by running `go build .`
 1. Run `slo-rules-generator` with `slo-domain.yaml.example` as an argument by running the following command:
    ```bash
    slo-rules-generator slo-domains.yaml.example
    ```
## Metrics in generated output
### slo:stable_version
- used in order to link given domain to specific team (label `team`)
- `enabled="true|false"` disable burn_rate, error budget alerts
- `escalate` label documents first escalation level for the given domain

Example:
```
slo:stable_version{enabled="true", escalate="sre-team@company.org", namespace="test", slo_domain="example-domain", slo_version="1", team="example-team@company.org"}
```
### slo:violation_ratio_threshold
- holds value of threshold for given `slo_version, slo_domain, slo_class, slo_type, namespace`
- additional labels simplify values visualization in Grafana for latency-related SLO types - `percentile`, `le` (same as for `le` in Prometheus histograms, documents latency threshold)

Example:
```
slo:violation_ratio_threshold{le="0.6", namespace="test", percentile="90", slo_class="critical", slo_domain="example-domain", slo_type="latency90", slo_version="1"}
	0.9
slo:violation_ratio_threshold{le="12.0", namespace="test", percentile="99", slo_class="critical", slo_domain="example-domain", slo_type="latency99", slo_version="1"}
	0.99
slo:violation_ratio_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_type="availability", slo_version="1"}
	0.9
```
### slo:burn_rate_threshold
- modifier for slo:burn_rate based alerts' threshold
- default values are usually reasonable, make sure you read chapters on SLO from [SRE workbook](https://sre.google/workbook/table-of-contents/) before even considering to change these

Example:
```
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="1d", slo_type="availability", slo_version="1"}
	2.8
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="1d", slo_type="latency90", slo_version="1"}
	2.8
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="1d", slo_type="latency99", slo_version="1"}
	2.8
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="1h", slo_type="availability", slo_version="1"}
	13.44
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="1h", slo_type="latency90", slo_version="1"}
	13.44
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="1h", slo_type="latency99", slo_version="1"}
	13.44
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="3d", slo_type="availability", slo_version="1"}
	1
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="3d", slo_type="latency90", slo_version="1"}
	1
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="3d", slo_type="latency99", slo_version="1"}
	1
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="6h", slo_type="availability", slo_version="1"}
	5.6
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="6h", slo_type="latency90", slo_version="1"}
	5.6
slo:burn_rate_threshold{namespace="test", slo_class="critical", slo_domain="example-domain", slo_time_range="6h", slo_type="latency99", slo_version="1"}
	5.6
```
