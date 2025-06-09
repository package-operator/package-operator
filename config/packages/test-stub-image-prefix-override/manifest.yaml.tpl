apiVersion: manifests.package-operator.run/v1alpha1
kind: PackageManifest
metadata:
  name: test-stub
spec:
  scopes:
  - Namespaced
  phases:
  - name: deploy
  availabilityProbes:
  - probes:
    - condition:
        type: Available
        status: "True"
    - fieldsEqual:
        fieldA: .status.updatedReplicas
        fieldB: .status.replicas
    selector:
      kind:
        group: apps
        kind: Deployment
  images:
  - name: teststub
    image: ##image-registry##/src/test-stub-mirror:##tag##
