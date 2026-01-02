package hostedclusterpackages

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"slices"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

type HostedClusterPackageController struct {
	client client.Client
	log    logr.Logger
	scheme *runtime.Scheme
}

func NewHostedClusterPackageController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *HostedClusterPackageController {
	return &HostedClusterPackageController{
		client: c,
		log:    log,
		scheme: scheme,
	}
}

func (c *HostedClusterPackageController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("HostedClusterPackage", req.String())
	defer log.Info("reconciled")

	ctx = logr.NewContext(ctx, log)
	hostedClusterPackage := &corev1alpha1.HostedClusterPackage{}
	if err := c.client.Get(ctx, req.NamespacedName, hostedClusterPackage); err != nil {
		// Ignore not found errors on delete
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !hostedClusterPackage.DeletionTimestamp.IsZero() {
		log.Info("HostedClusterPackage is deleting")
		return ctrl.Result{}, nil
	}

	// Plan:
	// 1. Check/Update processing queue to remove up-to-date and available packages.
	// 2. (Re)Create any missing packages.
	// 3. Add new updates - up to maxUnavailable to processing queue
	// 4. Patch/Reconcile Packages in processing queue
	// 5. Report status.

	if err := c.updateProcessingQueue(ctx, hostedClusterPackage); err != nil {
		return ctrl.Result{}, fmt.Errorf("update processing queue: %w", err)
	}

	state, err := c.indexPackageState(ctx, hostedClusterPackage)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("indexing package state: %w", err)
	}

	if err := c.createMissingPackages(ctx, hostedClusterPackage, state); err != nil {
		return ctrl.Result{}, fmt.Errorf("creating missing Packages: %w", err)
	}

	if err := c.updatePackages(ctx, hostedClusterPackage, state); err != nil {
		return ctrl.Result{}, fmt.Errorf("update Packages: %w", err)
	}

	return ctrl.Result{}, nil
}

func (c *HostedClusterPackageController) createMissingPackages(
	ctx context.Context, hcpkg *corev1alpha1.HostedClusterPackage,
	state *packageStates,
) error {
	for _, hcMissingPackage := range state.ListHostedClustersMissingPackage() {
		pkg, err := c.constructPackage(hcpkg, hcMissingPackage)
		if err != nil {
			return fmt.Errorf("constructing Package: %w", err)
		}
		if err := c.client.Create(
			ctx, pkg, client.FieldOwner(constants.FieldOwner),
		); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("creating Package: %w", err)
		}
	}
	return nil
}

func (c *HostedClusterPackageController) updatePackages(
	ctx context.Context, hcpkg *corev1alpha1.HostedClusterPackage,
	state *packageStates,
) error {
	disruptionBudget := state.DisruptionBudget()
	packagesToUpdate := state.ListPackagesToUpdate()

	if len(packagesToUpdate) == 0 {
		return nil
	}

	hcpkg.Status.Processing = nil

	if disruptionBudget > 0 {
		// Make sure we have our updated processing queue persisted.
		// But don't use the processing queue at all, if we have the disruption budget disabled.
		for _, pkg := range packagesToUpdate {
			hcpkg.Status.Processing = append(hcpkg.Status.Processing, corev1alpha1.HostedClusterPackageRefStatus{
				UID:       pkg.UID,
				Name:      pkg.Name,
				Namespace: pkg.Namespace,
			})
		}
	}

	if err := c.client.Status().Update(ctx, hcpkg, client.FieldOwner(constants.FieldOwner)); err != nil {
		return fmt.Errorf("updating HostedClusterPackage status: %w", err)
	}

	for _, pkg := range packagesToUpdate {
		pkg.Spec = hcpkg.Spec.Template.Spec
		if err := c.client.Update(ctx, &pkg, client.FieldOwner(constants.FieldOwner)); err != nil {
			return fmt.Errorf("updating package: %w", err)
		}
	}

	return nil
}

