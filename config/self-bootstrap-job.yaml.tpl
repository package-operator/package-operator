apiVersion: v1
kind: Namespace
metadata:
  name: package-operator-system
  labels:
    package-operator.run/cache: "True"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: package-operator
  namespace: package-operator-system
  labels:
    package-operator.run/cache: "True"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: package-operator
  labels:
    package-operator.run/cache: "True"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: package-operator
  namespace: package-operator-system
---
apiVersion: batch/v1
kind: Job
metadata:
  name: package-operator-bootstrap
  namespace: package-operator-system
spec:
  # delete right after completion
  ttlSecondsAfterFinished: 0
  # set deadline to 30min
  activeDeadlineSeconds: 1800
  template:
    spec:
      restartPolicy: OnFailure
      serviceAccountName: package-operator
      containers:
      - name: package-operator
        image: quay.io/package-operator/package-operator-manager:latest
        args: ["-self-bootstrap=quay.io/package-operator/package-operator-package:latest"]
        env:
        - name: PKO_REGISTRY_HOST_OVERRIDES
          value: "##registry-overrides##"
        - name: PKO_CONFIG
          value: '##pko-config##'
        - name: PKO_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
  backoffLimit: 3
