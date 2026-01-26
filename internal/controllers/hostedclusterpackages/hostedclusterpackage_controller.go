package hostedclusterpackages

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	v1 "k8s.io/client-go/applyconfigurations/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1acs "package-operator.run/apis/applyconfigurations/core/v1alpha1"
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
		// Ignore not found errors on delete.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !hostedClusterPackage.DeletionTimestamp.IsZero() {
		log.Info("HostedClusterPackage is deleting")
		return ctrl.Result{}, nil
	}

	unstructuredHostedClusterPackage, err := toUnstructured(hostedClusterPackage)
	if err != nil {
		return ctrl.Result{}, err
	}

	packageTemplateApplyConfiguration, err := ExtractPackageTemplateFields(unstructuredHostedClusterPackage)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("extracting package template: %w", err)
	}

	// Plan:
	// 1. Check/Update processing queue to remove up-to-date and available packages.
	// 2. (Re)Create any missing packages.
	// 3. Add new updates - up to maxUnavailable to processing queue
	// 4. Patch/Reconcile Packages in processing queue
	// 5. Report status.

	if err := c.rebuildProcessingQueue(ctx, hostedClusterPackage); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating processing queue: %w", err)
	}

	state, err := c.indexPackageState(ctx, hostedClusterPackage)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("indexing package state: %w", err)
	}

	if err := c.createMissingPackages(
		ctx,
		hostedClusterPackage,
		packageTemplateApplyConfiguration,
		state,
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("creating missing Packages: %w", err)
	}

	if err := c.updatePackages(
		ctx,
		hostedClusterPackage,
		packageTemplateApplyConfiguration,
		state,
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating Packages: %w", err)
	}

	state, err = c.indexPackageState(ctx, hostedClusterPackage)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("indexing package state: %w", err)
	}

	if err := c.updateStatus(ctx, hostedClusterPackage, state); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating HostedClusterPackage status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (c *HostedClusterPackageController) createMissingPackages(
	ctx context.Context,
	hcpkg *corev1alpha1.HostedClusterPackage,
	packageTemplateApplyConfiguration *corev1alpha1acs.PackageTemplateSpecApplyConfiguration,
	state *packageStates,
) error {
	for _, hcMissingPackage := range state.ListHostedClustersMissingPackage() {
		ac := c.constructPackage(hcpkg, packageTemplateApplyConfiguration, &hcMissingPackage)
		if err := c.client.Apply(
			ctx, ac, client.FieldOwner(constants.FieldOwner),
		); err != nil {
			return fmt.Errorf("creating Package: %w", err)
		}
	}
	return nil
}

