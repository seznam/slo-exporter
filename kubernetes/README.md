# Kubernetes manifests

These serve as an example. You will probably want to use kustomize or some other configuration/deployment tool.

## Workload type

We recommend using a StatefulSet instead of deployment to mitigate high churn in long term metrics - statefulset' stable pod name format (which is propagated to Prometheus' instance label) makes it easier for Prometheus to calculate the SLO over long time periods.

## Configuration

We recommend to either building own docker image based on the upstream one with the configuration baked in or including configuration as a versioned configmap(s), in order to simplify rollbacks.

In this example we use it without the versioning just for simplification.

