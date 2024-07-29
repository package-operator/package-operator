package presets

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var clusterScopedGK = map[schema.GroupKind]struct{}{
	{Kind: "Namespace"}:        {},
	{Kind: "IngressClass"}:     {},
	{Kind: "PersistentVolume"}: {},

	{Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"}:        {},
	{Kind: "ClusterRoleBinding", Group: "rbac.authorization.k8s.io"}: {},

	{Kind: "PriorityClass", Group: "scheduling.k8s.io"}:   {},
	{Kind: "APIService", Group: "apiregistration.k8s.io"}: {},

	{Kind: "StorageClass", Group: "storage.k8s.io"}:       {},
	{Kind: "CSIDriver", Group: "storage.k8s.io"}:          {},
	{Kind: "CSINode", Group: "storage.k8s.io"}:            {},
	{Kind: "CSIStorageCapacity", Group: "storage.k8s.io"}: {},

	{Kind: "MutatingWebhookConfiguration", Group: "admissionregistration.k8s.io"}:   {},
	{Kind: "ValidatingWebhookConfiguration", Group: "admissionregistration.k8s.io"}: {},
	{Kind: "ValidatingAdmissionPolicy", Group: "admissionregistration.k8s.io"}:      {},
}

func isClusterScoped(obj unstructured.Unstructured) bool {
	_, ok := clusterScopedGK[obj.GroupVersionKind().GroupKind()]
	return ok
}
