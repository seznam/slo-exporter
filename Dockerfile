FROM debian:stable

COPY slo_exporter  /slo_exporter/
COPY Dockerfile /

WORKDIR /slo_exporter

ENTRYPOINT ["/slo_exporter/slo_exporter"]

CMD ["--help"]

ARG BUILD_DATE=unknown-date
ARG VERSION=unknown-version
ARG BUILD_TYPE=manual
ARG BUILD_HOSTNAME
ARG BUILD_JOB_NAME
ARG BUILD_NUMBER
ARG VCS_REF
LABEL maintainer="sklik.devops@firma.seznam.cz" \
      org.label-schema.schema-version="1.0.0-rc.1" \
      org.label-schema.vendor="Seznam, a.s." \
      org.label-schema.build-date=$BUILD_DATE \
      org.label-schema.build-type="$BUILD_TYPE" \
      org.label-schema.build-ci-job-name="$BUILD_JOB_NAME" \
      org.label-schema.build-ci-build-id="$BUILD_NUMBER" \
      org.label-schema.build-ci-host-name="$BUILD_HOSTNAME" \
      org.label-schema.version=$VERSION \
      org.label-schema.name="slo-exporter" \
      org.label-schema.description="SLO exporter" \
      org.label-schema.usage="https://github.com/seznam/slo-exporter" \
      org.label-schema.url="https://github.com/seznam/slo-exporter" \
      org.label-schema.vcs-url="git@github.com:seznam/slo-exporter.git" \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.docker.dockerfile="/Dockerfile" \
      org.label-schema.docker.cmd="docker run <image> --help"
