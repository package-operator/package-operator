apiVersion: v1
kind: Service
metadata:
  name: webhook-service
  namespace: package-operator-system
spec:
  ports:
    - port: 443
      targetPort: 8080
  selector:
    app.kubernetes.io/name: package-operator-webook-server
