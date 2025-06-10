package packagedeploy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/equality"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/utils"

	"pkg.package-operator.run/boxcutter/ownerhandling"
)

type (
	sliceCollisionError struct {
		key client.ObjectKey
	}

	ownerStrategy interface {
		IsController(owner, obj metav1.Object) bool
		SetControllerReference(owner, obj metav1.Object) error
	}
)

const sliceOwnerLabel = "slices.package-operator.run/owner"

// DeploymentReconciler creates or updates an (Cluster)ObjectDeployment.
// Will respect the given chunking strategy to create multiple ObjectSlices.
type DeploymentReconciler struct {
	scheme              *runtime.Scheme
	client              client.Client
	newObjectDeployment adapters.ObjectDeploymentFactory
	newObjectSlice      adapters.ObjectSliceFactory
	newObjectSliceList  adapters.ObjectSliceListFactory
	newObjectSetList    genericObjectSetListFactory
	ownerStrategy       ownerStrategy
}

func newDeploymentReconciler(
	scheme *runtime.Scheme,
	client client.Client,
	newObjectDeployment adapters.ObjectDeploymentFactory,
	newObjectSlice adapters.ObjectSliceFactory,
	newObjectSliceList adapters.ObjectSliceListFactory,
	newObjectSetList genericObjectSetListFactory,
) *DeploymentReconciler {
	return &DeploymentReconciler{
		scheme:              scheme,
		client:              client,
		newObjectDeployment: newObjectDeployment,
		newObjectSlice:      newObjectSlice,
		newObjectSliceList:  newObjectSliceList,
		newObjectSetList:    newObjectSetList,
		ownerStrategy:       ownerhandling.NewNative(scheme),
	}
}

func (r *DeploymentReconciler) Reconcile(
	ctx context.Context, desiredDeploy adapters.ObjectDeploymentAccessor, chunker objectChunker,
) error {
	templateSpec := desiredDeploy.GetSpecTemplateSpec()

	// Get existing ObjectDeployment
	actualDeploy := r.newObjectDeployment(r.scheme)
	err := r.client.Get(ctx, client.ObjectKeyFromObject(desiredDeploy.ClientObject()), actualDeploy.ClientObject())
	if apimachineryerrors.IsNotFound(err) {
		// Pre-Create the ObjectDeployment without phases,
		// so we can create Slices with an OwnerRef to the Deployment.
		desiredDeploy.SetSpecTemplateSpec(corev1alpha1.ObjectSetTemplateSpec{})
		if err := r.client.Create(ctx, desiredDeploy.ClientObject()); err != nil {
			return err
		}
		actualDeploy = desiredDeploy
	} else if err != nil {
		return fmt.Errorf("getting ObjectDeployment: %w", err)
	}

	// ObjectSlices
	for i := range templateSpec.Phases {
		phase := &templateSpec.Phases[i]
		err := r.chunkPhase(ctx, actualDeploy, phase, chunker)
		if err != nil {
			return fmt.Errorf("reconcile phase: %w", err)
		}
	}

	// Update Deployment
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		annotations := labels.Merge(
			actualDeploy.ClientObject().GetAnnotations(),
			desiredDeploy.ClientObject().GetAnnotations(),
		)
		annotations[constants.ChangeCauseAnnotation] = getChangeCause(actualDeploy, desiredDeploy)
		actualDeploy.ClientObject().SetAnnotations(annotations)

		labels := labels.Merge(
			actualDeploy.ClientObject().GetLabels(),
			desiredDeploy.ClientObject().GetLabels(),
		)
		actualDeploy.ClientObject().SetLabels(labels)

		actualDeploy.SetSpecTemplateSpec(templateSpec)

		err := r.client.Update(ctx, actualDeploy.ClientObject())
		if err == nil {
			return nil
		}

		if apimachineryerrors.IsConflict(err) {
			// Get latest version of the ObjectDeployment to resolve conflict.
			if err := r.client.Get(
				ctx,
				client.ObjectKeyFromObject(desiredDeploy.ClientObject()),
				actualDeploy.ClientObject(),
			); err != nil {
				return fmt.Errorf("getting ObjectDeployment to resolve conflict: %w", err)
			}
		}
		return fmt.Errorf("updating ObjectDeployment: %w", err)
	})
	if err != nil {
		return err
	}

	if err := r.sliceGarbageCollection(ctx, actualDeploy); err != nil {
		return fmt.Errorf("slice garbage collection: %w", err)
	}
	return nil
}

