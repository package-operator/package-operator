apiVersion: apps/v1
kind: Deployment
metadata:
  name: package-operator-webhook
  namespace: package-operator-system
  labels:
    app.kubernetes.io/name: package-operator-webook-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: package-operator-webook-server
  template:
    metadata:
      labels:
        app.kubernetes.io/name: package-operator-webook-server
    spec:
      serviceAccountName: package-operator
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - preference:
              matchExpressions:
              - key: node-role.kubernetes.io/infra
                operator: Exists
            weight: 1
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app.kubernetes.io/name
                  operator: In
                  values:
                  - package-operator-webook-server
              topologyKey: "kubernetes.io/hostname"
      tolerations:
        - effect: NoSchedule
          key: node-role.kubernetes.io/infra
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
      containers:
      - name: webhook
        image: quay.io/openshift/package-operator-webhook:latest
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
            cpu: 200m
            memory: 50Mi
          requests:
            cpu: 100m
            memory: 30Mi
      volumes:
      - name: tls
        secret:
          secretName: webhook-server-cert
