package components

import (
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	controllerspackages "package-operator.run/internal/controllers/packages"
	"package-operator.run/internal/imageprefix"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/packages"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	PackageController struct {
		controllerAndEnvSinker
	}
	ClusterPackageController struct {
		controllerAndEnvSinker
	}
)

func ProvideRequestManager(log logr.Logger, uncachedClient UncachedClient, opts Options) *packages.RequestManager {
	return packages.NewRequestManager(
		prepareRegistryHostOverrides(log, opts.RegistryHostOverrides),
		prepareImagePrefixOverrides(log, opts.ImagePrefixOverrides),
		uncachedClient.Client,
		types.NamespacedName{
			Namespace: opts.ServiceAccountNamespace,
			Name:      opts.ServiceAccountName,
		},
	)
}

func prepareRegistryHostOverrides(log logr.Logger, flag string) map[string]string {
	if len(flag) == 0 {
		return nil
	}

	log.WithName("Registry").Info("registry host overrides active", "overrides", flag)
	out := map[string]string{}
	overrides := strings.Split(flag, ",")
	for _, or := range overrides {
		parts := strings.SplitN(or, "=", 2)
		if len(parts) != 2 {
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out
}

func prepareImagePrefixOverrides(log logr.Logger, flag string) []imageprefix.Override {
	if len(flag) == 0 {
		return nil
	}

	log.WithName("ImagePrefix").Info("image prefix overrides active", "overrides", flag)
	out := []imageprefix.Override{}
	overrides := strings.Split(flag, ",")
	for _, or := range overrides {
		parts := strings.SplitN(or, "=", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, imageprefix.Override{From: parts[0], To: parts[1]})
	}
	return out
}

func ProvidePackageController(
	mgr ctrl.Manager, log logr.Logger, uncachedClient UncachedClient,
	requestManager *packages.RequestManager,
	recorder *metrics.Recorder,
	opts Options,
) PackageController {
	return PackageController{
		controllerspackages.NewPackageController(
			mgr.GetClient(),
			uncachedClient,
			log.WithName("controllers").WithName("Package"),
			mgr.GetScheme(),
			requestManager, recorder, opts.PackageHashModifier,
			prepareImagePrefixOverrides(log, opts.ImagePrefixOverrides),
		),
	}
}

func ProvideClusterPackageController(
	mgr ctrl.Manager, log logr.Logger,
	uncachedClient UncachedClient,
	requestManager *packages.RequestManager,
	recorder *metrics.Recorder,
	opts Options,
) ClusterPackageController {
	return ClusterPackageController{
		controllerspackages.NewClusterPackageController(
			mgr.GetClient(), uncachedClient.Client,
			log.WithName("controllers").WithName("ClusterPackage"),
			mgr.GetScheme(),
			requestManager, recorder, opts.PackageHashModifier,
			prepareImagePrefixOverrides(log, opts.ImagePrefixOverrides),
		),
	}
}
