module github.com/openshift/addon-operator

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/stretchr/testify v1.6.1
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/yaml v1.2.0
)
