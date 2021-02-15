FROM debian:stable-slim

RUN apt-get update && apt-get install ca-certificates -y && apt-get clean

COPY slo_exporter  /slo_exporter/
COPY Dockerfile /

WORKDIR /slo_exporter

ENTRYPOINT ["/slo_exporter/slo_exporter"]

CMD ["--help"]
