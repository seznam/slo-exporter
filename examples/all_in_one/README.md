# All-in-one example

### Overview
Use the provided [docker-compose](./docker-compose.yaml) to start the complete setup with 
Prometheus instance loaded with [example SLO recording rules](../../prometheus_rules),
and Grafana instance with loaded [SLO dashboards](../../grafana_dashboards).

Description of the whole setup follows:
- **Nginx configured with the following paths:**
  - `nginx:8080/`    -> `HTTP 200`, all ok
  - `nginx:8080/err` -> `HTTP 500`, availability violation
  - `nginx:8080/drop`-> `limit 1r/m`, latency violation
- **Slo-exporter configured to tail the Nginx's logs**
- **Prometheus**
  - configured to scrape the slo-exporter's metrics
  - loaded with necessary recording-rules for SLO computation
- **Grafana**
  - with Prometheus preconfigured as a datasource
  - loaded with [SLO dashboards](../../grafana_dashboards/)
- **Slo-event-generator**
  - an infinite loop accessing the Nginx instance to generate slo-events.

### How to run it
```
docker-compose up
```

To access Grafana and Prometheus:
```
# http://localhost:9090 Prometheus
# http://localhost:9000 Grafana
#  User: admin
#  Password: admin
```
