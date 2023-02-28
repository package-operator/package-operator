## package-operator.run/v1alpha1

The package v1alpha1 contains API Schema definitions for the v1alpha1 version of the core Package Operator API group,
containing basic building blocks that other auxiliary APIs can build on top of.

* [ClusterObjectDeployment](#clusterobjectdeployment)
* [ClusterObjectSet](#clusterobjectset)
* [ClusterObjectSetPhase](#clusterobjectsetphase)
* [ClusterObjectSlice](#clusterobjectslice)
* [ClusterObjectTemplate](#clusterobjecttemplate)
* [ClusterPackage](#clusterpackage)
* [ObjectDeployment](#objectdeployment)
* [ObjectSet](#objectset)
* [ObjectSetPhase](#objectsetphase)
* [ObjectSlice](#objectslice)
* [ObjectTemplate](#objecttemplate)
* [Package](#package)


### ClusterObjectDeployment

ClusterObjectDeployment is the Schema for the ClusterObjectDeployments API


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ClusterObjectDeployment
metadata:
  name: example
spec:
  revisionHistoryLimit: 10
  selector: metav1.LabelSelector
  template:
    metadata: metav1.ObjectMeta
    spec:
      availabilityProbes:
      - probes:
        - condition:
            status: "True"
            type: Available
          fieldsEqual:
            fieldA: .spec.fieldA
            fieldB: .status.fieldB
        selector:
          kind:
            group: apps
            kind: Deployment
          selector:
            matchLabels:
              app.kubernetes.io/name: example-operator
      phases:
      - class: ipsum
        name: lorem
        objects:
        - conditionMappings:
          - destinationType: sit
            sourceType: dolor
          object:
            apiVersion: apps/v1
            kind: Deployment
            metadata:
              name: example-deployment
        slices:
        - amet
status:
  phase:Pending: null

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#clusterobjectdeploymentspec">ClusterObjectDeploymentSpec</a> | ClusterObjectDeploymentSpec defines the desired state of a ClusterObjectDeployment. |
| `status` <br><a href="#clusterobjectdeploymentstatus">ClusterObjectDeploymentStatus</a> | ClusterObjectDeploymentStatus defines the observed state of a ClusterObjectDeployment. |


### ClusterObjectSet

ClusterObjectSet reconciles a collection of objects through ordered phases and aggregates their status.

ClusterObjectSets behave similarly to Kubernetes ReplicaSets, by managing a collection of objects and being itself mostly immutable.
This object type is able to suspend/pause reconciliation of specific objects to facilitate the transition between revisions.

Archived ClusterObjectSets may stay on the cluster, to store information about previous revisions.

A Namespace-scoped version of this API is available as ObjectSet.


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ClusterObjectSet
metadata:
  name: example
spec:
  availabilityProbes:
  - probes:
    - condition:
        status: "True"
        type: Available
      fieldsEqual:
        fieldA: .spec.fieldA
        fieldB: .status.fieldB
    selector:
      kind:
        group: apps
        kind: Deployment
      selector:
        matchLabels:
          app.kubernetes.io/name: example-operator
  lifecycleState: Active
  phases:
  - class: sadipscing
    name: consetetur
    objects:
    - conditionMappings:
      - destinationType: sed
        sourceType: elitr
      object:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: example-deployment
    slices:
    - diam
  previous:
  - name: previous-revision
status:
  phase: Pending

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#clusterobjectsetspec">ClusterObjectSetSpec</a> | ClusterObjectSetSpec defines the desired state of a ClusterObjectSet. |
| `status` <br><a href="#clusterobjectsetstatus">ClusterObjectSetStatus</a> | ClusterObjectSetStatus defines the observed state of a ClusterObjectSet. |


### ClusterObjectSetPhase

ClusterObjectSetPhase is an internal API, allowing a ClusterObjectSet to delegate a single phase to another custom controller.
ClusterObjectSets will create subordinate ClusterObjectSetPhases when `.class` is set within the phase specification.


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ClusterObjectSetPhase
metadata:
  name: example
spec:
  availabilityProbes:
  - probes:
    - condition:
        status: "True"
        type: Available
      fieldsEqual:
        fieldA: .spec.fieldA
        fieldB: .status.fieldB
    selector:
      kind:
        group: apps
        kind: Deployment
      selector:
        matchLabels:
          app.kubernetes.io/name: example-operator
  objects:
  - conditionMappings:
    - destinationType: eirmod
      sourceType: nonumy
    object:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: example-deployment
  paused: "true"
  previous:
  - name: previous-revision
  revision: 42
status:
  conditions:
  - status: "True"
    type: Available
  controllerOf:
  - group: lorem
    kind: tempor
    name: ipsum
    namespace: dolor

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#clusterobjectsetphasespec">ClusterObjectSetPhaseSpec</a> | ClusterObjectSetPhaseSpec defines the desired state of a ClusterObjectSetPhase. |
| `status` <br><a href="#clusterobjectsetphasestatus">ClusterObjectSetPhaseStatus</a> | ClusterObjectSetPhaseStatus defines the observed state of a ClusterObjectSetPhase. |


### ClusterObjectSlice

ClusterObjectSlices are referenced by ObjectSets or ObjectDeployments and contain objects to
limit the size of ObjectSet and ObjectDeployments when big packages are installed.
This is necessary to work around the etcd object size limit of ~1.5MiB and to reduce load on the kube-apiserver.


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ClusterObjectSlice
metadata:
  name: example
objects:
- conditionMappings:
  - destinationType: amet
    sourceType: sit
  object:
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: example-deployment

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `objects` <b>required</b><br><a href="#objectsetobject">[]ObjectSetObject</a> |  |


### ClusterObjectTemplate

ClusterObjectTemplate contain a go template of a Kubernetes manifest. The manifest is then templated with the
sources provided in the .Spec.Sources. The sources can come from objects from any namespace or cluster scoped
objects.


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ClusterObjectTemplate
metadata:
  name: example
spec:
  sources:
  - apiVersion: sadipscing
    items:
    - destination: eirmod
      key: nonumy
    kind: elitr
    name: diam
    namespace: sed
  template: consetetur

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#objecttemplatespec">ObjectTemplateSpec</a> | ObjectTemplateSpec specification. |


### ClusterPackage




**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ClusterPackage
metadata:
  name: example
spec:
  config: runtime.RawExtension
  image: tempor
status:
  phase: Pending

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#packagespec">PackageSpec</a> | Package specification. |
| `status` <br><a href="#packagestatus">PackageStatus</a> | PackageStatus defines the observed state of a Package. |


### ObjectDeployment

ObjectDeployment is the Schema for the ObjectDeployments API


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ObjectDeployment
metadata:
  name: example
  namespace: default
spec:
  revisionHistoryLimit: 10
  selector: metav1.LabelSelector
  template:
    metadata: metav1.ObjectMeta
    spec:
      availabilityProbes:
      - probes:
        - condition:
            status: "True"
            type: Available
          fieldsEqual:
            fieldA: .spec.fieldA
            fieldB: .status.fieldB
        selector:
          kind:
            group: apps
            kind: Deployment
          selector:
            matchLabels:
              app.kubernetes.io/name: example-operator
      phases:
      - class: ipsum
        name: lorem
        objects:
        - conditionMappings:
          - destinationType: sit
            sourceType: dolor
          object:
            apiVersion: apps/v1
            kind: Deployment
            metadata:
              name: example-deployment
        slices:
        - amet
status:
  phase:Pending: null

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#objectdeploymentspec">ObjectDeploymentSpec</a> | ObjectDeploymentSpec defines the desired state of a ObjectDeployment. |
| `status` <br><a href="#objectdeploymentstatus">ObjectDeploymentStatus</a> | ObjectDeploymentStatus defines the observed state of a ObjectDeployment. |


### ObjectSet

ObjectSet reconciles a collection of objects through ordered phases and aggregates their status.

ObjectSets behave similarly to Kubernetes ReplicaSets, by managing a collection of objects and being itself mostly immutable.
This object type is able to suspend/pause reconciliation of specific objects to facilitate the transition between revisions.

Archived ObjectSets may stay on the cluster, to store information about previous revisions.

A Cluster-scoped version of this API is available as ClusterObjectSet.


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ObjectSet
metadata:
  name: example
  namespace: default
spec:
  availabilityProbes:
  - probes:
    - condition:
        status: "True"
        type: Available
      fieldsEqual:
        fieldA: .spec.fieldA
        fieldB: .status.fieldB
    selector:
      kind:
        group: apps
        kind: Deployment
      selector:
        matchLabels:
          app.kubernetes.io/name: example-operator
  lifecycleState: Active
  phases:
  - class: sadipscing
    name: consetetur
    objects:
    - conditionMappings:
      - destinationType: sed
        sourceType: elitr
      object:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: example-deployment
    slices:
    - diam
  previous:
  - name: previous-revision
status:
  phase: Pending

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#objectsetspec">ObjectSetSpec</a> | ObjectSetSpec defines the desired state of a ObjectSet. |
| `status` <br><a href="#objectsetstatus">ObjectSetStatus</a> | ObjectSetStatus defines the observed state of a ObjectSet. |


### ObjectSetPhase

ObjectSetPhase is an internal API, allowing an ObjectSet to delegate a single phase to another custom controller.
ObjectSets will create subordinate ObjectSetPhases when `.class` within the phase specification is set.


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ObjectSetPhase
metadata:
  name: example
  namespace: default
spec:
  availabilityProbes:
  - probes:
    - condition:
        status: "True"
        type: Available
      fieldsEqual:
        fieldA: .spec.fieldA
        fieldB: .status.fieldB
    selector:
      kind:
        group: apps
        kind: Deployment
      selector:
        matchLabels:
          app.kubernetes.io/name: example-operator
  objects:
  - conditionMappings:
    - destinationType: eirmod
      sourceType: nonumy
    object:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: example-deployment
  paused: "true"
  previous:
  - name: previous-revision
  revision: 42
status:
  conditions:
  - status: "True"
    type: Available
  controllerOf:
  - group: lorem
    kind: tempor
    name: ipsum
    namespace: dolor

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#objectsetphasespec">ObjectSetPhaseSpec</a> | ObjectSetPhaseSpec defines the desired state of a ObjectSetPhase. |
| `status` <br><a href="#objectsetphasestatus">ObjectSetPhaseStatus</a> | ObjectSetPhaseStatus defines the observed state of a ObjectSetPhase. |


### ObjectSlice

ObjectSlices are referenced by ObjectSets or ObjectDeployments and contain objects to
limit the size of ObjectSets and ObjectDeployments when big packages are installed.
This is necessary to work around the etcd object size limit of ~1.5MiB and to reduce load on the kube-apiserver.


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ObjectSlice
metadata:
  name: example
  namespace: default
objects:
- conditionMappings:
  - destinationType: amet
    sourceType: sit
  object:
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: example-deployment

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `objects` <b>required</b><br><a href="#objectsetobject">[]ObjectSetObject</a> |  |


### ObjectTemplate

ObjectTemplates contain a go template of a Kubernetes manifest. This manifest is then templated with the
sources provided in the .Spec.Sources. The sources can only come from objects within the same nampespace
as the ObjectTemplate.


**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: ObjectTemplate
metadata:
  name: example
  namespace: default
spec:
  sources:
  - apiVersion: sadipscing
    items:
    - destination: eirmod
      key: nonumy
    kind: elitr
    name: diam
    namespace: sed
  template: consetetur

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#objecttemplatespec">ObjectTemplateSpec</a> | ObjectTemplateSpec specification. |


### Package




**Example**

```yaml
apiVersion: package-operator.run/v1alpha1
kind: Package
metadata:
  name: example
  namespace: default
spec:
  config: runtime.RawExtension
  image: tempor
status:
  phase: Pending

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#packagespec">PackageSpec</a> | Package specification. |
| `status` <br><a href="#packagestatus">PackageStatus</a> | PackageStatus defines the observed state of a Package. |




---

### ClusterObjectDeploymentSpec

ClusterObjectDeploymentSpec defines the desired state of a ClusterObjectDeployment.

| Field | Description |
| ----- | ----------- |
| `revisionHistoryLimit` <br><a href="#int32">int32</a> | Number of old revisions in the form of archived ObjectSets to keep. |
| `selector` <b>required</b><br>metav1.LabelSelector | Selector targets ObjectSets managed by this Deployment. |
| `template` <b>required</b><br><a href="#objectsettemplate">ObjectSetTemplate</a> | Template to create new ObjectSets from. |


Used in:
* [ClusterObjectDeployment](#clusterobjectdeployment)


### ClusterObjectDeploymentStatus

ClusterObjectDeploymentStatus defines the observed state of a ClusterObjectDeployment.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#objectdeploymentphase">ObjectDeploymentPhase</a> | This field is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |
| `collisionCount` <br><a href="#int32">int32</a> | Count of hash collisions of the ClusterObjectDeployment. |
| `templateHash` <br>string | Computed TemplateHash. |


Used in:
* [ClusterObjectDeployment](#clusterobjectdeployment)


### ClusterObjectSetPhaseSpec

ClusterObjectSetPhaseSpec defines the desired state of a ClusterObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `paused` <br><a href="#bool">bool</a> | Disables reconciliation of the ClusterObjectSet.<br>Only Status updates will still be propagated, but object changes will not be reconciled. |
| `revision` <b>required</b><br>int64 | Revision of the parent ObjectSet to use during object adoption. |
| `previous` <br><a href="#previousrevisionreference">[]PreviousRevisionReference</a> | Previous revisions of the ClusterObjectSet to adopt objects from. |
| `availabilityProbes` <br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |
| `objects` <b>required</b><br><a href="#objectsetobject">[]ObjectSetObject</a> | Objects belonging to this phase. |


Used in:
* [ClusterObjectSetPhase](#clusterobjectsetphase)


### ClusterObjectSetPhaseStatus

ClusterObjectSetPhaseStatus defines the observed state of a ClusterObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `controllerOf` <br><a href="#controlledobjectreference">[]ControlledObjectReference</a> | References all objects controlled by this instance. |


Used in:
* [ClusterObjectSetPhase](#clusterobjectsetphase)


### ClusterObjectSetSpec

ClusterObjectSetSpec defines the desired state of a ClusterObjectSet.

| Field | Description |
| ----- | ----------- |
| `lifecycleState` <br><a href="#objectsetlifecyclestate">ObjectSetLifecycleState</a> | Specifies the lifecycle state of the ClusterObjectSet. |
| `previous` <br><a href="#previousrevisionreference">[]PreviousRevisionReference</a> | Previous revisions of the ClusterObjectSet to adopt objects from. |
| `phases` <br><a href="#objectsettemplatephase">[]ObjectSetTemplatePhase</a> | Reconcile phase configuration for a ObjectSet.<br>Phases will be reconciled in order and the contained objects checked<br>against given probes before continuing with the next phase. |
| `availabilityProbes` <br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |


Used in:
* [ClusterObjectSet](#clusterobjectset)


### ClusterObjectSetStatus

ClusterObjectSetStatus defines the observed state of a ClusterObjectSet.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#objectsetstatusphase">ObjectSetStatusPhase</a> | Phase is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |
| `revision` <br>int64 | Computed revision number, monotonically increasing. |
| `remotePhases` <br><a href="#remotephasereference">[]RemotePhaseReference</a> | Remote phases aka ClusterObjectSetPhase objects. |
| `controllerOf` <br><a href="#controlledobjectreference">[]ControlledObjectReference</a> | References all objects controlled by this instance. |


Used in:
* [ClusterObjectSet](#clusterobjectset)


### ConditionMapping



| Field | Description |
| ----- | ----------- |
| `sourceType` <b>required</b><br>string | Source condition type. |
| `destinationType` <b>required</b><br>string | Destination condition type to report into Package Operator APIs. |


Used in:
* [ObjectSetObject](#objectsetobject)


### ControlledObjectReference

References an object controlled by this ObjectSet/ObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `kind` <b>required</b><br>string | Object Kind. |
| `group` <b>required</b><br>string | Object Group. |
| `name` <b>required</b><br>string | Object Name. |
| `namespace` <br>string | Object Namespace. |


Used in:
* [ClusterObjectSetPhaseStatus](#clusterobjectsetphasestatus)
* [ClusterObjectSetStatus](#clusterobjectsetstatus)
* [ObjectSetPhaseStatus](#objectsetphasestatus)
* [ObjectSetStatus](#objectsetstatus)


### ObjectDeploymentSpec

ObjectDeploymentSpec defines the desired state of a ObjectDeployment.

| Field | Description |
| ----- | ----------- |
| `revisionHistoryLimit` <br><a href="#int32">int32</a> | Number of old revisions in the form of archived ObjectSets to keep. |
| `selector` <b>required</b><br>metav1.LabelSelector | Selector targets ObjectSets managed by this Deployment. |
| `template` <b>required</b><br><a href="#objectsettemplate">ObjectSetTemplate</a> | Template to create new ObjectSets from. |


Used in:
* [ObjectDeployment](#objectdeployment)


### ObjectDeploymentStatus

ObjectDeploymentStatus defines the observed state of a ObjectDeployment.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#objectdeploymentphase">ObjectDeploymentPhase</a> | This field is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |
| `collisionCount` <br><a href="#int32">int32</a> | Count of hash collisions of the ObjectDeployment. |
| `templateHash` <br>string | Computed TemplateHash. |


Used in:
* [ObjectDeployment](#objectdeployment)


### ObjectSetObject

An object that is part of the phase of an ObjectSet.

| Field | Description |
| ----- | ----------- |
| `object` <b>required</b><br>unstructured.Unstructured |  |
| `conditionMappings` <br><a href="#conditionmapping">[]ConditionMapping</a> | Maps conditions from this object into the Package Operator APIs. |


Used in:
* [ClusterObjectSetPhaseSpec](#clusterobjectsetphasespec)
* [ObjectSetPhaseSpec](#objectsetphasespec)
* [ObjectSetTemplatePhase](#objectsettemplatephase)
* [ClusterObjectSlice](#clusterobjectslice)
* [ObjectSlice](#objectslice)


### ObjectSetPhaseSpec

ObjectSetPhaseSpec defines the desired state of a ObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `paused` <br><a href="#bool">bool</a> | Disables reconciliation of the ObjectSet.<br>Only Status updates will still be propagated, but object changes will not be reconciled. |
| `revision` <b>required</b><br>int64 | Revision of the parent ObjectSet to use during object adoption. |
| `previous` <br><a href="#previousrevisionreference">[]PreviousRevisionReference</a> | Previous revisions of the ObjectSet to adopt objects from. |
| `availabilityProbes` <br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |
| `objects` <b>required</b><br><a href="#objectsetobject">[]ObjectSetObject</a> | Objects belonging to this phase. |


Used in:
* [ObjectSetPhase](#objectsetphase)


### ObjectSetPhaseStatus

ObjectSetPhaseStatus defines the observed state of a ObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `controllerOf` <br><a href="#controlledobjectreference">[]ControlledObjectReference</a> | References all objects controlled by this instance. |


Used in:
* [ObjectSetPhase](#objectsetphase)


### ObjectSetProbe

ObjectSetProbe define how ObjectSets check their children for their status.

| Field | Description |
| ----- | ----------- |
| `probes` <b>required</b><br><a href="#probe">[]Probe</a> | Probe configuration parameters. |
| `selector` <b>required</b><br><a href="#probeselector">ProbeSelector</a> | Selector specifies which objects this probe should target. |


Used in:
* [ClusterObjectSetPhaseSpec](#clusterobjectsetphasespec)
* [ClusterObjectSetSpec](#clusterobjectsetspec)
* [ObjectSetPhaseSpec](#objectsetphasespec)
* [ObjectSetSpec](#objectsetspec)
* [ObjectSetTemplateSpec](#objectsettemplatespec)


### ObjectSetSpec

ObjectSetSpec defines the desired state of a ObjectSet.

| Field | Description |
| ----- | ----------- |
| `lifecycleState` <br><a href="#objectsetlifecyclestate">ObjectSetLifecycleState</a> | Specifies the lifecycle state of the ObjectSet. |
| `previous` <br><a href="#previousrevisionreference">[]PreviousRevisionReference</a> | Previous revisions of the ObjectSet to adopt objects from. |
| `phases` <br><a href="#objectsettemplatephase">[]ObjectSetTemplatePhase</a> | Reconcile phase configuration for a ObjectSet.<br>Phases will be reconciled in order and the contained objects checked<br>against given probes before continuing with the next phase. |
| `availabilityProbes` <br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |


Used in:
* [ObjectSet](#objectset)


### ObjectSetStatus

ObjectSetStatus defines the observed state of a ObjectSet.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#objectsetstatusphase">ObjectSetStatusPhase</a> | Phase is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |
| `revision` <br>int64 | Computed revision number, monotonically increasing. |
| `remotePhases` <br><a href="#remotephasereference">[]RemotePhaseReference</a> | Remote phases aka ObjectSetPhase objects. |
| `controllerOf` <br><a href="#controlledobjectreference">[]ControlledObjectReference</a> | References all objects controlled by this instance. |


Used in:
* [ObjectSet](#objectset)


### ObjectSetTemplate

ObjectSetTemplate describes the template to create new ObjectSets from.

| Field | Description |
| ----- | ----------- |
| `metadata` <b>required</b><br>metav1.ObjectMeta | Common Object Metadata. |
| `spec` <b>required</b><br><a href="#objectsettemplatespec">ObjectSetTemplateSpec</a> | ObjectSet specification. |


Used in:
* [ClusterObjectDeploymentSpec](#clusterobjectdeploymentspec)
* [ObjectDeploymentSpec](#objectdeploymentspec)


### ObjectSetTemplatePhase

ObjectSet reconcile phase.

| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string | Name of the reconcile phase. Must be unique within a ObjectSet. |
| `class` <br>string | If non empty, the ObjectSet controller will delegate phase reconciliation to another controller, by creating an ObjectSetPhase object.<br>If set to the string "default" the built-in Package Operator ObjectSetPhase controller will reconcile the object in the same way the ObjectSet would.<br>If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects. |
| `objects` <br><a href="#objectsetobject">[]ObjectSetObject</a> | Objects belonging to this phase. |
| `slices` <br>[]string | References to ObjectSlices containing objects for this phase. |


Used in:
* [ClusterObjectSetSpec](#clusterobjectsetspec)
* [ObjectSetSpec](#objectsetspec)
* [ObjectSetTemplateSpec](#objectsettemplatespec)


### ObjectSetTemplateSpec

ObjectSet specification.

| Field | Description |
| ----- | ----------- |
| `phases` <br><a href="#objectsettemplatephase">[]ObjectSetTemplatePhase</a> | Reconcile phase configuration for a ObjectSet.<br>Phases will be reconciled in order and the contained objects checked<br>against given probes before continuing with the next phase. |
| `availabilityProbes` <br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |


Used in:
* [ObjectSetTemplate](#objectsettemplate)


### ObjectTemplateSource



| Field | Description |
| ----- | ----------- |
| `apiVersion` <b>required</b><br>string |  |
| `kind` <b>required</b><br>string |  |
| `namespace` <br>string |  |
| `name` <b>required</b><br>string |  |
| `items` <b>required</b><br><a href="#objecttemplatesourceitem">[]ObjectTemplateSourceItem</a> |  |


Used in:
* [ObjectTemplateSpec](#objecttemplatespec)


### ObjectTemplateSourceItem



| Field | Description |
| ----- | ----------- |
| `key` <b>required</b><br>string |  |
| `destination` <b>required</b><br>string |  |


Used in:
* [ObjectTemplateSource](#objecttemplatesource)


### ObjectTemplateSpec

ObjectTemplateSpec specification.

| Field | Description |
| ----- | ----------- |
| `template` <b>required</b><br>string | Go template of a Kubernetes manifest |
| `sources` <b>required</b><br><a href="#objecttemplatesource">[]ObjectTemplateSource</a> | Objects in which configuration parameters are fetched |


Used in:
* [ClusterObjectTemplate](#clusterobjecttemplate)
* [ObjectTemplate](#objecttemplate)


### PackageProbeKindSpec

Kind package probe parameters.
selects objects based on Kind and API Group.

| Field | Description |
| ----- | ----------- |
| `group` <b>required</b><br>string | Object Group to apply a probe to. |
| `kind` <b>required</b><br>string | Object Kind to apply a probe to. |


Used in:
* [ProbeSelector](#probeselector)


### PackageSpec

Package specification.

| Field | Description |
| ----- | ----------- |
| `image` <b>required</b><br>string | the image containing the contents of the package<br>this image will be unpacked by the package-loader to render the ObjectDeployment for propagating the installation of the package. |
| `config` <br>runtime.RawExtension | Package configuration parameters. |


Used in:
* [ClusterPackage](#clusterpackage)
* [Package](#package)


### PackageStatus

PackageStatus defines the observed state of a Package.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#packagestatusphase">PackageStatusPhase</a> | This field is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |


Used in:
* [ClusterPackage](#clusterpackage)
* [Package](#package)


### PreviousRevisionReference

References a previous revision of an ObjectSet or ClusterObjectSet.

| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string | Name of a previous revision. |


Used in:
* [ClusterObjectSetPhaseSpec](#clusterobjectsetphasespec)
* [ClusterObjectSetSpec](#clusterobjectsetspec)
* [ObjectSetPhaseSpec](#objectsetphasespec)
* [ObjectSetSpec](#objectsetspec)


### Probe

Defines probe parameters. Only one can be filled.

| Field | Description |
| ----- | ----------- |
| `condition` <br><a href="#probeconditionspec">ProbeConditionSpec</a> | Checks whether or not the object reports a condition with given type and status. |
| `fieldsEqual` <br><a href="#probefieldsequalspec">ProbeFieldsEqualSpec</a> | Compares two fields specified by JSON Paths. |


Used in:
* [ObjectSetProbe](#objectsetprobe)


### ProbeConditionSpec

Checks whether or not the object reports a condition with given type and status.

| Field | Description |
| ----- | ----------- |
| `type` <b>required</b><br>string | Condition type to probe for. |
| `status` <b>required</b><br>string | Condition status to probe for. |


Used in:
* [Probe](#probe)


### ProbeFieldsEqualSpec

Compares two fields specified by JSON Paths.

| Field | Description |
| ----- | ----------- |
| `fieldA` <b>required</b><br>string | First field for comparison. |
| `fieldB` <b>required</b><br>string | Second field for comparison. |


Used in:
* [Probe](#probe)


### ProbeSelector

Selects a subset of objects to apply probes to.
e.g. ensures that probes defined for apps/Deployments are not checked against ConfigMaps.

| Field | Description |
| ----- | ----------- |
| `kind` <b>required</b><br><a href="#packageprobekindspec">PackageProbeKindSpec</a> | Kind and API Group of the object to probe. |
| `selector` <br>metav1.LabelSelector | Further sub-selects objects based on a Label Selector. |


Used in:
* [ObjectSetProbe](#objectsetprobe)


### RemotePhaseReference

References remote phases aka ObjectSetPhase/ClusterObjectSetPhase objects to which a phase is delegated.

| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string |  |
| `uid` <b>required</b><br>types.UID |  |


Used in:
* [ClusterObjectSetStatus](#clusterobjectsetstatus)
* [ObjectSetStatus](#objectsetstatus)
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
