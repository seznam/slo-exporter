# Kubernetes manifests

These serve mostly as an example. You will probably want to use kustomize or some other configuration/deployment tool.

## Workload type

We recommend using a StatefulSet instead of deployment to mitigate high churn in long term metrics.
Predictable pod name which in the instance label makes it easier for Prometheus to calculate the SLO over long time
periods.

## Configuration

We recommend to either build own docker image based on the upstream one with the configuration baked in or if using
config maps, use some kind of versioning for it to allow rolling upgrades etc.

In this example we use it without the versioning just for simplification.