func (c *HostedClusterPackageController) updatePackages(
	ctx context.Context,
	hcpkg *corev1alpha1.HostedClusterPackage,
	packageTemplateApplyConfiguration *corev1alpha1acs.PackageTemplateSpecApplyConfiguration,
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
		ac := c.constructPackage(
			hcpkg,
			packageTemplateApplyConfiguration,
			state.PackageToHostedCluster(&pkg),
		)
		if err := c.client.Apply(ctx, ac, client.FieldOwner(constants.FieldOwner)); err != nil {
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

// rebuildProcessingQueue looks at the current processing queue checking their Package status.
// Packages that are updated and report Available are removed from the queue to free up slots.
func (c *HostedClusterPackageController) rebuildProcessingQueue(
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

func (c *HostedClusterPackageController) constructPackage(
	hcpkg *corev1alpha1.HostedClusterPackage,
	packageTemplateApplyConfiguration *corev1alpha1acs.PackageTemplateSpecApplyConfiguration,
	hc *v1beta1.HostedCluster,
) *corev1alpha1acs.PackageApplyConfiguration {
	ownerGVK := hcpkg.GroupVersionKind()

	ac := corev1alpha1acs.Package(hcpkg.Name, v1beta1.HostedClusterNamespace(*hc)).
		WithOwnerReferences(v1.OwnerReference().
			WithAPIVersion(ownerGVK.GroupVersion().String()).
			WithKind(ownerGVK.Kind).
			WithName(hcpkg.Name).
			WithUID(hcpkg.UID).
			WithBlockOwnerDeletion(true).
			WithController(true)).
		WithLabels(packageTemplateApplyConfiguration.Labels).
		WithAnnotations(packageTemplateApplyConfiguration.Annotations).
		WithSpec(packageTemplateApplyConfiguration.Spec)

	return ac
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

func (c *HostedClusterPackageController) updateStatus(
	ctx context.Context,
	hcpkg *corev1alpha1.HostedClusterPackage,
	state *packageStates,
) error {
	totalPackages := int32(len(state.hcToPackage))

	if hcpkg.Status.Conditions == nil {
		hcpkg.Status.Conditions = make([]metav1.Condition, 0, 2)
	}

	maxUnavailable := 0
	if hcpkg.Spec.Strategy.RollingUpgrade != nil {
		maxUnavailable = hcpkg.Spec.Strategy.RollingUpgrade.MaxUnavailable
	}
	if state.unavailablePkgs <= maxUnavailable {
		meta.SetStatusCondition(&hcpkg.Status.Conditions, metav1.Condition{
			ObservedGeneration: hcpkg.Generation,
			Type:               corev1alpha1.HostedClusterPackageAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "EnoughPackagesAvailable",
			Message:            fmt.Sprintf("%d/%d packages available.", state.availablePkgs, totalPackages),
		})
	} else {
		meta.SetStatusCondition(&hcpkg.Status.Conditions, metav1.Condition{
			ObservedGeneration: hcpkg.Generation,
			Type:               corev1alpha1.HostedClusterPackageAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "NotEnoughPackagesAvailable",
			Message:            fmt.Sprintf("%d/%d packages available.", state.availablePkgs, totalPackages),
		})
	}

	if state.progressedPkgs == int(totalPackages) {
		meta.SetStatusCondition(&hcpkg.Status.Conditions, metav1.Condition{
			ObservedGeneration: hcpkg.Generation,
			Type:               corev1alpha1.HostedClusterPackageProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             "AllPackagesProgressed",
			Message:            fmt.Sprintf("%d/%d packages progressed.", state.progressedPkgs, totalPackages),
		})
	} else {
		meta.SetStatusCondition(&hcpkg.Status.Conditions, metav1.Condition{
			ObservedGeneration: hcpkg.Generation,
			Type:               corev1alpha1.HostedClusterPackageProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             "NotAllPackagesProgressed",
			Message:            fmt.Sprintf("%d/%d packages progressed.", state.progressedPkgs, totalPackages),
		})
	}

	if state.pausedPkgs > 0 {
		meta.SetStatusCondition(&hcpkg.Status.Conditions, metav1.Condition{
			ObservedGeneration: hcpkg.Generation,
			Type:               corev1alpha1.HostedClusterPackageHasPausedPackage,
			Status:             metav1.ConditionTrue,
			Reason:             "AtleastOnePackagePaused",
			Message:            fmt.Sprintf("%d/%d packages paused.", state.pausedPkgs, totalPackages),
		})
	} else {
		meta.SetStatusCondition(&hcpkg.Status.Conditions, metav1.Condition{
			ObservedGeneration: hcpkg.Generation,
			Type:               corev1alpha1.HostedClusterPackageHasPausedPackage,
			Status:             metav1.ConditionFalse,
			Reason:             "NoPackagePaused",
			Message:            fmt.Sprintf("0/%d packages paused.", totalPackages),
		})
	}

	hcpkg.Status.HostedClusterPackageCountsStatus = corev1alpha1.HostedClusterPackageCountsStatus{
		ObservedGeneration: int32(hcpkg.Generation),
		TotalPackages:      totalPackages,
		AvailablePackages:  int32(state.availablePkgs),
		ProgressedPackages: int32(state.progressedPkgs),
		UpdatedPackages:    int32(state.updatedPkgs),
	}

	return c.client.Status().Update(ctx, hcpkg, client.FieldOwner(constants.FieldOwner))
}
