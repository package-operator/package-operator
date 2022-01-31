module github.com/openshift/addon-operator

go 1.16

require (
	github.com/go-logr/logr v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/magefile/mage v1.12.1
	github.com/mt-sre/devkube v0.2.1
	github.com/openshift/addon-operator/apis v0.0.0-20220111092509-93ca25c9359f
	github.com/openshift/api v0.0.0-20211122204231-b094ceff1955
	github.com/operator-framework/api v0.8.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.51.2
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.23.0
	k8s.io/apiextensions-apiserver v0.23.0
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.0
	k8s.io/kubectl v0.22.4
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/openshift/addon-operator/apis => ./apis

// tracks github.com/openshift/prometheus-operator/pkg/apis/monitoring release-4.8
replace github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring => github.com/openshift/prometheus-operator/pkg/apis/monitoring v0.0.0-20210811191557-8f4efab9e7fa
