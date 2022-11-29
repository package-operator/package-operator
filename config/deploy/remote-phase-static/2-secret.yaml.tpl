apiVersion: v1
kind: Secret
metadata:
  name: admin-kubeconfig
  namespace: package-operator-system
  labels:
    app.kubernetes.io/name: remote-phase-operator
data: # will be filled in in mage file