// GarbageCollect Slices that are no longer referenced.
func (r *DeploymentReconciler) sliceGarbageCollection(
	ctx context.Context, deploy adapters.ObjectDeploymentAccessor,
) error {
	objectSets, err := r.listObjectSetsForDeployment(ctx, deploy)
	if err != nil {
		return fmt.Errorf("listing Deployments ObjectSets for GC evaluation: %w", err)
	}

	referencedSlices := map[string]struct{}{}
	for _, phase := range deploy.GetSpecTemplateSpec().Phases {
		for _, slice := range phase.Slices {
			referencedSlices[slice] = struct{}{}
		}
	}
	for _, objectSet := range objectSets {
		for _, phase := range objectSet.GetSpecPhases() {
			for _, slice := range phase.Slices {
				referencedSlices[slice] = struct{}{}
			}
		}
	}

	// List all Slices controlled by this Deployment.
	controlledSlicesList := r.newObjectSliceList(r.scheme)
	if err := r.client.List(
		ctx, controlledSlicesList.ClientObjectList(),
		client.MatchingLabels{
			sliceOwnerLabel: deploy.ClientObject().GetName(),
		},
		client.InNamespace(
			deploy.ClientObject().GetNamespace()),
	); err != nil {
		return fmt.Errorf("listing all controlled slices: %w", err)
	}

	// Delete Slices not referenced anymore.
	for _, slice := range controlledSlicesList.GetItems() {
		if _, referenced := referencedSlices[slice.ClientObject().GetName()]; referenced {
			continue
		}

		// Slice is not referenced anymore.
		if err := r.client.Delete(ctx, slice.ClientObject()); err != nil {
			return fmt.Errorf("garbage collect ObjectSlice: %w", err)
		}
	}

	return nil
}

func (r *DeploymentReconciler) listObjectSetsForDeployment(
	ctx context.Context, deploy adapters.ObjectDeploymentAccessor,
) ([]genericObjectSet, error) {
	labelSelector := deploy.GetSpecSelector()
	objectSetSelector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid selector: %w", err)
	}

	objectSetList := r.newObjectSetList(r.scheme)
	if err := r.client.List(
		ctx, objectSetList.ClientObjectList(),
		client.MatchingLabelsSelector{
			Selector: objectSetSelector,
		},
		client.InNamespace(deploy.ClientObject().GetNamespace()),
	); err != nil {
		return nil, fmt.Errorf("listing ObjectSets: %w", err)
	}

	items := objectSetList.GetItems()
	return items, nil
}

func (r *DeploymentReconciler) chunkPhase(
	ctx context.Context, deploy adapters.ObjectDeploymentAccessor,
	phase *corev1alpha1.ObjectSetTemplatePhase, chunker objectChunker,
) error {
	log := ctrl.LoggerFrom(ctx)

	objectsForSlices, err := chunker.Chunk(ctx, phase)
	if err != nil {
		return fmt.Errorf("chunking strategy: %w", err)
	}
	if len(objectsForSlices) == 0 {
		log.Info("no chunking taking place", "phase", phase.Name)
		return nil
	}

	phase.Objects = nil // Objects now live within the ObjectSlice instances.
	sliceNames := make([]string, len(objectsForSlices))
	for i, objectsForSlice := range objectsForSlices {
		slice := r.newObjectSlice(r.scheme)
		slice.ClientObject().SetNamespace(deploy.ClientObject().GetNamespace())
		slice.ClientObject().SetLabels(map[string]string{
			sliceOwnerLabel: deploy.ClientObject().GetName(),
		})
		slice.SetObjects(objectsForSlice)

		if err := r.reconcileSlice(ctx, deploy, slice); err != nil {
			return fmt.Errorf("reconcile ObjectSlice: %w", err)
		}
		sliceNames[i] = slice.ClientObject().GetName()

		log.Info("reconciled Slice for phase",
			"phase", phase.Name,
			"ObjectDeployment", client.ObjectKeyFromObject(deploy.ClientObject()),
			"ObjectSlice", client.ObjectKeyFromObject(slice.ClientObject()))
	}
	phase.Slices = sliceNames
	return nil
}

