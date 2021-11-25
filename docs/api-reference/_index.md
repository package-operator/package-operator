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
	* [EnvObject](#envobjectaddonsmanagedopenshiftiov1alpha1)
	* [SubscriptionConfig](#subscriptionconfigaddonsmanagedopenshiftiov1alpha1)

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

### AddonOperatorSpec.addons.managed.openshift.io/v1alpha1

AddonOperatorSpec defines the desired state of Addon operator.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| pause | Pause reconciliation on all Addons in the cluster when set to True | bool | true |

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
| pause | Pause reconciliation of Addon when set to True | bool | true |
| namespaces | Defines a list of Kubernetes Namespaces that belong to this Addon. Namespaces listed here will be created prior to installation of the Addon and will be removed from the cluster when the Addon is deleted. Collisions with existing Namespaces are NOT allowed. | [][AddonNamespace.addons.managed.openshift.io/v1alpha1](#addonnamespaceaddonsmanagedopenshiftiov1alpha1) | false |
| install | Defines how an Addon is installed. This field is immutable. | [AddonInstallSpec.addons.managed.openshift.io/v1alpha1](#addoninstallspecaddonsmanagedopenshiftiov1alpha1) | true |
| resourceAdoptionStrategy | ResourceAdoptionStrategy coordinates resource adoption for an Addon Originally introduced for coordinating fleetwide migration on OSD with pre-existing OLM objects. NOTE: This field is for internal usage only and not to be modified by the user. | ResourceAdoptionStrategyType.addons.managed.openshift.io/v1alpha1 | false |

[Back to Group]()

### AddonStatus.addons.managed.openshift.io/v1alpha1

AddonStatus defines the observed state of Addon

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| observedGeneration | The most recent generation observed by the controller. | int64 | false |
| conditions | Conditions is a list of status conditions ths object is in. | []metav1.Condition | false |
| phase | DEPRECATED: This field is not part of any API contract it will go away as soon as kubectl can print conditions! Human readable status - please use .Conditions from code | AddonPhase.addons.managed.openshift.io/v1alpha1 | false |

[Back to Group]()

### EnvObject.addons.managed.openshift.io/v1alpha1



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name of the environment variable | string | true |
| value | Value of the environment variable | string | true |

[Back to Group]()

### SubscriptionConfig.addons.managed.openshift.io/v1alpha1



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| env | Array of env variables to be passed to the subscription object. | [][EnvObject.addons.managed.openshift.io/v1alpha1](#envobjectaddonsmanagedopenshiftiov1alpha1) | true |

[Back to Group]()
