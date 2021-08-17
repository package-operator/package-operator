apiVersion: v1
kind: Service
metadata:
  name: webhook-service
  namespace: addon-operator
spec:
  ports:
    - port: 443
      targetPort: 8080
  selector:
    app.kubernetes.io/name: addon-operator-webook-server
