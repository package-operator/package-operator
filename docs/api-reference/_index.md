---
title: API Reference
weight: 50
---

# Addon Operator API Reference

The Addon Operator APIs are an extension of the [Kubernetes API](https://kubernetes.io/docs/reference/using-api/api-overview/) using `CustomResourceDefinitions`.

## `addons.managed.openshift.io`

The `addons.managed.openshift.io` API group in managed OpenShift contains all Addon related API objects.

* [AddonInstance](#addoninstanceaddonsmanagedopenshiftiov1alpha1)
	* [AddonInstanceSpec](#addoninstancespecaddonsmanagedopenshiftiov1alpha1)
	* [AddonInstanceStatus](#addoninstancestatusaddonsmanagedopenshiftiov1alpha1)
* [AddonOperator](#addonoperatoraddonsmanagedopenshiftiov1alpha1)
	* [AddonOperatorOCM](#addonoperatorocmaddonsmanagedopenshiftiov1alpha1)
	* [AddonOperatorSpec](#addonoperatorspecaddonsmanagedopenshiftiov1alpha1)
	* [AddonOperatorStatus](#addonoperatorstatusaddonsmanagedopenshiftiov1alpha1)
* [Addon](#addonaddonsmanagedopenshiftiov1alpha1)
	* [AddonInstallOLMAllNamespaces](#addoninstallolmallnamespacesaddonsmanagedopenshiftiov1alpha1)
	* [AddonInstallOLMCommon](#addoninstallolmcommonaddonsmanagedopenshiftiov1alpha1)
	* [AddonInstallOLMOwnNamespace](#addoninstallolmownnamespaceaddonsmanagedopenshiftiov1alpha1)
	* [AddonInstallSpec](#addoninstallspecaddonsmanagedopenshiftiov1alpha1)
	* [AddonNamespace](#addonnamespaceaddonsmanagedopenshiftiov1alpha1)
	* [AddonSpec](#addonspecaddonsmanagedopenshiftiov1alpha1)
	* [AddonStatus](#addonstatusaddonsmanagedopenshiftiov1alpha1)
	* [AddonUpgradePolicy](#addonupgradepolicyaddonsmanagedopenshiftiov1alpha1)
	* [AddonUpgradePolicyStatus](#addonupgradepolicystatusaddonsmanagedopenshiftiov1alpha1)
	* [EnvObject](#envobjectaddonsmanagedopenshiftiov1alpha1)
	* [MonitoringFederationSpec](#monitoringfederationspecaddonsmanagedopenshiftiov1alpha1)
	* [MonitoringSpec](#monitoringspecaddonsmanagedopenshiftiov1alpha1)
	* [SubscriptionConfig](#subscriptionconfigaddonsmanagedopenshiftiov1alpha1)
	* [ClusterSecretReference](#clustersecretreferenceaddonsmanagedopenshiftiov1alpha1)

### AddonInstance.addons.managed.openshift.io/v1alpha1

AddonInstance is a managed service facing interface to get configuration and report status back.

**Example**
```yaml
apiVersion: addons.managed.openshift.io/v1alpha1
kind: AddonInstance
metadata:
  name: addon-instance
  namespace: my-addon-namespace
spec:
  heartbeatUpdatePeriod: 30s
status:
  lastHeartbeatTime: 2021-10-11T08:14:50Z
  conditions:
  - type: addons.managed.openshift.io/Healthy
    status: "True"
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta) | false |
| spec |  | [AddonInstanceSpec.addons.managed.openshift.io/v1alpha1](#addoninstancespecaddonsmanagedopenshiftiov1alpha1) | false |
| status |  | [AddonInstanceStatus.addons.managed.openshift.io/v1alpha1](#addoninstancestatusaddonsmanagedopenshiftiov1alpha1) | false |

[Back to Group]()

### AddonInstanceSpec.addons.managed.openshift.io/v1alpha1

AddonInstanceSpec defines the configuration to consider while taking AddonInstance-related decisions such as HeartbeatTimeouts

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| heartbeatUpdatePeriod | The periodic rate at which heartbeats are expected to be received by the AddonInstance object | metav1.Duration | false |

[Back to Group]()

### AddonInstanceStatus.addons.managed.openshift.io/v1alpha1

AddonInstanceStatus defines the observed state of Addon

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| observedGeneration | The most recent generation observed by the controller. | int64 | false |
| conditions | Conditions is a list of status conditions ths object is in. | []metav1.Condition | false |
| lastHeartbeatTime | Timestamp of the last reported status check | metav1.Time | true |

[Back to Group]()

### AddonOperator.addons.managed.openshift.io/v1alpha1

AddonOperator is the Schema for the AddonOperator API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta) | false |
| spec |  | [AddonOperatorSpec.addons.managed.openshift.io/v1alpha1](#addonoperatorspecaddonsmanagedopenshiftiov1alpha1) | false |
| status |  | [AddonOperatorStatus.addons.managed.openshift.io/v1alpha1](#addonoperatorstatusaddonsmanagedopenshiftiov1alpha1) | false |

[Back to Group]()

### AddonOperatorOCM.addons.managed.openshift.io/v1alpha1

OCM specific configuration.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| endpoint | Root of the OCM API Endpoint. | string | true |
| secret | Secret to authenticate to the OCM API Endpoint. Only supports secrets of type "kubernetes.io/dockerconfigjson" https://kubernetes.io/docs/concepts/configuration/secret/#secret-types | [ClusterSecretReference.addons.managed.openshift.io/v1alpha1](#clustersecretreferenceaddonsmanagedopenshiftiov1alpha1) | true |

[Back to Group]()

### AddonOperatorSpec.addons.managed.openshift.io/v1alpha1

AddonOperatorSpec defines the desired state of Addon operator.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| pause | Pause reconciliation on all Addons in the cluster when set to True | bool | true |
| ocm | OCM specific configuration. Setting this subconfig will enable deeper OCM integration. e.g. push status reporting, etc. | *[AddonOperatorOCM.addons.managed.openshift.io/v1alpha1](#addonoperatorocmaddonsmanagedopenshiftiov1alpha1) | false |

[Back to Group]()

### AddonOperatorStatus.addons.managed.openshift.io/v1alpha1

AddonOperatorStatus defines the observed state of Addon

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| observedGeneration | The most recent generation observed by the controller. | int64 | false |
| conditions | Conditions is a list of status conditions ths object is in. | []metav1.Condition | false |
| lastHeartbeatTime | Timestamp of the last reported status check | metav1.Time | true |
| phase | DEPRECATED: This field is not part of any API contract it will go away as soon as kubectl can print conditions! Human readable status - please use .Conditions from code | AddonPhase.addons.managed.openshift.io/v1alpha1 | false |

[Back to Group]()

### Addon.addons.managed.openshift.io/v1alpha1

Addon is the Schema for the Addons API

**Example**
```yaml
apiVersion: addons.managed.openshift.io/v1alpha1
kind: Addon
metadata:
  name: reference-addon
spec:
  displayName: An amazing example addon!
  namespaces:
  - name: reference-addon
  install:
    type: OLMOwnNamespace
    olmOwnNamespace:
      namespace: reference-addon
      packageName: reference-addon
      channel: alpha
      catalogSourceImage: quay.io/osd-addons/reference-addon-index@sha256:58cb1c4478a150dc44e6c179d709726516d84db46e4e130a5227d8b76456b5bd
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta) | false |
| spec |  | [AddonSpec.addons.managed.openshift.io/v1alpha1](#addonspecaddonsmanagedopenshiftiov1alpha1) | false |
| status |  | [AddonStatus.addons.managed.openshift.io/v1alpha1](#addonstatusaddonsmanagedopenshiftiov1alpha1) | false |

[Back to Group]()

### AddonInstallOLMAllNamespaces.addons.managed.openshift.io/v1alpha1

AllNamespaces specific Addon installation parameters.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |

[Back to Group]()

### AddonInstallOLMCommon.addons.managed.openshift.io/v1alpha1

Common Addon installation parameters.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| namespace | Namespace to install the Addon into. | string | true |
| catalogSourceImage | Defines the CatalogSource image. | string | true |
| channel | Channel for the Subscription object. | string | true |
| packageName | Name of the package to install via OLM. OLM will resove this package name to install the matching bundle. | string | true |
| pullSecretName | Reference to a secret of type kubernetes.io/dockercfg or kubernetes.io/dockerconfigjson in the addon operators installation namespace. The secret referenced here, will be made available to the addon in the addon installation namespace, as addon-pullsecret prior to installing the addon itself. | string | false |
| config | Configs to be passed to subscription OLM object | *[SubscriptionConfig.addons.managed.openshift.io/v1alpha1](#subscriptionconfigaddonsmanagedopenshiftiov1alpha1) | false |

[Back to Group]()

### AddonInstallOLMOwnNamespace.addons.managed.openshift.io/v1alpha1

OwnNamespace specific Addon installation parameters.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |

[Back to Group]()

### AddonInstallSpec.addons.managed.openshift.io/v1alpha1

AddonInstallSpec defines the desired Addon installation type.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| type | Type of installation. | AddonInstallType.addons.managed.openshift.io/v1alpha1 | true |
| olmAllNamespaces | OLMAllNamespaces config parameters. Present only if Type = OLMAllNamespaces. | *[AddonInstallOLMAllNamespaces.addons.managed.openshift.io/v1alpha1](#addoninstallolmallnamespacesaddonsmanagedopenshiftiov1alpha1) | false |
| olmOwnNamespace | OLMOwnNamespace config parameters. Present only if Type = OLMOwnNamespace. | *[AddonInstallOLMOwnNamespace.addons.managed.openshift.io/v1alpha1](#addoninstallolmownnamespaceaddonsmanagedopenshiftiov1alpha1) | false |

[Back to Group]()

### AddonNamespace.addons.managed.openshift.io/v1alpha1



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name of the KubernetesNamespace. | string | true |

[Back to Group]()

### AddonSpec.addons.managed.openshift.io/v1alpha1

AddonSpec defines the desired state of Addon.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| displayName | Human readable name for this addon. | string | true |
| version | Version of the Addon to deploy. Used for reporting via status and metrics. | string | false |
| pause | Pause reconciliation of Addon when set to True | bool | true |
| namespaces | Defines a list of Kubernetes Namespaces that belong to this Addon. Namespaces listed here will be created prior to installation of the Addon and will be removed from the cluster when the Addon is deleted. Collisions with existing Namespaces are NOT allowed. | [][AddonNamespace.addons.managed.openshift.io/v1alpha1](#addonnamespaceaddonsmanagedopenshiftiov1alpha1) | false |
| install | Defines how an Addon is installed. This field is immutable. | [AddonInstallSpec.addons.managed.openshift.io/v1alpha1](#addoninstallspecaddonsmanagedopenshiftiov1alpha1) | true |
| resourceAdoptionStrategy | ResourceAdoptionStrategy coordinates resource adoption for an Addon Originally introduced for coordinating fleetwide migration on OSD with pre-existing OLM objects. NOTE: This field is for internal usage only and not to be modified by the user. | ResourceAdoptionStrategyType.addons.managed.openshift.io/v1alpha1 | false |
| upgradePolicy | UpgradePolicy enables status reporting via upgrade policies. | *[AddonUpgradePolicy.addons.managed.openshift.io/v1alpha1](#addonupgradepolicyaddonsmanagedopenshiftiov1alpha1) | false |
| monitoring | Defines how an addon is monitored. | *[MonitoringSpec.addons.managed.openshift.io/v1alpha1](#monitoringspecaddonsmanagedopenshiftiov1alpha1) | false |

[Back to Group]()

### AddonStatus.addons.managed.openshift.io/v1alpha1

AddonStatus defines the observed state of Addon

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| observedGeneration | The most recent generation observed by the controller. | int64 | false |
| conditions | Conditions is a list of status conditions ths object is in. | []metav1.Condition | false |
| phase | DEPRECATED: This field is not part of any API contract it will go away as soon as kubectl can print conditions! Human readable status - please use .Conditions from code | AddonPhase.addons.managed.openshift.io/v1alpha1 | false |
| upgradePolicy | Tracks last reported upgrade policy status. | *[AddonUpgradePolicyStatus.addons.managed.openshift.io/v1alpha1](#addonupgradepolicystatusaddonsmanagedopenshiftiov1alpha1) | false |
| observedVersion | Observed version of the Addon on the cluster, only present when .spec.version is populated. | string | false |

[Back to Group]()

### AddonUpgradePolicy.addons.managed.openshift.io/v1alpha1



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| id | Upgrade policy id. | string | true |

[Back to Group]()

### AddonUpgradePolicyStatus.addons.managed.openshift.io/v1alpha1

Tracks the last state last reported to the Upgrade Policy endpoint.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| id | Upgrade policy id. | string | true |
| value | Upgrade policy value. | AddonUpgradePolicyValue.addons.managed.openshift.io/v1alpha1 | true |
| observedGeneration | The most recent generation a status update was based on. | int64 | true |

[Back to Group]()

### EnvObject.addons.managed.openshift.io/v1alpha1



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name of the environment variable | string | true |
| value | Value of the environment variable | string | true |

[Back to Group]()

### MonitoringFederationSpec.addons.managed.openshift.io/v1alpha1



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| namespace | Namespace where the prometheus server is running. | string | true |
| matchNames | List of series names to federate from the prometheus server. | []string | true |
| matchLabels | List of labels used to discover the prometheus server(s) to be federated. | map[string]string | true |

[Back to Group]()

### MonitoringSpec.addons.managed.openshift.io/v1alpha1



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| federation | Configuration parameters to be injected in the ServiceMonitor used for federation. The target prometheus server found by matchLabels needs to serve service-ca signed TLS traffic (https://docs.openshift.com/container-platform/4.6/security/certificate_types_descriptions/service-ca-certificates.html), and it needs to be runing inside the namespace specified by `.monitoring.federation.namespace` with the service name 'prometheus'. | *[MonitoringFederationSpec.addons.managed.openshift.io/v1alpha1](#monitoringfederationspecaddonsmanagedopenshiftiov1alpha1) | false |

[Back to Group]()

### SubscriptionConfig.addons.managed.openshift.io/v1alpha1



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| env | Array of env variables to be passed to the subscription object. | [][EnvObject.addons.managed.openshift.io/v1alpha1](#envobjectaddonsmanagedopenshiftiov1alpha1) | true |

[Back to Group]()

### ClusterSecretReference.addons.managed.openshift.io/v1alpha1

References a secret on the cluster.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name of the secret object. | string | true |
| namespace | Namespace of the secret object. | string | true |

[Back to Group]()
