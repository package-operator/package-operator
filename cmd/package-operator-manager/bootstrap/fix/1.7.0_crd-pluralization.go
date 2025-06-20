package fix

import (
	"context"
	"fmt"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"pkg.package-operator.run/cardboard/kubeutils/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

/*
This fix cleans up the singular version of two pluralized CRDs.
* An upgrade [1] to the controller-runtime dependency also upgraded the code generators that generate our
* CustomResourceDefinition manifests from the apis package. This upgrade fixed a pluralization library used
* in the generators, which in turn 'fixed' the `plural` names of the CRDs `ClusterObjectSlices` and
* `ObjectSlices` and appended a missing 's' to both names.
* - `objectslice` -> `objectslices`
* - `clusterobjectslice` -> `clusterobjectslices`
*
* Upgrading from an existing PKO installation with the old (singular) CRDs in place to a new version with
* the new (pluralized) CRDs results in a broken state because the old singular CRDs must be removed from the
* apiserver for the new pluralized CRDs to become ready. PKO can't handle this situation when it tries to
* upgrade itself and the ClusterObjectSet for PKO's new version will never be able to adopt all PKO resources,
* thus stalling the upgrade, leaving the old PKO version running indefinitely.
* Luckily the old singular CRDs can just be deleted because PKO is not using this api yet.
*
* Rough overview of the steps this fix takes to rectify the upgrade:
* - Needs package-operator-manager to be stopped (which the take-over bootstrap procedure already handles).
* - Remove all ClusterObjectSets under the package-operator ClusterPackage with propagation policy set to
*   `Orphan`, thus leaving all children objects intact. This is really important to avoid the loss of every
*   installed package in the cluster.
* - Remove the singular CRDs.
* - Continue with the bootstrap process which will recreate a ClusterObjectSet, adopt all PKO resources
*   and spin up a fresh PKO deployment which then takes over.

* [1] https://github.com/package-operator/package-operator/commit/928fbace122d35d4d5002d740d7d6ca14248f315
*/
type CRDPluralizationFix struct{}

var (
	pkoClusterObjectSetLabelSelector = manifestsv1alpha1.PackageLabel + "=package-operator"
	deletionWaitTimeout              = time.Minute
	deletionWaitInterval             = time.Second
)

// Organize references to CRD names together.
var (
	singularObjectSliceCRD        = "objectslice.package-operator.run"
	singularClusterObjectSliceCRD = "clusterobjectslice.package-operator.run"

	// The CRDs that will be removed as part of the fix.
	crdsToBeRemoved = []string{
		singularObjectSliceCRD,
		singularClusterObjectSliceCRD,
	}

	crdPairs = [][2]string{
		{
			singularClusterObjectSliceCRD,
			"clusterobjectslices.package-operator.run",
		},
		{
			singularObjectSliceCRD,
			"objectslices.package-operator.run",
		},
	}
)

func (f CRDPluralizationFix) Check(ctx context.Context, fc Context) (bool, error) {
	// Assert cluster conditions that make fix applicable.
	// This one is easy: when either of (cluster)objectslice(s) CRDs exist in singularized and pluralized form.
	// The fix itself has to make sure to remove this condition as late as possible from the cluster to avoid
	// getting stuck in an "errored fix state" which this check does not detect.

	return f.atleastOneCRDPairExists(ctx, fc, crdPairs)
}

func (f CRDPluralizationFix) Run(ctx context.Context, fc Context) error {
	// ensure ORPHANING deletion of package-operator ClusterObjectSets
	// (the ones that are created under the package-operator Package)
	if err := f.ensureClusterObjectSetsGoneWithOrphansLeft(ctx, fc, pkoClusterObjectSetLabelSelector); err != nil {
		return err
	}

	// remove singular CRDs - this step happens last, to keep the fix
	// condition open until after the actual fix has happened
	for _, crd := range crdsToBeRemoved {
		if err := f.ensureCRDGone(ctx, fc, crd); err != nil {
			return err
		}
	}

	return nil
}

func mustParseLabelSelector(selector string) labels.Selector {
	labelSelector, err := labels.Parse(selector)
	if err != nil {
		panic(fmt.Errorf("must be able to parse label selector string: %s, %w", selector, err))
	}
	return labelSelector
}

func (f CRDPluralizationFix) ensureClusterObjectSetsGoneWithOrphansLeft(
	ctx context.Context, fc Context, labelSelectorString string,
) error {
	// build pkoLabelSelector to match PKO ClusterObjectSets
	pkoLabelSelector := client.MatchingLabelsSelector{
		Selector: mustParseLabelSelector(labelSelectorString),
	}

	// delete all PKO ClusterObjectSets by labelSelector the `Orphan` PropagationPolicy is important here!
	// We don't want kube to clean up all child objects (because that would include ALL PKO CRDs and this
	// would in turn include all installed objects!)
	if err := fc.Client.DeleteAllOf(
		ctx,
		&corev1alpha1.ClusterObjectSet{},
		pkoLabelSelector,
		client.PropagationPolicy(metav1.DeletePropagationOrphan),
	); err != nil {
		return err
	}

	// list all PKO ClusterObjectSets by pkoLabelSelector,
	list := &corev1alpha1.ClusterObjectSetList{}
	if err := fc.Client.List(ctx, list, pkoLabelSelector); err != nil {
		return err
	}

	// for each listed ClusterObjectSet: remove all finalizers and wait for them to be gone.
	for _, cos := range list.Items {
		patch := client.MergeFrom(cos.DeepCopy())
		cos.Finalizers = []string{}
		if err := fc.Client.Patch(ctx, &cos, patch); err != nil {
			return err
		}

		waiter := wait.NewWaiter(fc.Client, fc.Client.Scheme(),
			wait.WithInterval(deletionWaitInterval),
			wait.WithTimeout(deletionWaitTimeout))
		if err := waiter.WaitToBeGone(ctx,
			&cos,
			func(client.Object) (done bool, err error) { return false, nil },
		); err != nil {
			return err
		}
	}

	return nil
}

// Ensures that the CRD `name` is deleted and waits for the object to be fully gone.
func (f CRDPluralizationFix) ensureCRDGone(ctx context.Context, fc Context, name string) error {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := fc.Client.Delete(ctx, crd)
	if apimachineryerrors.IsNotFound(err) {
		// object is already gone
		return nil
	} else if err != nil {
		return err
	}

	// wait for object to be fully deleted
	waiter := wait.NewWaiter(
		fc.Client, fc.Client.Scheme(),
		wait.WithInterval(deletionWaitInterval), wait.WithTimeout(deletionWaitTimeout),
	)
	return waiter.WaitToBeGone(ctx, crd, func(client.Object) (bool, error) { return false, nil })
}

// Checks if at least one of the given CRD pairs exists in the apiserver.
func (f CRDPluralizationFix) atleastOneCRDPairExists(ctx context.Context, fc Context, pairs [][2]string) (bool, error) {
	onePairExistsInBothForms := false

	for _, pair := range pairs {
		pairExists, err := f.crdPairExists(ctx, fc, pair)
		if err != nil {
			return false, err
		}
		if pairExists {
			onePairExistsInBothForms = true
		}
	}

	return onePairExistsInBothForms, nil
}

// Check if two CRDs exist by querying the apiserver.
func (f CRDPluralizationFix) crdPairExists(ctx context.Context, fc Context, crdPair [2]string) (bool, error) {
	pairExists := [2]bool{false, false}

	for i, crdName := range crdPair {
		exists, err := f.crdExists(ctx, fc.Client, crdName)
		if err != nil {
			return false, err
		}

		pairExists[i] = exists
	}

	return pairExists[0] && pairExists[1], nil
}

func (f CRDPluralizationFix) crdExists(ctx context.Context, c client.Client, name string) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := c.Get(ctx, client.ObjectKey{
		Name: name,
	}, crd)

	if apimachineryerrors.IsNotFound(err) {
		return false, nil
	}

	// crd exists if error is nil, otherwise error must be passed
	return err == nil, err
}
