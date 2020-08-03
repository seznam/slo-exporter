FROM debian:stable-slim

COPY slo_exporter  /slo_exporter/
COPY Dockerfile /

WORKDIR /slo_exporter

ENTRYPOINT ["/slo_exporter/slo_exporter"]

CMD ["--help"]
