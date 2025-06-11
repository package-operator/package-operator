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
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      restartPolicy: OnFailure
      serviceAccountName: package-operator
      containers:
      - name: package-operator
        image: "##pko-manager-image##"
        args: ["-self-bootstrap=##pko-package-image##"]
        env:
        - name: PKO_REGISTRY_HOST_OVERRIDES
          value: "##registry-overrides##"
        - name: PKO_IMAGE_PREFIX_OVERRIDES
          value: "##image-prefix-overrides##"
        - name: PKO_CONFIG
          value: '##pko-config##'
        - name: PKO_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: PKO_SERVICE_ACCOUNT_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: PKO_SERVICE_ACCOUNT_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.serviceAccountName
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
  backoffLimit: 3