// indexPackageState lists all HostedClusters targeted by the HostedClusterPackage,
// finds the Package instance that was created for them and indexes everything for
// further processing.
func (c *HostedClusterPackageController) indexPackageState(
	ctx context.Context, hcpkg *corev1alpha1.HostedClusterPackage,
) (*packageStates, error) {
	state := newPackageStates(hcpkg)

	s, err := metav1.LabelSelectorAsSelector(&hcpkg.Spec.HostedClusterSelector)
	if err != nil {
		return nil, fmt.Errorf("parsing label selector: %w", err)
	}

	hostedClusters := &v1beta1.HostedClusterList{}
	if err := c.client.List(ctx, hostedClusters, client.MatchingLabelsSelector{Selector: s}); err != nil {
		return state, fmt.Errorf("listing clusters: %w", err)
	}

	for _, hc := range hostedClusters.Items {
		pkg := &corev1alpha1.Package{}
		err := c.client.Get(ctx, client.ObjectKey{
			Name:      hcpkg.Name,
			Namespace: v1beta1.HostedClusterNamespace(hc),
		}, pkg)
		if err == nil {
			// package found.
			state.Add(&hc, pkg)
			continue
		}
		if errors.IsNotFound(err) {
			// package does not exist.
			state.Missing(&hc)
			continue
		}
		return nil, fmt.Errorf("getting Package for HostedCluster: %w", err)
	}
	return state, nil
}

type packageStates struct {
	// HostedClusterPackage object controlling this state.
	hcpkg *corev1alpha1.HostedClusterPackage
	// UID of HostedCluster objects mapped to Package objects in their namespace.
	hcToPackage map[types.UID]*corev1alpha1.Package
	// HostedCluster objects selected by the HostedClusterPackage
	// indexed by their own UID.
	hostedClusters map[types.UID]*v1beta1.HostedCluster
	// needsUpdate maps partitions to lists of Packages belonging to that partition.
	needsUpdate map[string][]*corev1alpha1.Package
	// unavailablePkgs tracks total number of Packages not reporting Available == True.
	unavailablePkgs int
}

const defaultPartitionGroup = "_*_"

func newPackageStates(hcpkg *corev1alpha1.HostedClusterPackage) *packageStates {
	return &packageStates{
		hcToPackage:    map[types.UID]*corev1alpha1.Package{},
		hostedClusters: map[types.UID]*v1beta1.HostedCluster{},
		needsUpdate:    map[string][]*corev1alpha1.Package{},
		hcpkg:          hcpkg,
	}
}

