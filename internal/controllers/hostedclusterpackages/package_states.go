package hostedclusterpackages

import (
	"math"
	"slices"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

const (
	defaultPartitionGroup = "_*_"
)

type packageStates struct {
	// HostedClusterPackage object controlling this state.
	hcpkg *corev1alpha1.HostedClusterPackage
	// UID of HostedCluster objects mapped to Package objects in their namespace.
	hcToPackage map[types.UID]*corev1alpha1.Package
	// UID of Package objects mapped to HostedCluster objects in their namespace.
	packageToHc map[types.UID]*v1beta1.HostedCluster
	// HostedCluster objects selected by the HostedClusterPackage
	// indexed by their own UID.
	hostedClusters map[types.UID]*v1beta1.HostedCluster
	// needsUpdate maps partitions to lists of Packages belonging to that partition.
	needsUpdate map[string][]*corev1alpha1.Package
	// needsUpdateAndUnavailable maps partitions to lists of Packages belonging to that partition.
	needsUpdateAndUnavailable map[string][]*corev1alpha1.Package
	// availablePkgs tracks total number of Packages reporting Available == True.
	availablePkgs int
	// unavailablePkgs tracks total number of Packages not reporting Available == True.
	unavailablePkgs int
	// progressedPkgs tracks total number of Packages reporting Unpacked == True and Progressing == False.
	progressedPkgs int
	// updatedPkgs tracks total number of Packages that have a Spec that matches the HostedClusterPackage template Spec.
	updatedPkgs int
	// pausedPkgs tracks total number of Packages that have Spec.Paused set to true.
	pausedPkgs int
}

func newPackageStates(hcpkg *corev1alpha1.HostedClusterPackage) *packageStates {
	return &packageStates{
		hcToPackage:               map[types.UID]*corev1alpha1.Package{},
		packageToHc:               map[types.UID]*v1beta1.HostedCluster{},
		hostedClusters:            map[types.UID]*v1beta1.HostedCluster{},
		needsUpdate:               map[string][]*corev1alpha1.Package{},
		needsUpdateAndUnavailable: map[string][]*corev1alpha1.Package{},
		hcpkg:                     hcpkg,
	}
}

func (ps *packageStates) Add(hc *v1beta1.HostedCluster, pkg *corev1alpha1.Package) {
	ps.hcToPackage[hc.UID] = pkg
	ps.packageToHc[pkg.UID] = hc
	ps.hostedClusters[hc.UID] = hc

	if isPackageAvailable(pkg) {
		ps.availablePkgs++
	} else {
		ps.unavailablePkgs++
	}

	if isPackageProgressed(pkg) {
		ps.progressedPkgs++
	}

	if isPackagePaused(pkg) {
		ps.pausedPkgs++
	}

	// Check if the Package needs to be updated.
	if equality.Semantic.DeepEqual(pkg.Spec, ps.hcpkg.Spec.Template.Spec) {
		ps.updatedPkgs++
		return
	}

	if isPackageAvailable(pkg) {
		ps.needsUpdate[ps.partitionKey(hc)] = append(ps.needsUpdate[ps.partitionKey(hc)], pkg)
	} else {
		ps.needsUpdateAndUnavailable[ps.partitionKey(hc)] = append(
			ps.needsUpdateAndUnavailable[ps.partitionKey(hc)], pkg)
	}
}

func (ps *packageStates) Missing(hc *v1beta1.HostedCluster) {
	ps.hostedClusters[hc.UID] = hc
	ps.unavailablePkgs++
}

func (ps *packageStates) ListHostedClustersMissingPackage() []v1beta1.HostedCluster {
	var hcMissingPackages []v1beta1.HostedCluster
	for hcUID, hc := range ps.hostedClusters {
		if pkg, ok := ps.hcToPackage[hcUID]; !ok || pkg == nil {
			hcMissingPackages = append(hcMissingPackages, *hc)
		}
	}
	return hcMissingPackages
}

// ListPackagesToUpdate returns Packages that need to be updated.
// It will only return Packages up to the amount the disruption budget allows.
func (ps *packageStates) ListPackagesToUpdate() []corev1alpha1.Package {
	limit := ps.DisruptionBudget()
	if limit == -1 {
		limit = math.MaxInt
	}

	// First update packages already in the processing queue:
	var packages []corev1alpha1.Package
	processingUIDs := map[types.UID]struct{}{}
	for _, processing := range ps.hcpkg.Status.Processing {
		processingUIDs[processing.UID] = struct{}{}
	}
	for _, pkg := range ps.hcToPackage {
		if _, ok := processingUIDs[pkg.GetUID()]; ok {
			packages = append(packages, *pkg)
		}
	}

	// Add additional packages.
	for partitionIdx, partition := range ps.partitionList() {
		for _, pkg := range ps.needsUpdateAndUnavailable[partition] {
			if len(packages) >= limit && partitionIdx > 0 {
				// Only the first partition should update failing packages
				// beyond the disruption budget to enable progression.
				return packages
			}
			if _, ok := processingUIDs[pkg.UID]; ok {
				continue
			}
			packages = append(packages, *pkg)
		}

		for _, pkg := range ps.needsUpdate[partition] {
			if len(packages) >= limit {
				return packages
			}
			if _, ok := processingUIDs[pkg.UID]; ok {
				continue
			}
			packages = append(packages, *pkg)
		}
	}

	return packages
}

func (ps *packageStates) DisruptionBudget() int {
	if ps.hcpkg.Spec.Strategy.Instant != nil ||
		ps.hcpkg.Spec.Strategy.RollingUpgrade == nil {
		// update everything / disruption budget disabled
		return -1
	}

	numToUpdate := ps.hcpkg.Spec.Strategy.RollingUpgrade.MaxUnavailable - ps.unavailablePkgs
	if numToUpdate < 0 {
		return 0
	}
	return numToUpdate
}

func (ps *packageStates) PackageToHostedCluster(pkg *corev1alpha1.Package) *v1beta1.HostedCluster {
	return ps.packageToHc[pkg.UID]
}

func (ps *packageStates) partitionList() []string {
	if ps.hcpkg.Spec.Partition == nil {
		return []string{defaultPartitionGroup}
	}

	if ps.hcpkg.Spec.Partition.Order == nil ||
		ps.hcpkg.Spec.Partition.Order.AlphanumericAsc != nil {
		var partitions []string
		partitionKeys := map[string]struct{}{}
		for partitionGroupKey := range ps.needsUpdate {
			partitionKeys[partitionGroupKey] = struct{}{}
		}
		for partitionGroupKey := range ps.needsUpdateAndUnavailable {
			partitionKeys[partitionGroupKey] = struct{}{}
		}

		for partitionGroupKey := range partitionKeys {
			if partitionGroupKey == defaultPartitionGroup {
				continue // will be added back at the end
			}
			partitions = append(partitions, partitionGroupKey)
		}
		slices.Sort(partitions)
		return append(partitions, defaultPartitionGroup) // default group always comes last
	}

	// must have static ordering
	partitions := make([]string, 0, len(ps.hcpkg.Spec.Partition.Order.Static))
	for _, partitionGroupKey := range ps.hcpkg.Spec.Partition.Order.Static {
		if partitionGroupKey == "*" {
			// special character allows users to pick placement of "default" group.
			partitions = append(partitions, defaultPartitionGroup)
			continue
		}
		partitions = append(partitions, partitionGroupKey)
	}
	return partitions
}

func (ps *packageStates) partitionKey(hc *v1beta1.HostedCluster) string {
	if ps.hcpkg.Spec.Partition == nil ||
		hc.Labels == nil ||
		len(hc.Labels[ps.hcpkg.Spec.Partition.LabelKey]) == 0 {
		return defaultPartitionGroup
	}

	return hc.Labels[ps.hcpkg.Spec.Partition.LabelKey]
}
