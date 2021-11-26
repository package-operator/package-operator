apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-mock
  namespace: api-mock
  labels:
    app.kubernetes.io/name: api-mock
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: api-mock
  template:
    metadata:
      labels:
        app.kubernetes.io/name: api-mock
    spec:
      containers:
      - name: manager
        image: quay.io/app-sre/api-mock:latest
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 30m
            memory: 30Mi
          requests:
            cpu: 30m
            memory: 30Mi
