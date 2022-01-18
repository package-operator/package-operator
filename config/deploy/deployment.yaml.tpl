apiVersion: apps/v1
kind: Deployment
metadata:
  name: addon-operator
  namespace: addon-operator
  labels:
    app.kubernetes.io/name: addon-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: addon-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: addon-operator
    spec:
      serviceAccountName: addon-operator
      volumes:
      - name: tls
        secret:
          secretName: metrics-server-cert
      containers:
      - name: metrics-relay-server
        image: quay.io/brancz/kube-rbac-proxy:v0.11.0
        args:
        - "--secure-listen-address=0.0.0.0:8443"
        - "--upstream=http://127.0.0.1:8080/"
        - "--tls-cert-file=/tmp/k8s-metrics-server/serving-certs/tls.crt"
        - "--tls-private-key-file=/tmp/k8s-metrics-server/serving-certs/tls.key"
        - "--logtostderr=true"
        - "--ignore-paths=/metrics,/healthz"
        - "--v=10"
        volumeMounts:
        - name: tls
          mountPath: "/tmp/k8s-metrics-server/serving-certs/"
          readOnly: true
      - name: manager
        image: quay.io/openshift/addon-operator:latest
        args:
        - --enable-leader-election
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
            memory: 100Mi
          requests:
            cpu: 100m
            memory: 100Mi
