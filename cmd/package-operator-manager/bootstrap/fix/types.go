package fix

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Context struct {
	Client       client.Client
	Log          logr.Logger
	PKONamespace string
}
