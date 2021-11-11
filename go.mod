module github.com/openshift/addon-operator

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/openshift/addon-operator/apis v0.0.0-00010101000000-000000000000
	github.com/operator-framework/api v0.8.1
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.20.2
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/openshift/addon-operator/apis => ./apis
