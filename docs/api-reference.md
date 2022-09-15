## package-operator.run/v1alpha1

The package v1alpha1 contains API Schema definitions for the v1alpha1 version of the core Package Operator API group,
containing basic building blocks that other auxiliary APIs can build on top of.

* [ClusterObjectSet](#clusterobjectset)
* [ClusterObjectSetPhase](#clusterobjectsetphase)
* [ObjectSet](#objectset)
* [ObjectSetPhase](#objectsetphase)


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
  - class: ipsum
    name: lorem
    objects:
    - object:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: example-deployment
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
  class: sit
  name: dolor
  objects:
  - object:
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

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#clusterobjectsetphasespec">ClusterObjectSetPhaseSpec</a> | ClusterObjectSetPhaseSpec defines the desired state of a ClusterObjectSetPhase. |
| `status` <br><a href="#clusterobjectsetphasestatus">ClusterObjectSetPhaseStatus</a> | ClusterObjectSetPhaseStatus defines the observed state of a ClusterObjectSetPhase. |


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
  - class: consetetur
    name: amet
    objects:
    - object:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: example-deployment
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
  class: elitr
  name: sadipscing
  objects:
  - object:
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

```


| Field | Description |
| ----- | ----------- |
| `metadata` <br>metav1.ObjectMeta |  |
| `spec` <br><a href="#objectsetphasespec">ObjectSetPhaseSpec</a> | ObjectSetPhaseSpec defines the desired state of a ObjectSetPhase. |
| `status` <br><a href="#objectsetphasestatus">ObjectSetPhaseStatus</a> | ObjectSetPhaseStatus defines the observed state of a ObjectSetPhase. |




---

### ClusterObjectSetPhaseSpec

ClusterObjectSetPhaseSpec defines the desired state of a ClusterObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `paused` <br><a href="#bool">bool</a> | Disables reconciliation of the ObjectSet.<br>Only Status updates will still propagated, but object changes will not be reconciled. |
| `revision` <b>required</b><br>int64 | Revision of the parent ObjectSet to use during object adoption. |
| `previous` <br><a href="#previousrevisionreference">[]PreviousRevisionReference</a> | Previous revisions of the ClusterObjectSet to adopt objects from. |
| `availabilityProbes` <b>required</b><br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |
| `name` <b>required</b><br>string | Name of the reconcile phase. Must be unique within a ObjectSet. |
| `class` <br>string | If non empty, the ObjectSet controller will delegate phase reconciliation to another controller, by creating an ObjectSetPhase object.<br>If set to the string "default" the built-in Package Operator ObjectSetPhase controller will reconcile the object in the same way the ObjectSet would.<br>If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects. |
| `objects` <b>required</b><br><a href="#objectsetobject">[]ObjectSetObject</a> | Objects belonging to this phase. |


Used in:
* [ClusterObjectSetPhase](#clusterobjectsetphase)


### ClusterObjectSetPhaseStatus

ClusterObjectSetPhaseStatus defines the observed state of a ClusterObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |


Used in:
* [ClusterObjectSetPhase](#clusterobjectsetphase)


### ClusterObjectSetSpec

ClusterObjectSetSpec defines the desired state of a ClusterObjectSet.

| Field | Description |
| ----- | ----------- |
| `lifecycleState` <br><a href="#objectsetlifecyclestate">ObjectSetLifecycleState</a> | Specifies the lifecycle state of the ClusterObjectSet. |
| `previous` <br><a href="#previousrevisionreference">[]PreviousRevisionReference</a> | Previous revisions of the ClusterObjectSet to adopt objects from. |
| `phases` <b>required</b><br><a href="#objectsettemplatephase">[]ObjectSetTemplatePhase</a> | Reconcile phase configuration for a ObjectSet.<br>Phases will be reconciled in order and the contained objects checked<br>against given probes before continuing with the next phase. |
| `availabilityProbes` <b>required</b><br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |


Used in:
* [ClusterObjectSet](#clusterobjectset)


### ClusterObjectSetStatus

ClusterObjectSetStatus defines the observed state of a ClusterObjectSet.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#objectsetstatusphase">ObjectSetStatusPhase</a> | This field is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |
| `revision` <br>int64 | Computed revision number, monotonically increasing. |
| `remotePhases` <br><a href="#remotephasereference">[]RemotePhaseReference</a> | Remote phases aka ClusterObjectSetPhase objects. |


Used in:
* [ClusterObjectSet](#clusterobjectset)


### ObjectSetObject

An object that is part of the phase of an ObjectSet.

| Field | Description |
| ----- | ----------- |
| `object` <b>required</b><br>runtime.RawExtension |  |


Used in:
* [ClusterObjectSetPhaseSpec](#clusterobjectsetphasespec)
* [ObjectSetPhaseSpec](#objectsetphasespec)
* [ObjectSetTemplatePhase](#objectsettemplatephase)


### ObjectSetPhaseSpec

ObjectSetPhaseSpec defines the desired state of a ObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `paused` <br><a href="#bool">bool</a> | Disables reconciliation of the ObjectSet.<br>Only Status updates will still propagated, but object changes will not be reconciled. |
| `revision` <b>required</b><br>int64 | Revision of the parent ObjectSet to use during object adoption. |
| `previous` <br><a href="#previousrevisionreference">[]PreviousRevisionReference</a> | Previous revisions of the ClusterObjectSet to adopt objects from. |
| `availabilityProbes` <b>required</b><br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |
| `name` <b>required</b><br>string | Name of the reconcile phase. Must be unique within a ObjectSet. |
| `class` <br>string | If non empty, the ObjectSet controller will delegate phase reconciliation to another controller, by creating an ObjectSetPhase object.<br>If set to the string "default" the built-in Package Operator ObjectSetPhase controller will reconcile the object in the same way the ObjectSet would.<br>If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects. |
| `objects` <b>required</b><br><a href="#objectsetobject">[]ObjectSetObject</a> | Objects belonging to this phase. |


Used in:
* [ObjectSetPhase](#objectsetphase)


### ObjectSetPhaseStatus

ObjectSetPhaseStatus defines the observed state of a ObjectSetPhase.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |


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


### ObjectSetSpec

ObjectSetSpec defines the desired state of a ObjectSet.

| Field | Description |
| ----- | ----------- |
| `lifecycleState` <br><a href="#objectsetlifecyclestate">ObjectSetLifecycleState</a> | Specifies the lifecycle state of the ObjectSet. |
| `previous` <br><a href="#previousrevisionreference">[]PreviousRevisionReference</a> | Previous revisions of the ObjectSet to adopt objects from. |
| `phases` <b>required</b><br><a href="#objectsettemplatephase">[]ObjectSetTemplatePhase</a> | Reconcile phase configuration for a ObjectSet.<br>Phases will be reconciled in order and the contained objects checked<br>against given probes before continuing with the next phase. |
| `availabilityProbes` <b>required</b><br><a href="#objectsetprobe">[]ObjectSetProbe</a> | Availability Probes check objects that are part of the package.<br>All probes need to succeed for a package to be considered Available.<br>Failing probes will prevent the reconciliation of objects in later phases. |


Used in:
* [ObjectSet](#objectset)


### ObjectSetStatus

ObjectSetStatus defines the observed state of a ObjectSet.

| Field | Description |
| ----- | ----------- |
| `conditions` <br>[]metav1.Condition | Conditions is a list of status conditions ths object is in. |
| `phase` <br><a href="#objectsetstatusphase">ObjectSetStatusPhase</a> | This field is not part of any API contract<br>it will go away as soon as kubectl can print conditions!<br>When evaluating object state in code, use .Conditions instead. |
| `revision` <br>int64 | Computed revision number, monotonically increasing. |
| `remotePhases` <br><a href="#remotephasereference">[]RemotePhaseReference</a> | Remote phases aka ObjectSetPhase objects. |


Used in:
* [ObjectSet](#objectset)


### ObjectSetTemplatePhase

ObjectSet reconcile phase.

| Field | Description |
| ----- | ----------- |
| `name` <b>required</b><br>string | Name of the reconcile phase. Must be unique within a ObjectSet. |
| `class` <br>string | If non empty, the ObjectSet controller will delegate phase reconciliation to another controller, by creating an ObjectSetPhase object.<br>If set to the string "default" the built-in Package Operator ObjectSetPhase controller will reconcile the object in the same way the ObjectSet would.<br>If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects. |
| `objects` <b>required</b><br><a href="#objectsetobject">[]ObjectSetObject</a> | Objects belonging to this phase. |


Used in:
* [ClusterObjectSetSpec](#clusterobjectsetspec)
* [ObjectSetSpec](#objectsetspec)


### PackageProbeKindSpec

Kind package probe parameters.
selects objects based on Kind and API Group.

| Field | Description |
| ----- | ----------- |
| `group` <b>required</b><br>string | Object Group to apply a probe to. |
| `kind` <b>required</b><br>string | Object Kind to apply a probe to. |


Used in:
* [ProbeSelector](#probeselector)


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