func (ps *packageStates) Add(hc *v1beta1.HostedCluster, pkg *corev1alpha1.Package) {
	ps.hcToPackage[hc.UID] = pkg
	ps.hostedClusters[hc.UID] = hc

	if !isPackageAvailable(pkg) {
		ps.unavailablePkgs++
	}

	// Check if the Package needs to be updated.
	if equality.Semantic.DeepEqual(pkg.Spec, ps.hcpkg.Spec.Template.Spec) {
		return
	}

	ps.needsUpdate[ps.partitionKey(hc)] = append(ps.needsUpdate[ps.partitionKey(hc)], pkg)
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
	for _, partition := range ps.partitionList() {
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

func (ps *packageStates) partitionList() []string {
	if ps.hcpkg.Spec.Partition == nil {
		return []string{defaultPartitionGroup}
	}

	if ps.hcpkg.Spec.Partition.Order == nil ||
		ps.hcpkg.Spec.Partition.Order.AlphanumericAsc != nil {
		var partitions []string
		for partitionGroupKey := range ps.needsUpdate {
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

// updateProcessingQueue looks at the current processing queue checking their Package status.
// Packages that are updated and report Available are removed from the queue to free up slots.
func (c *HostedClusterPackageController) updateProcessingQueue(
	ctx context.Context, hcpkg *corev1alpha1.HostedClusterPackage,
) error {
	updatedQueue := make([]corev1alpha1.HostedClusterPackageRefStatus, 0, len(hcpkg.Status.Processing))
	for _, processingPkg := range hcpkg.Status.Processing {
		pkg := &corev1alpha1.Package{}
		if err := c.client.Get(ctx, client.ObjectKey{
			Name:      processingPkg.Name,
			Namespace: processingPkg.Namespace,
		}, pkg); err != nil {
			return fmt.Errorf("getting Package in processing: %w", err)
		}

		if pkg.UID != processingPkg.UID {
			// strange...
			// Package must have been deleted and recreated.
			// Don't put this Package back into the queue because
			// it is a completely new and different object.
			continue
		}

		if isPackageAvailable(pkg) && equality.Semantic.DeepEqual(hcpkg.Spec.Template.Spec, pkg.Spec) {
			// Package is available & up-to-date.
			continue
		}

		// Put Package back into the queue.
		updatedQueue = append(updatedQueue, processingPkg)
	}
	hcpkg.Status.Processing = updatedQueue
	return nil
}

func isPackageAvailable(pkg *corev1alpha1.Package) bool {
	cond := meta.FindStatusCondition(
		pkg.Status.Conditions, corev1alpha1.PackageAvailable,
	)
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.ObservedGeneration == pkg.Generation
}

func (c *HostedClusterPackageController) reconcileHostedCluster(
	ctx context.Context,
	clusterPackage *corev1alpha1.HostedClusterPackage,
	hc v1beta1.HostedCluster,
) error {
	log := logr.FromContextOrDiscard(ctx)

	if !meta.IsStatusConditionTrue(hc.Status.Conditions, v1beta1.HostedClusterAvailable) {
		log.Info(fmt.Sprintf("waiting for HostedCluster '%s' to become ready", hc.Name))
		return nil
	}

	pkg, err := c.constructPackage(clusterPackage, hc)
	if err != nil {
		return fmt.Errorf("constructing Package: %w", err)
	}

	existingPkg := &corev1alpha1.Package{}
	err = c.client.Get(ctx, client.ObjectKeyFromObject(pkg), existingPkg)
	if errors.IsNotFound(err) {
		if err := c.client.Create(ctx, pkg); err != nil {
			return fmt.Errorf("creating Package: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting Package: %w", err)
	}

	// Update package if spec is different.
	if !reflect.DeepEqual(existingPkg.Spec, pkg.Spec) {
		existingPkg.Spec = pkg.Spec
		if err := c.client.Update(ctx, existingPkg); err != nil {
			return fmt.Errorf("updating outdated Package: %w", err)
		}
	}

	return nil
}

func (c *HostedClusterPackageController) constructPackage(
	hcpkg *corev1alpha1.HostedClusterPackage,
	hc v1beta1.HostedCluster,
) (*corev1alpha1.Package, error) {
	pkg := &corev1alpha1.Package{
		ObjectMeta: *hcpkg.Spec.Template.ObjectMeta.DeepCopy(),
		Spec:       *hcpkg.Spec.Template.Spec.DeepCopy(),
	}
	pkg.Name = hcpkg.Name
	pkg.Namespace = v1beta1.HostedClusterNamespace(hc)

	if err := controllerutil.SetControllerReference(
		hcpkg, pkg, c.scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference: %w", err)
	}
	return pkg, nil
}

func (c *HostedClusterPackageController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.HostedClusterPackage{}).
		Owns(&corev1alpha1.Package{}).
		Watches(
			&v1beta1.HostedCluster{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
				hcpkgList := &corev1alpha1.HostedClusterPackageList{}
				if err := c.client.List(ctx, hcpkgList); err != nil {
					return nil
				}

				// Enqueue all HostedClusterPackages on HostedCluster change
				requests := make([]reconcile.Request, len(hcpkgList.Items))
				for i, hcpkg := range hcpkgList.Items {
					requests[i] = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: hcpkg.Name,
						},
					}
				}

				return requests
			}),
		).
		Complete(c)
}
