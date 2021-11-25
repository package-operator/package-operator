module github.com/openshift/addon-operator

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/gorilla/mux v1.8.0
	github.com/openshift/addon-operator/apis v0.0.0-00010101000000-000000000000
	github.com/openshift/api v0.0.0-20211122204231-b094ceff1955
	github.com/operator-framework/api v0.8.1
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.22.4
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/kubectl v0.22.4
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.10.3
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/openshift/addon-operator/apis => ./apis
