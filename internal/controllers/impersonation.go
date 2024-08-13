package controllers

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// AutoImpersonatingWriterWrapper wraps calls from the client.Writer interface
// into new clients using impersonation depending on the root owner.
type AutoImpersonatingWriterWrapper struct {
	restConfig rest.Config
	scheme     *runtime.Scheme
	// I want a cached client pls!
	reader client.Reader
}

func NewAutoImpersonatingWriter(
	restConfig rest.Config,
	scheme *runtime.Scheme,
	reader client.Reader,
) *AutoImpersonatingWriterWrapper {
	return &AutoImpersonatingWriterWrapper{
		restConfig: restConfig,
		scheme:     scheme,
		reader:     reader,
	}
}

// Interface function so clients can require an impersonation aware client.
func (w *AutoImpersonatingWriterWrapper) Impersonate() {}

func (w *AutoImpersonatingWriterWrapper) Create(
	ctx context.Context, obj client.Object, opts ...client.CreateOption,
) error {
	c, err := w.clientUsingPKOImpersonationSettings(ctx, obj)
	if err != nil {
		return err
	}
	return c.Create(ctx, obj, opts...)
}

func (w *AutoImpersonatingWriterWrapper) Delete(
	ctx context.Context, obj client.Object, opts ...client.DeleteOption,
) error {
	c, err := w.clientUsingPKOImpersonationSettings(ctx, obj)
	if err != nil {
		return err
	}
	return c.Delete(ctx, obj, opts...)
}

func (w *AutoImpersonatingWriterWrapper) Update(
	ctx context.Context, obj client.Object, opts ...client.UpdateOption,
) error {
	c, err := w.clientUsingPKOImpersonationSettings(ctx, obj)
	if err != nil {
		return err
	}
	return c.Update(ctx, obj, opts...)
}

func (w *AutoImpersonatingWriterWrapper) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption,
) error {
	c, err := w.clientUsingPKOImpersonationSettings(ctx, obj)
	if err != nil {
		return err
	}
	return c.Patch(ctx, obj, patch, opts...)
}

func (w *AutoImpersonatingWriterWrapper) DeleteAllOf(
	ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption,
) error {
	c, err := w.clientUsingPKOImpersonationSettings(ctx, obj)
	if err != nil {
		return err
	}
	return c.DeleteAllOf(ctx, obj, opts...)
}

func (w *AutoImpersonatingWriterWrapper) clientUsingPKOImpersonationSettings(
	ctx context.Context, obj client.Object,
) (client.Writer, error) {
	c := w.restConfig // shallow colpy of rest.Config
	ic, err := w.impersonationConfigForObject(ctx, obj)
	if err != nil {
		return nil, err
	}
	c.Impersonate = ic
	return client.New(&c, client.Options{})
}

func (w AutoImpersonatingWriterWrapper) impersonationConfigForObject(
	ctx context.Context,
	obj client.Object,
) (rest.ImpersonationConfig, error) {
	toImpersonate, err := w.getOwner(ctx, obj)
	if err != nil {
		return rest.ImpersonationConfig{}, fmt.Errorf("get owner for: %w", err)
	}

	user, groups := impersonationUserAndGroupsForObject(toImpersonate)
	return rest.ImpersonationConfig{
		UserName: user,
		Groups:   groups,
	}, nil
}

type objectIdentity struct {
	Kind      string
	Name      string
	Namespace string
}

func (w AutoImpersonatingWriterWrapper) getOwner(ctx context.Context, obj client.Object) (objectIdentity, error) {
	for _, ownerRef := range obj.GetOwnerReferences() {
		gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			return objectIdentity{}, err
		}
		if ownerRef.Controller != nil &&
			*ownerRef.Controller &&
			gv.Group == corev1alpha1.GroupVersion.Group {
			// The controller is a Package Operator Object.
			// -> check it's owners.
			robj, err := w.scheme.New(schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind))
			if err != nil {
				return objectIdentity{}, err
			}
			potentialOwner := robj.(client.Object)

			var ns string
			if !strings.HasPrefix(ownerRef.Kind, "Cluster") {
				ns = obj.GetNamespace() // same namespace as self.
			}
			if err := w.reader.Get(ctx, client.ObjectKey{
				Name:      ownerRef.Name,
				Namespace: ns,
			}, potentialOwner); err != nil {
				return objectIdentity{}, err
			}
			// recurse into this function
			return w.getOwner(ctx, potentialOwner)
		}
	}
	return objectIdentity{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      obj.GetObjectKind().GroupVersionKind().Kind,
	}, nil
}

const impersonationPrefix = "pko"

func impersonationUserAndGroupsForObject(oi objectIdentity) (user string, groups []string) {
	var (
		resourceSingular string
		resourcePlural   string
		isClusterScope   bool
	)
	switch oi.Kind {
	case "ObjectSet":
		resourceSingular = "objectset"
		resourcePlural = "objectsets"
	case "ObjectDeployment":
		resourceSingular = "objectdeployment"
		resourcePlural = "objectdeployments"
	case "Package":
		resourceSingular = "package"
		resourcePlural = "packages"
	case "ClusterObjectSet":
		resourceSingular = "clusterobjectset"
		resourcePlural = "clusterobjectsets"
		isClusterScope = true
	case "ClusterObjectDeployment":
		resourceSingular = "clusterobjectdeployment"
		resourcePlural = "clusterobjectdeployments"
		isClusterScope = true
	case "ClusterPackage":
		resourceSingular = "clusterpackage"
		resourcePlural = "clusterpackages"
		isClusterScope = true
	default:
		return "", nil
	}

	if isClusterScope {
		// Example:
		// User: pko:clusterpackage:banana
		// Groups:
		// - pko:clusterpackages
		return strings.Join([]string{
				impersonationPrefix, resourceSingular, oi.Name,
			}, ":"), []string{
				strings.Join([]string{
					impersonationPrefix, resourcePlural,
				}, ":"),
			}
	}
	// Example:
	// User: pko:package:fruits:banana
	// Groups:
	// - pko:packages:fruits
	// - pko:packages
	return strings.Join([]string{
			impersonationPrefix, resourceSingular, oi.Namespace, oi.Name,
		}, ":"), []string{
			strings.Join([]string{
				impersonationPrefix, resourcePlural, oi.Namespace,
			}, ":"),
			strings.Join([]string{
				impersonationPrefix, resourcePlural,
			}, ":"),
		}
}
