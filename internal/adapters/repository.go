package adapters

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/utils"
)

type GenericRepositoryAccessor interface {
	ClientObject() client.Object
	IsNamespaced() bool
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetImage() string
	GetSpecHash(repositoryHashModifier *int32) string
	GetUnpackedHash() string
	SetUnpackedHash(hash string)
	setStatusPhase(phase corev1alpha1.RepositoryStatusPhase)
}

type GenericRepositoryFactory func(scheme *runtime.Scheme) GenericRepositoryAccessor

var (
	repositoryGVK        = corev1alpha1.GroupVersion.WithKind("Repository")
	clusterRepositoryGVK = corev1alpha1.GroupVersion.WithKind("ClusterRepository")
)

func NewGenericRepository(scheme *runtime.Scheme) GenericRepositoryAccessor {
	obj, err := scheme.New(repositoryGVK)
	if err != nil {
		panic(err)
	}

	return &GenericRepository{
		Repository: *obj.(*corev1alpha1.Repository),
	}
}

func NewGenericClusterRepository(scheme *runtime.Scheme) GenericRepositoryAccessor {
	obj, err := scheme.New(clusterRepositoryGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterRepository{
		ClusterRepository: *obj.(*corev1alpha1.ClusterRepository),
	}
}

var (
	_ GenericRepositoryAccessor = (*GenericRepository)(nil)
	_ GenericRepositoryAccessor = (*GenericClusterRepository)(nil)
)

type GenericRepository struct {
	corev1alpha1.Repository
}

func (a *GenericRepository) ClientObject() client.Object {
	return &a.Repository
}

func (a *GenericRepository) IsNamespaced() bool {
	return true
}

func (a *GenericRepository) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericRepository) UpdatePhase() {
	updateRepositoryPhase(a)
}

func (a *GenericRepository) GetImage() string {
	return a.Spec.Image
}

func (a *GenericRepository) GetSpecHash(repositoryHashModifier *int32) string {
	return utils.ComputeSHA256Hash(a.Spec, repositoryHashModifier)
}

func (a *GenericRepository) SetUnpackedHash(hash string) {
	a.Status.UnpackedHash = hash
}

func (a *GenericRepository) GetUnpackedHash() string {
	return a.Status.UnpackedHash
}

func (a *GenericRepository) setStatusPhase(phase corev1alpha1.RepositoryStatusPhase) {
	a.Status.Phase = phase
}

type GenericClusterRepository struct {
	corev1alpha1.ClusterRepository
}

func (a *GenericClusterRepository) ClientObject() client.Object {
	return &a.ClusterRepository
}

func (a *GenericClusterRepository) IsNamespaced() bool {
	return false
}

func (a *GenericClusterRepository) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterRepository) UpdatePhase() {
	updateRepositoryPhase(a)
}

func (a *GenericClusterRepository) GetImage() string {
	return a.Spec.Image
}

func (a *GenericClusterRepository) GetSpecHash(repositoryHashModifier *int32) string {
	return utils.ComputeSHA256Hash(a.Spec, repositoryHashModifier)
}

func (a *GenericClusterRepository) setStatusPhase(phase corev1alpha1.RepositoryStatusPhase) {
	a.Status.Phase = phase
}

func (a *GenericClusterRepository) SetUnpackedHash(hash string) {
	a.Status.UnpackedHash = hash
}

func (a *GenericClusterRepository) GetUnpackedHash() string {
	return a.Status.UnpackedHash
}

func updateRepositoryPhase(pkg GenericRepositoryAccessor) {
	var filteredCond []metav1.Condition
	for _, c := range *pkg.GetConditions() {
		if c.ObservedGeneration == pkg.ClientObject().GetGeneration() {
			filteredCond = append(filteredCond, c)
		}
	}

	if meta.IsStatusConditionTrue(filteredCond, corev1alpha1.RepositoryInvalid) {
		pkg.setStatusPhase(corev1alpha1.RepositoryPhaseInvalid)
		return
	}

	unpackCond := meta.FindStatusCondition(filteredCond, corev1alpha1.RepositoryUnpacked)
	if unpackCond == nil {
		pkg.setStatusPhase(corev1alpha1.RepositoryPhaseUnpacking)
		return
	}

	if meta.IsStatusConditionTrue(
		filteredCond,
		corev1alpha1.RepositoryAvailable,
	) {
		pkg.setStatusPhase(corev1alpha1.RepositoryPhaseAvailable)
		return
	}

	pkg.setStatusPhase(corev1alpha1.RepositoryPhaseNotReady)
}
