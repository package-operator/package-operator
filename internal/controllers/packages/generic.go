package packages

import (
	pkocore "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/utils"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func RevisionPtr[T pkocore.ClusterPackage | pkocore.Package | pkocore.ClusterObjectDeployment | pkocore.ObjectDeployment](thing *T) *int64 {
	switch concrete := any(thing).(type) {
	case *pkocore.ClusterPackage:
		return &concrete.Status.Revision
	case *pkocore.ClusterObjectDeployment:
		return &concrete.Status.Revision
	case *pkocore.Package:
		return &concrete.Status.Revision
	case *pkocore.ObjectDeployment:
		return &concrete.Status.Revision
	default:
		panic("illegal type")
	}
}

func ConditionsPtr[T pkocore.ClusterPackage | pkocore.Package | pkocore.ClusterObjectDeployment | pkocore.ObjectDeployment](thing *T) *[]meta.Condition {
	switch concrete := any(thing).(type) {
	case *pkocore.ClusterPackage:
		return &concrete.Status.Conditions
	case *pkocore.ClusterObjectDeployment:
		return &concrete.Status.Conditions
	case *pkocore.Package:
		return &concrete.Status.Conditions
	case *pkocore.ObjectDeployment:
		return &concrete.Status.Conditions
	default:
		panic("illegal type")
	}
}

func PackagePhasePtr[P pkocore.ClusterPackage | pkocore.Package](pkg *P) *pkocore.PackageStatusPhase {
	switch concrete := any(pkg).(type) {
	case *pkocore.ClusterPackage:
		return &concrete.Status.Phase
	case *pkocore.Package:
		return &concrete.Status.Phase
	default:
		panic("illegal type")
	}
}

func ObjectDeploymentPhasePtr[D pkocore.ClusterObjectDeployment | pkocore.ObjectDeployment](deployment *D) *pkocore.ObjectDeploymentPhase {
	switch concrete := any(deployment).(type) {
	case *pkocore.ClusterObjectDeployment:
		return &concrete.Status.Phase
	case *pkocore.ObjectDeployment:
		return &concrete.Status.Phase
	default:
		panic("illegal type")
	}
}

func GenericPackage[P pkocore.ClusterPackage | pkocore.Package](pkg P) adapters.GenericPackageAccessor {
	switch concrete := any(pkg).(type) {
	case pkocore.ClusterPackage:
		return &adapters.GenericClusterPackage{ClusterPackage: concrete}
	case pkocore.Package:
		return &adapters.GenericPackage{Package: concrete}
	default:
		panic("illegal type")
	}
}

func PackageUnpackHashPtr[P pkocore.ClusterPackage | pkocore.Package](pkg P) *string {
	switch concrete := any(pkg).(type) {
	case pkocore.ClusterPackage:
		return &concrete.Status.UnpackedHash
	case pkocore.Package:
		return &concrete.Status.UnpackedHash
	default:
		panic("illegal type")
	}
}

func StatusRevisionPtr[T pkocore.ClusterPackage | pkocore.Package | pkocore.ClusterObjectDeployment | pkocore.ObjectDeployment](thing *T) *int64 {
	switch concrete := any(thing).(type) {
	case *pkocore.ClusterPackage:
		return &concrete.Status.Revision
	case *pkocore.ClusterObjectDeployment:
		return &concrete.Status.Revision
	case *pkocore.Package:
		return &concrete.Status.Revision
	case *pkocore.ObjectDeployment:
		return &concrete.Status.Revision
	default:
		panic("illegal type")
	}
}

func PackageImagePtr[P pkocore.ClusterPackage | pkocore.Package](pkg P) *string {
	switch concrete := any(pkg).(type) {
	case pkocore.ClusterPackage:
		return &concrete.Spec.Image
	case pkocore.Package:
		return &concrete.Spec.Image
	default:
		panic("illegal type")
	}
}

func PackageSpecHash[P pkocore.ClusterPackage | pkocore.Package](pkg P, packageHashModifier *int32) string {
	switch concrete := any(pkg).(type) {
	case pkocore.ClusterPackage:
		return utils.ComputeSHA256Hash(concrete.Spec, packageHashModifier)
	case pkocore.Package:
		return utils.ComputeSHA256Hash(concrete.Spec, packageHashModifier)
	default:
		panic("illegal type")
	}
}
