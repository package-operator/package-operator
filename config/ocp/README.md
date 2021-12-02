# OCP APIs

## Openshift

- `cluster-version-operator_01_clusterversion.crd.yaml`
- `config-operator_01_proxy.crd.yaml`

From https://github.com/openshift/api/tree/master/config/v1

ClusterVersion API is required to lookup the cluster ID, when reporting to upgrade policies endpoints.
And the Proxy API needs to be present for OLM, as OLM thinks it is running on OpenShift.

## Prometheus-Operator

- `monitoring.coreos.com_servicemonitors.yaml`

From https://raw.githubusercontent.com/openshift/prometheus-operator/release-4.8/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml

ServiceMonitors from the Monitoring API are required to manage monitoring federation.
