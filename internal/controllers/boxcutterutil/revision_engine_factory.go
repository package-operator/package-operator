package boxcutterutil

import "pkg.package-operator.run/boxcutter/managedcache"

type RevisionEngineFactory interface {
	New(accessor managedcache.Accessor)
}
