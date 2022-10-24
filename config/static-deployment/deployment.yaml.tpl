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
      serviceAccountName: package-operator
      containers:
      - name: manager
        image: quay.io/openshift/package:latest
        args:
        - --enable-leader-election
        env:
        - name: PKO_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: PKO_SENTRY_DSN
          value: "https://2a9bea4b2ee04fafa940ee32b0e4660b@glitchtip.stage.devshift.net/17"
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
