package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packagestructure"
)

const (
	packageOperatorClusterPackageName   = "package-operator"
	packageOperatorPackageCheckInterval = 2 * time.Second
)

func runBootstrap(log logr.Logger, scheme *runtime.Scheme, opts opts) error {
	loader := packagestructure.NewLoader(scheme)

	ctx := logr.NewContext(context.Background(), log.WithName("bootstrap"))
	packgeContent, err := loader.LoadFromPath(ctx, "/package")
	if err != nil {
		return err
	}

	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	templateSpec := packgeContent.ToTemplateSpec()

	// Install CRDs or the manager wont start
	crdGK := schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}
	for _, phase := range templateSpec.Phases {
		for _, obj := range phase.Objects {
			gk := obj.Object.GetObjectKind().GroupVersionKind().GroupKind()
			if gk != crdGK {
				continue
			}

			crd := &obj.Object

			// Set cache label
			labels := crd.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels[controllers.DynamicCacheLabel] = "True"
			crd.SetLabels(labels)

			if err := c.Create(ctx, crd); err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	packageOperatorPackage := &corev1alpha1.ClusterPackage{}
	err = c.Get(ctx, client.ObjectKey{
		Name: "package-operator",
	}, packageOperatorPackage)
	if err == nil &&
		meta.IsStatusConditionTrue(packageOperatorPackage.Status.Conditions, corev1alpha1.PackageAvailable) {
		// Package Operator is already installed
		log.Info("Package Operator already installed, updating via in-cluster Package Operator")
		packageOperatorPackage.Spec.Image = opts.selfBootstrap
		return c.Update(ctx, packageOperatorPackage)
	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error looking up Package Operator ClusterPackage: %w", err)
	}

	log.Info("Package Operator NOT Available, self-bootstrapping")

	if err == nil {
		// Cluster Package already present but broken for some reason.
		// Ensure clean install by re-creating ClusterPackage.
		if err := forcedCleanup(ctx, c, packageOperatorPackage); err != nil {
			return fmt.Errorf("forced cleanup: %w", err)
		}
	}

	// Create ClusterPackage Object
	// Create PackageOperator ClusterPackage
	packageOperatorPackage = &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: packageOperatorClusterPackageName,
		},
		Spec: corev1alpha1.PackageSpec{
			Image: opts.selfBootstrap,
		},
	}
	if err := c.Create(ctx, packageOperatorPackage); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("creating Package Operator ClusterPackage: %w", err)
	}

	// Force Adoption of objects during initial bootstrap to take ownership of
	// CRDs, Namespace, ServiceAccount and ClusterRoleBinding.
	if err := os.Setenv("PKO_FORCE_ADOPTION", "1"); err != nil {
		return err
	}

	// Wait till PKO is ready.
	go func() {
		err := wait.PollImmediateUntilWithContext(
			ctx, packageOperatorPackageCheckInterval,
			func(ctx context.Context) (done bool, err error) {
				packageOperatorPackage := &corev1alpha1.ClusterPackage{}
				err = c.Get(ctx, client.ObjectKey{Name: packageOperatorClusterPackageName}, packageOperatorPackage)
				if err != nil {
					return done, err
				}

				if meta.IsStatusConditionTrue(packageOperatorPackage.Status.Conditions, corev1alpha1.PackageAvailable) {
					return true, nil
				}
				return false, nil
			})
		if err != nil {
			panic(err)
		}

		log.Info("Package Operator bootstrapped successfully!")
		os.Exit(0)
	}()

	// Run Manager until it has bootstrapped itself.
	return runManager(log, scheme, opts)
}

func forcedCleanup(
	ctx context.Context, c client.Client,
	packageOperatorPackage *corev1alpha1.ClusterPackage,
) error {
	log := logr.FromContextOrDiscard(ctx)
	if err := c.Delete(ctx, packageOperatorPackage); err != nil {
		return fmt.Errorf("deleting stuck PackageOperator ClusterPackage: %w", err)
	}
	if len(packageOperatorPackage.Finalizers) > 0 {
		packageOperatorPackage.Finalizers = []string{}
		if err := c.Update(ctx, packageOperatorPackage); err != nil {
			return fmt.Errorf("releasing finalizers on stuck PackageOperator ClusterPackage: %w", err)
		}
	}
	log.Info("deleted ClusterPackage", "obj", packageOperatorPackage)
	if err := c.Get(
		ctx, client.ObjectKeyFromObject(packageOperatorPackage), packageOperatorPackage,
	); !errors.IsNotFound(err) {
		return fmt.Errorf("ensuring ClusterPackage is gone: %w", err)
	}

	// Also nuke all the ClusterObjectDeployment belonging to it.
	clusterObjectDeploymentList := &corev1alpha1.ClusterObjectDeploymentList{}
	if err := c.List(ctx, clusterObjectDeploymentList, client.MatchingLabels{
		"package-operator.run/instance": packageOperatorClusterPackageName,
		"package-operator.run/package":  packageOperatorClusterPackageName,
	}); err != nil {
		return fmt.Errorf("listing stuck PackageOperator ClusterObjectDeployments: %w", err)
	}
	for i := range clusterObjectDeploymentList.Items {
		clusterObjectDeployment := &clusterObjectDeploymentList.Items[i]
		if err := c.Delete(ctx, clusterObjectDeployment); err != nil {
			return fmt.Errorf("deleting stuck PackageOperator ClusterObjectDeployment: %w", err)
		}
		if len(clusterObjectDeployment.Finalizers) > 0 {
			clusterObjectDeployment.Finalizers = []string{}
			if err := c.Update(ctx, clusterObjectDeployment); err != nil {
				return fmt.Errorf("releasing finalizers on stuck PackageOperator ClusterObjectDeployment: %w", err)
			}
		}
		log.Info("deleted ClusterObjectDeployment", "name", clusterObjectDeployment.Name, "obj", clusterObjectDeployment)
		if err := c.Get(
			ctx, client.ObjectKeyFromObject(clusterObjectDeployment), clusterObjectDeployment,
		); !errors.IsNotFound(err) {
			return fmt.Errorf("ensuring ClusterObjectDeployment is gone: %w", err)
		}
	}

	// Also nuke all the ClusterObjectSets belonging to it.
	clusterObjectSetList := &corev1alpha1.ClusterObjectSetList{}
	if err := c.List(ctx, clusterObjectSetList, client.MatchingLabels{
		"package-operator.run/instance": packageOperatorClusterPackageName,
		"package-operator.run/package":  packageOperatorClusterPackageName,
	}); err != nil {
		return fmt.Errorf("listing stuck PackageOperator ClusterObjectSets: %w", err)
	}
	for i := range clusterObjectSetList.Items {
		clusterObjectSet := &clusterObjectSetList.Items[i]
		if err := c.Delete(ctx, clusterObjectSet); err != nil {
			return fmt.Errorf("deleting stuck PackageOperator ClusterObjectSet: %w", err)
		}
		if len(clusterObjectSet.Finalizers) > 0 {
			clusterObjectSet.Finalizers = []string{}
			if err := c.Update(ctx, clusterObjectSet); err != nil {
				return fmt.Errorf("releasing finalizers on stuck PackageOperator ClusterObjectSet: %w", err)
			}
		}
		log.Info("deleted ClusterObjectSet", "name", clusterObjectSet.Name, "obj", clusterObjectSet)
		if err := c.Get(
			ctx, client.ObjectKeyFromObject(clusterObjectSet), clusterObjectSet,
		); !errors.IsNotFound(err) {
			return fmt.Errorf("ensuring ClusterObjectSet is gone: %w", err)
		}
	}

	return nil
}
