# OCP APIs


- `cluster-version-operator_01_clusterversion.crd.yaml`
- `config-operator_01_proxy.crd.yaml`

From https://github.com/openshift/api/tree/master/config/v1
ClusterVersion API is required to lookup the cluster ID, when reporting to upgrade policies endpoints.
And the Proxy API needs to be present for OLM, as OLM thinks it is running on OpenShift.
