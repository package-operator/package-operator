package hostedclusterpackages

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

type partitioningReconciler struct {
	client         client.Client
	log            logr.Logger
	scheme         *runtime.Scheme
	partReconciler *partitionReconciler
}

type partition struct {
	labelValue     string
	hostedClusters []v1beta1.HostedCluster
}

func (r *partitioningReconciler) Reconcile(
	ctx context.Context, hostedClusterPackage *corev1alpha1.HostedClusterPackage,
) (ctrl.Result, error) {
	partitions, err := r.listPartitions(ctx, hostedClusterPackage)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing partitions: %w", err)
	}

	for _, part := range partitions {
		res, err := r.partReconciler.Reconcile(ctx, hostedClusterPackage, part)
		if err != nil || !res.IsZero() || !isUpdatedAndAvailable(hostedClusterPackage, part.labelValue) {
			// Don't reconcile next partition if current one hasn't successfully updated.
			return res, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *partitioningReconciler) listPartitions(
	ctx context.Context, hostedClusterPackage *corev1alpha1.HostedClusterPackage,
) ([]partition, error) {
	hostedClusters := &v1beta1.HostedClusterList{}
	if err := r.client.List(ctx, hostedClusters, client.InNamespace("default")); err != nil {
		return nil, fmt.Errorf("listing clusters: %w", err)
	}

	if hostedClusterPackage.Spec.Strategy.Partition == nil {
		return []partition{{labelValue: "*", hostedClusters: hostedClusters.Items}}, nil
	}

	partitionsMap := make(map[string][]v1beta1.HostedCluster)
	for _, hc := range hostedClusters.Items {
		label := hc.GetLabels()[hostedClusterPackage.Spec.Strategy.Partition.LabelKey]
		if len(label) == 0 {
			label = "*"
		}
		partitionsMap[label] = append(partitionsMap[label], hc)
	}

	partitions := make([]partition, 0, len(partitionsMap))
	for label, hcList := range partitionsMap {
		partitions = append(partitions, partition{
			labelValue:     label,
			hostedClusters: hcList,
		})
	}

	return r.sortedPartitions(hostedClusterPackage, partitions), nil
}

func (r *partitioningReconciler) sortedPartitions(
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
	partitions []partition,
) []partition {
	if hostedClusterPackage.Spec.Strategy.Partition.Order == nil ||
		len(hostedClusterPackage.Spec.Strategy.Partition.Order.Static) == 0 {
		slices.SortFunc(partitions, func(a partition, b partition) int {
			return strings.Compare(strings.ToLower(a.labelValue), strings.ToLower(b.labelValue))
		})
		return partitions
	} else {
		// TODO: static ordering
	}

	return partitions
}

func isUpdatedAndAvailable(hostedClusterPackage *corev1alpha1.HostedClusterPackage, partitionName string) bool {
	for _, partStatus := range hostedClusterPackage.Status.Partitions {
		if partStatus.Name == partitionName {
			return partStatus.UpdatedPackages == partStatus.Packages && partStatus.AvailablePackages == partStatus.Packages
		}
	}
	panic("Developer error: partition not found")
}
