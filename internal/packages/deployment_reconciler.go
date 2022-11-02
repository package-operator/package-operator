package packages

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/ownerhandling"
	"package-operator.run/package-operator/internal/utils"
)

// DeploymentReconciler creates or updates an (Cluster)ObjectDeployment.
// Will respect the given chunking strategy to create multiple ObjectSlices.
type DeploymentReconciler struct {
	scheme              *runtime.Scheme
	client              client.Client
	newObjectDeployment genericObjectDeploymentFactory
	newObjectSlice      genericObjectSliceFactory
	ownerStrategy       ownerStrategy
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
	SetControllerReference(owner, obj metav1.Object) error
}

func newDeploymentReconciler(
	scheme *runtime.Scheme,
	client client.Client,
	newObjectDeployment genericObjectDeploymentFactory,
	newObjectSlice genericObjectSliceFactory,
) *DeploymentReconciler {
	return &DeploymentReconciler{
		scheme:              scheme,
		client:              client,
		newObjectDeployment: newObjectDeployment,
		newObjectSlice:      newObjectSlice,
		ownerStrategy:       ownerhandling.NewNative(scheme),
	}
}

func (r *DeploymentReconciler) Reconcile(
	ctx context.Context, desiredDeploy genericObjectDeployment,
	chunker objectChunker,
) error {
	templateSpec := desiredDeploy.GetTemplateSpec()

	// Get existing ObjectDeployment
	actualDeploy := r.newObjectDeployment(r.scheme)
	err := r.client.Get(
		ctx,
		client.ObjectKeyFromObject(desiredDeploy.ClientObject()),
		actualDeploy.ClientObject(),
	)
	if apierrors.IsNotFound(err) {
		// Pre-Create the ObjectDeployment without phases,
		// so we can create Slices with an OwnerRef to the Deployment.
		desiredDeploy.SetTemplateSpec(corev1alpha1.ObjectSetTemplateSpec{})
		if err := r.client.Create(ctx, desiredDeploy.ClientObject()); err != nil {
			return err
		}
		actualDeploy = desiredDeploy
	}
	if err != nil {
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
	desiredDeploy.SetTemplateSpec(templateSpec)

	// Update Deployment
	annotations := mergeKeysFrom(actualDeploy.ClientObject().GetAnnotations(), desiredDeploy.ClientObject().GetAnnotations())
	labels := mergeKeysFrom(actualDeploy.ClientObject().GetLabels(), desiredDeploy.ClientObject().GetLabels())
	actualDeploy.ClientObject().SetAnnotations(annotations)
	actualDeploy.ClientObject().SetLabels(labels)
	actualDeploy.SetTemplateSpec(desiredDeploy.GetTemplateSpec())

	if err := r.client.Update(ctx, actualDeploy.ClientObject()); err != nil {
		return fmt.Errorf("updating ObjectDeployment: %w", err)
	}
	return nil
}

func (r *DeploymentReconciler) chunkPhase(
	ctx context.Context, deploy genericObjectDeployment,
	phase *corev1alpha1.ObjectSetTemplatePhase,
	chunker objectChunker,
) error {
	log := logr.FromContextOrDiscard(ctx)

	objectsForSlices, err := chunker.Chunk(ctx, phase)
	if err != nil {
		return fmt.Errorf("chunking strategy: %w", err)
	}
	if len(objectsForSlices) == 0 {
		return nil
	}

	phase.Objects = nil // Objects now live within the ObjectSlice instances.
	var sliceNames []string
	for _, objectsForSlice := range objectsForSlices {
		slice := r.newObjectSlice(r.scheme)
		slice.ClientObject().SetNamespace(deploy.ClientObject().GetNamespace())
		slice.SetObjects(objectsForSlice)

		if err := r.reconcileSlice(ctx, deploy, slice); err != nil {
			return fmt.Errorf("reconcile ObjectSlice: %w", err)
		}
		sliceNames = append(sliceNames, slice.ClientObject().GetName())

		log.Info("reconciled Slice for phase",
			"ObjectDeployment", client.ObjectKeyFromObject(deploy.ClientObject()),
			"ObjectSlice", client.ObjectKeyFromObject(slice.ClientObject()))
	}
	phase.Slices = sliceNames
	return nil
}

// reconcile ObjectSlice and retry on hash collision.
func (r *DeploymentReconciler) reconcileSlice(
	ctx context.Context, deploy genericObjectDeployment,
	slice genericObjectSlice,
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

type sliceCollisionError struct {
	newSlice         genericObjectSlice
	conflictingSlice genericObjectSlice
}

func (e sliceCollisionError) Error() string {
	newKey := client.ObjectKeyFromObject(e.newSlice.ClientObject())
	conflictingKey := client.ObjectKeyFromObject(e.conflictingSlice.ClientObject())
	return newKey.String() + " collision with " + conflictingKey.String()
}

func (r *DeploymentReconciler) reconcileSliceWithCollisionCount(
	ctx context.Context, deploy genericObjectDeployment,
	slice genericObjectSlice, collisionCount int32,
) error {
	hash := utils.ComputeHash(slice.GetObjects(), &collisionCount)
	name := deploy.ClientObject().GetName() + "-" + hash
	slice.ClientObject().SetName(name)

	if err := r.ownerStrategy.SetControllerReference(deploy.ClientObject(), slice.ClientObject()); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}

	// Try to create slice
	err := r.client.Create(ctx, slice.ClientObject())
	if err == nil {
		return nil
	}
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("errored creating new ObjectSlice: %w", err)
	}

	// There is already a Slice with this name!
	conflictingSlice := r.newObjectSlice(r.scheme)
	if err := r.client.Get(
		ctx, client.ObjectKeyFromObject(slice.ClientObject()),
		conflictingSlice.ClientObject(),
	); err != nil {
		return fmt.Errorf("getting conflicting ObjectSlice: %w", err)
	}
	// object already exists, check for hash collision
	if r.ownerStrategy.IsController(deploy.ClientObject(), slice.ClientObject()) &&
		equality.Semantic.DeepEqual(conflictingSlice.GetObjects(), slice.GetObjects()) {
		// we are owner and object is equal
		// -> all good :)
		return nil
	}

	return &sliceCollisionError{
		newSlice:         slice,
		conflictingSlice: conflictingSlice,
	}
}
