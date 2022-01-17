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
      - name: manager
        image: quay.io/openshift/addon-operator:latest
        args:
        - --enable-leader-election
        - --enable-metrics-tls-termination # redundant as default is `true` as well but putting it here to highlight it
        volumeMounts:
        - name: tls
          mountPath: "/tmp/k8s-metrics-server/serving-certs/"
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
            memory: 100Mi
          requests:
            cpu: 100m
            memory: 100Mi
