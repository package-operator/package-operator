apiVersion: apps/v1
kind: Deployment
metadata:
  name: remote-phase-manager
  namespace: package-operator-system
  labels:
    app.kubernetes.io/name: remote-phase-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: remote-phase-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: remote-phase-operator
    spec:
      # serviceAccountName: remote-phase-operator
      volumes:
      - name: kubeconfig
        secret:
          secretName: admin-kubeconfig
          optional: false
      containers:
      - name: manager
        image: quay.io/openshift/remote-phase-manager:latest
        args:
        - --enable-leader-election
        - -target-cluster-kubeconfig-file=/data/kubeconfig
        - -class=hosted-cluster
        volumeMounts:
        - name: kubeconfig
          mountPath: /data
          readOnly: true
        env:
        - name: PKO_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        # livenessProbe:
        #   httpGet:
        #     path: /healthz
        #     port: 8081
        #   initialDelaySeconds: 15
        #   periodSeconds: 20
        # readinessProbe:
        #   httpGet:
        #     path: /readyz
        #     port: 8081
        #   initialDelaySeconds: 5
        #   periodSeconds: 10
        # resources:
        #   limits:
        #     cpu: 100m
        #     memory: 400Mi
        #   requests:
        #     cpu: 100m
        #     memory: 300Mi
