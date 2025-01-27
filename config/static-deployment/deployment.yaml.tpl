apiVersion: apps/v1
kind: Deployment
metadata:
  name: package-operator-manager
  namespace: package-operator-system
  labels:
    app.kubernetes.io/name: package-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: package-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: package-operator
    spec:
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      serviceAccountName: package-operator
      containers:
      - name: manager
        image: quay.io/package-operator/package-operator-manager:latest
        args:
        - --enable-leader-election
        env:
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
        - name: PKO_PACKAGE_OPERATOR_PACKAGE_IMAGE
          value: "quay.io/package-operator/package-operator-package:latest"
        ports:
        - name: metrics
          containerPort: 8080
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 100m
            memory: 400Mi
          requests:
            cpu: 100m
            memory: 300Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
