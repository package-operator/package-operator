package packages

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var (
	PackageManifestLockGroupKind = schema.GroupKind{Group: manifestsv1alpha1.GroupVersion.Group, Kind: "PackageManifestLock"}
	PackageManifestGroupKind     = schema.GroupKind{Group: manifestsv1alpha1.GroupVersion.Group, Kind: "PackageManifest"}
)
