package apis

import (
	"github.com/openshift/addon-operator/apis/addons"
)

// AddToScheme adds all api Resources to the Scheme
var AddToScheme = addons.AddToScheme
