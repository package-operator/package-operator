apiVersion: apps/v1
kind: Deployment
metadata:
  name: addon-operator-webhook
  namespace: addon-operator
  labels:
    app.kubernetes.io/name: addon-operator-webook-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: addon-operator-webook-server
  template:
    metadata:
      labels:
        app.kubernetes.io/name: addon-operator-webook-server
    spec:
      serviceAccountName: addon-operator
      containers:
      - name: webhook
        image: quay.io/openshift/addon-operator-webhook:latest
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: tls
          mountPath: "/tmp/k8s-webhook-server/serving-certs/"
          readOnly: true
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
            memory: 30Mi
          requests:
            cpu: 100m
            memory: 20Mi
      volumes:
      - name: tls
        secret:
          secretName: webhook-server-cert
