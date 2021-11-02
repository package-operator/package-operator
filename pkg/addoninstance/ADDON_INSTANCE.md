# AddonInstance

Purpose - To provide a custom resource called AddonInstance which acts as a central store for capturing the liveness of an addon

## How does it work?

For every Addon which is created on a cluster, a corresponding `AddonInstance` object gets created in its (addon's) targetNamespace.

That addon is programmed accordingly to periodically register its liveness but registering its heartbeats to its corresponding AddonInstance.

Heartbeats are registered merely in the form of `.status.conditions` only

The addon can be programmed to not just register heartbeats corresponding to its liveness but also a bunch of other conditions depending on its status, health and readiness.

Meanwhile, the addon operator is configured with a period heartbeat checker which periodically checks if all the addonInstance resources have a heartbeat registered recently. If any addonInstance doesn't have a heartbeat registered since a good amount of time (for example, 30 seconds), the heartbeat checker updates that AddonInstance's status.conditions to `HeartbeatTimeout`. Nonetheless, the corresponding Addon can choose to come back up anytime and start registering its heartbeats again.

## Let's see it in action

* Setup the local OSD cluster with AddonOperator installed
```sh
make test-local
```

* Create the reference-addon resource in your cluster
```sh
kubectl apply -f config/example/reference-addon.yaml
```

* List the addon to check if it got created
```sh
kubectl get addons

NAME              STATUS   AGE
reference-addon   Ready    8m33s
```

* List the addoninstances in the reference-addon's target namespace to see if it got created or not
```sh
kubectl get addoninstances -n reference-addon

NAME             LAST HEARTBEAT   AGE
addon-instance   12s              3m24s
```

* Export the above addoninstance's resource to YAML'd output and checkout its .status.conditions
```sh
kubectl get addoninstance addon-instance -n reference-addon -o yaml
```

```yaml
...
status:
  conditions:
  - lastTransitionTime: "2021-10-28T06:07:03Z"
    message: Reference Addon is operational.
    reason: ComponentsUp
    status: "True"
    type: addons.managed.openshift.io/Healthy
  lastHeartbeatTime: "2021-10-28T06:07:33Z"
...
```

You wouldn't see anything under its `.status.conditions` with the `type: "addons.managed.openshift.io/Healthy"` because currently, the reference-addon isn't programmed to register heartbeats.<br>
That's why, even the heartbeat checker (running as a part of addon-operator) wouldn't bother marking the above `AddonInstance` with HeartbeatTimeout because it can fairly assume that the absence of heartbeat in this case doesn't correlate to the Addon being dead and it's just because the addon isn't currently programmed to register heartbeats considering it hasn't registered even a single heartbeat in its lifetime (can be checked via `.status.lastHeartbeatTime.IsZero()`).

**No .status.lastHeartbeatTime found**
```yaml
apiVersion: addons.managed.openshift.io/v1alpha1
kind: AddonInstance
metadata:
  creationTimestamp: "2021-10-28T06:11:16Z"
  generation: 1
  name: addon-instance
  namespace: reference-addon
  ownerReferences:
  - apiVersion: addons.managed.openshift.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Addon
    name: reference-addon
    uid: 172fae0e-4b4d-46ae-bd06-2cf23c01cda2
  resourceVersion: "3209"
  uid: 3fa122fa-ef1a-44d3-9e7c-456dc2b44434
spec:
  heartbeatUpdatePeriod: 10s
```

**.status.lastHeartbeaTime found**
```yaml
apiVersion: addons.managed.openshift.io/v1alpha1
kind: AddonInstance
metadata:
  creationTimestamp: "2021-10-28T06:05:23Z"
  generation: 1
  name: addon-instance
  namespace: reference-addon
  ownerReferences:
  - apiVersion: addons.managed.openshift.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Addon
    name: reference-addon
    uid: 172fae0e-4b4d-46ae-bd06-2cf23c01cda2
  resourceVersion: "2603"
  uid: c503ab2a-4406-4e96-af5d-711aeb831e27
spec:
  heartbeatUpdatePeriod: 10s
status:
  conditions:
  - lastTransitionTime: "2021-10-28T06:07:03Z"
    message: Reference Addon is operational.
    reason: ComponentsUp
    status: "True"
    type: addons.managed.openshift.io/Healthy
  lastHeartbeatTime: "2021-10-28T06:07:33Z"
  observedGeneration: 1
```

But once the Addon (reference-addon in this example) registers even a single heartbeat in its lifetime, that would indicate to the `AddonInstance` (via `.status.lastHeartbeatTime`) and hence, to the heartbeat checker that the Addon is capable of registering heartbeats. And from then on, whenever the Addon would fail to register a heartbeat for a good amount of time, the checker would explicitly mark its corresponding `AddonInstance` with `HeartbeatTimeout`.

**.status.lastHeartbeatTime older beyond a certain threshold**
```yaml
apiVersion: addons.managed.openshift.io/v1alpha1
kind: AddonInstance
metadata:
  creationTimestamp: "2021-10-28T06:05:23Z"
  generation: 1
  name: addon-instance
  namespace: reference-addon
  ownerReferences:
  - apiVersion: addons.managed.openshift.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Addon
    name: reference-addon
    uid: 172fae0e-4b4d-46ae-bd06-2cf23c01cda2
  resourceVersion: "2748"
  uid: c503ab2a-4406-4e96-af5d-711aeb831e27
spec:
  heartbeatUpdatePeriod: 10s
status:
  conditions:
  - lastTransitionTime: "2021-10-28T06:08:25Z"
    message: Addon failed to send heartbeat.
    reason: HeartbeatTimeout
    status: Unknown
    type: addons.managed.openshift.io/Healthy
  lastHeartbeatTime: "2021-10-28T06:07:53Z"
  observedGeneration: 1
```

Also, the heartbeat can be in the form of any `.status.condition`. For example, even if some component X is failing or non-operational and that is making the Addon to malfunction, still the Addon can be gracefully register its liveness by registering a heartbeat with the following condition depicting that it's atleast alive:
```yaml
status:
  conditions:
  - lastTransitionTime: "2021-10-28T06:08:25Z"
    message: Component X is non-operational!
    reason: ComponentDown
    status: False
    type: addons.managed.openshift.io/Healthy
  lastHeartbeatTime: "2021-10-28T06:07:53Z"
```


## How can the addon be programmed to register a heartbeat

The addon, in its source code, just has to
```go
import "github.com/mt-sre/addon-operator/pkg/addoninstance"  // TODO: change this as the addoninstance package will be later separated out in a different repository
```
and just call
```go
addoninstance.SetConditions(...)
```
with the apt parameters according to its situation.


For example,

```go

package main

import (
    "fmt"
    "os"
    ...
    ...
    "github.com/mt-sre/addon-operator/pkg/addoninstance"
)

...
...

func main() {

    ...
    ...
    ...
    go func() {
        client := mgr.GetClient() // a cache backed client to talk to Kubernetes API
        addonName := "sample-addon"
        log := ctrl.Log.WithName("components").WithName("SampleAddonHeartbeatReporter")

        // report a heartbeat at every 10 seconds
        for range time.Tick(10*time.Second) {

            // setting the default condition to be true and healthy
            condition := v1.Condition{
				Type:    "addons.managed.openshift.io/Healthy",
				Status:  "True",
				Reason:  "ComponentsUp",
				Message: "Sample Addon is operational",
			}

            // updating the condition based on the following situations
            checkComponentX := checkComponentXHealth()
            if !checkComponentX {
                condition = metav1.Condition{
                    Type:    "addons.managed.openshift.io/Healthy",
                    Status:  "False",
                    Reason:  "ComponentDown",
                    Message: "Sample Addon is non-operational. Component X is failing!",
                }
            }

            checkComponentY := checkComponentYHealth()
            if !checkComponentY {
                condition = metav1.Condition{
                    Type:    "addons.managed.openshift.io/Healthy",
                    Status:  "False",
                    Reason:  "ComponentDown",
                    Message: "Sample Addon is non-operational. Component Y is failing!",
                }
            }

            //register the heartbeat
            if err := addoninstance.SetAddonInstanceConditions(ctx, client, condition, addonName, log); err != nil {
                log.Error("some error occurred", err)
                // or any other error handling mechanism: circuit breakers, backoffs, etc.
            }
        }
    }()
    ...
    ...
    ...

}

```