// reconcile ObjectSlice and retry on hash collision.
func (r *DeploymentReconciler) reconcileSlice(
	ctx context.Context, deploy adapters.ObjectDeploymentAccessor, slice adapters.ObjectSliceAccessor,
) error {
	var collisionCount int32
	for {
		err := r.reconcileSliceWithCollisionCount(ctx, deploy, slice, collisionCount)
		var collisionError *sliceCollisionError
		if errors.As(err, &collisionError) {
			collisionCount++

			continue
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func (e sliceCollisionError) Error() string { return "ObjectSlice collision with " + e.key.String() }

func (r *DeploymentReconciler) reconcileSliceWithCollisionCount(
	ctx context.Context, deploy adapters.ObjectDeploymentAccessor,
	slice adapters.ObjectSliceAccessor, collisionCount int32,
) error {
	hash := utils.ComputeFNV32Hash(slice.GetObjects(), &collisionCount)
	name := deploy.ClientObject().GetName() + "-" + hash
	slice.ClientObject().SetName(name)

	// controller ref, so Slices get auto garbage collected when the Deployment get's deleted.
	if err := r.ownerStrategy.SetControllerReference(deploy.ClientObject(), slice.ClientObject()); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}

	// Try to create slice
	err := r.client.Create(ctx, slice.ClientObject())
	if err == nil {
		return nil
	}

	if !apimachineryerrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating new ObjectSlice: %w", err)
	}

	// There is already a Slice with this name!
	conflictingSlice := r.newObjectSlice(r.scheme)
	sliceKey := client.ObjectKeyFromObject(slice.ClientObject())
	if err := r.client.Get(
		ctx, sliceKey,
		conflictingSlice.ClientObject(),
	); err != nil {
		return fmt.Errorf("getting conflicting ObjectSlice: %w", err)
	}
	// object already exists, check for hash collision
	isController := r.ownerStrategy.IsController(deploy.ClientObject(), conflictingSlice.ClientObject())
	isEqual := equality.Semantic.DeepEqual(conflictingSlice.GetObjects(), slice.GetObjects())
	if isController && isEqual {
		// we are controller and object is equal
		// -> all good, just a slow cache :)
		return nil
	}

	log := ctrl.LoggerFrom(ctx)
	log.Info(
		"ObjectSlice hash collision",
		"ObjectSlice", sliceKey,
		"isController", isController,
		"isEqual", isEqual,
		"collisionCount", collisionCount,
	)

	return &sliceCollisionError{
		key: sliceKey,
	}
}

func getChangeCause(
	actualObjectDeployment, desiredObjectDeployment adapters.ObjectDeploymentAccessor,
) string {
	actualAnnotations := actualObjectDeployment.ClientObject().GetAnnotations()
	desiredAnnotations := desiredObjectDeployment.ClientObject().GetAnnotations()

	actualSourceImage := actualAnnotations[manifestsv1alpha1.PackageSourceImageAnnotation]
	desiredSourceImage := desiredAnnotations[manifestsv1alpha1.PackageSourceImageAnnotation]

	actualConfig := actualAnnotations[manifestsv1alpha1.PackageConfigAnnotation]
	desiredConfig := desiredAnnotations[manifestsv1alpha1.PackageConfigAnnotation]

	var changes []string
	if actualSourceImage != desiredSourceImage {
		changes = append(changes, "source image")
	}
	if actualConfig != desiredConfig {
		changes = append(changes, "config")
	}

	if len(changes) == 0 {
		// retain old message.
		return actualAnnotations[constants.ChangeCauseAnnotation]
	}

	return fmt.Sprintf("Package %s changed.", strings.Join(changes, " and "))
}
