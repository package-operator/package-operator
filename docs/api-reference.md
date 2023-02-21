## coordination.package-operator.run/v1alpha1

The package v1alpha1 contains API Schema definitions for the v1alpha1 version of the coordination Package Operator API group,
containing helper APIs to coordinate rollout into the cluster.

* [Adoption](#adoption)
* [ClusterAdoption](#clusteradoption)


### Adoption

Adoption assigns initial labels to objects using one of multiple strategies.
e.g. to route them to a specific operator instance.


**Example**

```yaml
apiVersion: coordination.package-operator.run/v1alpha1
kind: Adoption
metadata:
  name: example
  namespace: default
spec:
  strategy:
    roundRobin:
      always: map[string]string
      options:
      - map[string]string
    static:
      labels: map[string]string
    type: Static
  targetAPI:
    group: lorem
    kind: dolor
    version: ipsum
status:
  phase:Pending: null

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#adoptionspec">AdoptionSpec</a> | AdoptionSpec defines the desired state of an Adoption. |
| `status` <br><a href="#adoptionstatus">AdoptionStatus</a> | AdoptionStatus defines the observed state of an Adoption. |


### ClusterAdoption

ClusterAdoption assigns initial labels to objects using one of multiple strategies.
e.g. to route them to a specific operator instance.


**Example**

```yaml
apiVersion: coordination.package-operator.run/v1alpha1
kind: ClusterAdoption
metadata:
  name: example
spec:
  strategy:
    roundRobin:
      always: map[string]string
      options:
      - map[string]string
    static:
      labels: map[string]string
    type: Static
  targetAPI:
    group: sit
    kind: consetetur
    version: amet
status:
  phase:Pending: null

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#clusteradoptionspec">ClusterAdoptionSpec</a> | ClusterAdoptionSpec defines the desired state of an ClusterAdoption. |
| `status` <br><a href="#clusteradoptionstatus">ClusterAdoptionStatus</a> | ClusterAdoptionStatus defines the observed state of an ClusterAdoption. |




---

### AdoptionRoundRobinStatus



| Field | Description |
| ----- | ----------- |
| `lastIndex` <b>required</b><br>int | Last index chosen by the round robin algorithm. |


Used in:
* [AdoptionStatus](#adoptionstatus)
* [ClusterAdoptionStatus](#clusteradoptionstatus)


### AdoptionSpec

AdoptionSpec defines the desired state of an Adoption.

| Field | Description |
| ----- | ----------- |
| `strategy` <b>required</b><br><a href="#adoptionstrategy">AdoptionStrategy</a> | Strategy to use for adoption. |
| `targetAPI` <b>required</b><br><a href="#targetapi">TargetAPI</a> | TargetAPI to use for adoption. |


Used in:
* [Adoption](#adoption)


### AdoptionStatus

AdoptionStatus defines the observed state of an Adoption.

| Field | Description |
| ----- | ----------- |
| `observedGeneration` <br>int64 | The most recent generation observed by the controller. |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#adoptionphase">AdoptionPhase</a> | This field is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |
| `roundRobin` <br><a href="#adoptionroundrobinstatus">AdoptionRoundRobinStatus</a> | Tracks round robin state to restart where the last operation ended. |


Used in:
* [Adoption](#adoption)


### AdoptionStrategy

AdoptionStrategy defines the strategy to handover objects.

| Field | Description |
| ----- | ----------- |
| `type` <b>required</b><br><a href="#adoptionstrategytype">AdoptionStrategyType</a> | Type of adoption strategy. Can be "Static", "RoundRobin". |
| `static` <br><a href="#adoptionstrategystaticspec">AdoptionStrategyStaticSpec</a> | Static adoption strategy configuration.<br>Only present when type=Static. |
| `roundRobin` <br><a href="#adoptionstrategyroundrobinspec">AdoptionStrategyRoundRobinSpec</a> | RoundRobin adoption strategy configuration.<br>Only present when type=RoundRobin. |


Used in:
* [AdoptionSpec](#adoptionspec)


### AdoptionStrategyRoundRobinSpec



| Field | Description |
| ----- | ----------- |
| `always` <b>required</b><br><a href="#map[string]string">map[string]string</a> | Labels to set always, no matter the round robin choice. |
| `options` <b>required</b><br><a href="#map[string]string">[]map[string]string</a> | Options for the round robin strategy to choose from.<br>Only a single label set of all the provided options will be applied. |


Used in:
* [AdoptionStrategy](#adoptionstrategy)
* [ClusterAdoptionStrategy](#clusteradoptionstrategy)


### AdoptionStrategyStaticSpec



| Field | Description |
| ----- | ----------- |
| `labels` <b>required</b><br><a href="#map[string]string">map[string]string</a> | Labels to set on objects. |


Used in:
* [AdoptionStrategy](#adoptionstrategy)
* [ClusterAdoptionStrategy](#clusteradoptionstrategy)


### ClusterAdoptionSpec

ClusterAdoptionSpec defines the desired state of an ClusterAdoption.

| Field | Description |
| ----- | ----------- |
| `strategy` <b>required</b><br><a href="#clusteradoptionstrategy">ClusterAdoptionStrategy</a> | Strategy to use for adoption. |
| `targetAPI` <b>required</b><br><a href="#targetapi">TargetAPI</a> | TargetAPI to use for adoption. |


Used in:
* [ClusterAdoption](#clusteradoption)


### ClusterAdoptionStatus

ClusterAdoptionStatus defines the observed state of an ClusterAdoption.

| Field | Description |
| ----- | ----------- |
| `observedGeneration` <br>int64 | The most recent generation observed by the controller. |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#adoptionphase">AdoptionPhase</a> | This field is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |
| `roundRobin` <br><a href="#adoptionroundrobinstatus">AdoptionRoundRobinStatus</a> | Tracks round robin state to restart where the last operation ended. |


Used in:
* [ClusterAdoption](#clusteradoption)


### ClusterAdoptionStrategy

ClusterAdoptionStrategy defines the strategy to handover objects.

| Field | Description |
| ----- | ----------- |
| `type` <b>required</b><br><a href="#adoptionstrategytype">AdoptionStrategyType</a> | Type of adoption strategy. Can be "Static", "RoundRobin". |
| `static` <br><a href="#adoptionstrategystaticspec">AdoptionStrategyStaticSpec</a> | Static adoption strategy configuration.<br>Only present when type=Static. |
| `roundRobin` <br><a href="#adoptionstrategyroundrobinspec">AdoptionStrategyRoundRobinSpec</a> | RoundRobin adoption strategy configuration.<br>Only present when type=RoundRobin. |


Used in:
* [ClusterAdoptionSpec](#clusteradoptionspec)


### TargetAPI

TargetAPI specifies an API to use for operations.

| Field | Description |
| ----- | ----------- |
| `group` <b>required</b><br>string |  |
| `version` <b>required</b><br>string |  |
| `kind` <b>required</b><br>string |  |


Used in:
* [AdoptionSpec](#adoptionspec)
* [ClusterAdoptionSpec](#clusteradoptionspec)
## manifests.package-operator.run/v1alpha1

The package v1alpha1 contains API Schema definitions for the v1alpha1 version of the manifests API group,
containing file-based manifests for the packaging infrastructure.

* [PackageManifest](#packagemanifest)
* [PackageManifestLock](#packagemanifestlock)


### PackageManifest




**Example**

```yaml
apiVersion: manifests.package-operator.run/v1alpha1
kind: PackageManifest
metadata:
  name: example
  namespace: default
spec:
  availabilityProbes:
  - corev1alpha1.ObjectSetProbe
  config:
    openAPIV3Schema: apiextensionsv1.JSONSchemaProps
  images:
  - image: sit
    name: dolor
  phases:
  - class: ipsum
    name: lorem
  scopes:
  - PackageManifestScope
test:
  template:
  - context:
      config: runtime.RawExtension
      package:
        metadata:
          annotations: map[string]string
          labels: map[string]string
          name: consetetur
          namespace: sadipscing
    name: amet

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#packagemanifestspec">PackageManifestSpec</a> | PackageManifestSpec represents the spec of the packagemanifest containing the details about phases and availability probes. |
| `test` <br><a href="#packagemanifesttest">PackageManifestTest</a> | PackageManifestTest configures test cases. |


### PackageManifestLock




**Example**

```yaml
apiVersion: manifests.package-operator.run/v1alpha1
kind: PackageManifestLock
metadata:
  name: example
  namespace: default
spec:
  images:
  - digest: diam
    image: sed
    name: elitr

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#packagemanifestlockspec">PackageManifestLockSpec</a> |  |




---

### PackageManifestImage

PackageManifestImage specifies an image tag to be resolved

| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string | Image name to be use to reference it in the templates |
| `image` <b>required</b><br>string | Image identifier (REPOSITORY[:TAG]) |


Used in:
* [PackageManifestSpec](#packagemanifestspec)


### PackageManifestLockImage

PackageManifestLockImage contains information about a resolved image

| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string | Image name to be use to reference it in the templates |
| `image` <b>required</b><br>string | Image identifier (REPOSITORY[:TAG]) |
| `digest` <b>required</b><br>string | Image digest |


Used in:
* [PackageManifestLockSpec](#packagemanifestlockspec)


### PackageManifestLockSpec



| Field | Description |
| ----- | ----------- |
| `images` <b>required</b><br><a href="#packagemanifestlockimage">[]PackageManifestLockImage</a> | List of resolved images |


Used in:
* [PackageManifestLock](#packagemanifestlock)


### PackageManifestPhase



| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string | Name of the reconcile phase. Must be unique within a PackageManifest |
| `class` <br>string | If non empty, phase reconciliation is delegated to another controller.<br>If set to the string "default" the built-in controller reconciling the object.<br>If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects. |


Used in:
* [PackageManifestSpec](#packagemanifestspec)


### PackageManifestSpec

PackageManifestSpec represents the spec of the packagemanifest containing the details about phases and availability probes.

| Field | Description |
| ----- | ----------- |
| `scopes` <b>required</b><br><a href="#packagemanifestscope">[]PackageManifestScope</a> | Scopes declare the available installation scopes for the package.<br>Either Cluster, Namespaced, or both. |
| `phases` <b>required</b><br><a href="#packagemanifestphase">[]PackageManifestPhase</a> | Phases correspond to the references to the phases which are going to be the part of the ObjectDeployment/ClusterObjectDeployment. |
| `availabilityProbes` <b>required</b><br>[]corev1alpha1.ObjectSetProbe | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |
| `config` <br><a href="#packagemanifestspecconfig">PackageManifestSpecConfig</a> | Configuration specification. |
| `images` <b>required</b><br><a href="#packagemanifestimage">[]PackageManifestImage</a> | List of images to be resolved |


Used in:
* [PackageManifest](#packagemanifest)


### PackageManifestSpecConfig



| Field | Description |
| ----- | ----------- |
| `openAPIV3Schema` <br>apiextensionsv1.JSONSchemaProps | OpenAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning. |


Used in:
* [PackageManifestSpec](#packagemanifestspec)


### PackageManifestTest

PackageManifestTest configures test cases.

| Field | Description |
| ----- | ----------- |
| `template` <br><a href="#packagemanifesttestcasetemplate">[]PackageManifestTestCaseTemplate</a> | Template testing configuration. |


Used in:
* [PackageManifest](#packagemanifest)


### PackageManifestTestCaseTemplate

PackageManifestTestCaseTemplate template testing configuration.

| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string | Name describing the test case. |
| `context` <br><a href="#templatecontext">TemplateContext</a> | Template data to use in the test case. |


Used in:
* [PackageManifestTest](#packagemanifesttest)


### TemplateContext

TemplateContext is available within the package templating process.

| Field | Description |
| ----- | ----------- |
| `package` <b>required</b><br><a href="#templatecontextpackage">TemplateContextPackage</a> | TemplateContextPackage represents the (Cluster)Package object requesting this package content. |
| `config` <br>runtime.RawExtension |  |


Used in:
* [PackageManifestTestCaseTemplate](#packagemanifesttestcasetemplate)


### TemplateContextObjectMeta

TemplateContextObjectMeta represents a simplified version of metav1.ObjectMeta for use in templates.

| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string |  |
| `namespace` <b>required</b><br>string |  |
| `labels` <b>required</b><br><a href="#map[string]string">map[string]string</a> |  |
| `annotations` <b>required</b><br><a href="#map[string]string">map[string]string</a> |  |


Used in:
* [TemplateContextPackage](#templatecontextpackage)


### TemplateContextPackage

TemplateContextPackage represents the (Cluster)Package object requesting this package content.

| Field | Description |
| ----- | ----------- |
| `metadata` <b>required</b><br><a href="#templatecontextobjectmeta">TemplateContextObjectMeta</a> | TemplateContextObjectMeta represents a simplified version of metav1.ObjectMeta for use in templates. |


Used in:
* [TemplateContext](#templatecontext)
