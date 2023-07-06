package components

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

// Flags.
const (
	metricsAddrFlagDescription    = "The address the metric endpoint binds to."
	pprofAddrFlagDescription      = "The address the pprof web endpoint binds to."
	namespaceFlagDescription      = "The namespace the operator is deployed into."
	leaderElectionFlagDescription = "Enable leader election for controller manager. " +
		"Enabling this will ensure there is only one active controller manager."
	probeAddrFlagDescription   = "The address the probe endpoint binds to."
	versionFlagDescription     = "print version information and exit."
	copyToFlagDescription      = "(internal) copy this binary to a new location"
	loadPackageFlagDescription = "(internal) runs the package-loader sub-component" +
		" to load a package mounted at /package"
	selfBootstrapFlagDescription = "(internal) bootstraps Package Operator" +
		" with Package Operator using the given Package Operator Package Image"
	remotePhasePackageImageFlagDescription = "Image pointing to a package operator remote phase package. " +
		"This image is used with the HyperShift integration to spin up the remote-phase-manager for every HostedCluster"
	registryHostOverrides = "List of registry host overrides to change during image pulling. e.g. quay.io=localhost:123,<original-host>=<new-host>"
	packageHashModifier   = "An additional value used for the generation of a package's unpackedHash."
)

type Options struct {
	MetricsAddr             string
	PPROFAddr               string
	Namespace               string
	EnableLeaderElection    bool
	ProbeAddr               string
	RemotePhasePackageImage string
	RegistryHostOverrides   string
	PackageHashModifier     *int32

	// sub commands
	SelfBootstrap       string
	SelfBootstrapConfig string
	PrintVersion        bool
	CopyTo              string
}

func ProvideOptions() (opts Options, err error) {
	flag.StringVar(
		&opts.MetricsAddr, "metrics-addr",
		":8080",
		metricsAddrFlagDescription)
	flag.StringVar(
		&opts.PPROFAddr, "pprof-addr",
		"",
		pprofAddrFlagDescription)
	flag.StringVar(
		&opts.Namespace, "namespace",
		os.Getenv("PKO_NAMESPACE"),
		namespaceFlagDescription)
	flag.BoolVar(
		&opts.EnableLeaderElection, "enable-leader-election",
		false,
		leaderElectionFlagDescription)
	flag.StringVar(
		&opts.ProbeAddr, "health-probe-bind-address", ":8081", probeAddrFlagDescription)
	flag.BoolVar(
		&opts.PrintVersion, "version", false,
		versionFlagDescription)
	flag.StringVar(
		&opts.CopyTo, "copy-to", "",
		copyToFlagDescription)
	flag.StringVar(
		&opts.SelfBootstrap, "self-bootstrap", "", selfBootstrapFlagDescription)
	flag.StringVar(
		&opts.SelfBootstrapConfig, "self-bootstrap-config", os.Getenv("PKO_CONFIG"), "")
	flag.StringVar(
		&opts.RemotePhasePackageImage, "remote-phase-package-image",
		os.Getenv("PKO_REMOTE_PHASE_PACKAGE_IMAGE"),
		remotePhasePackageImageFlagDescription)
	flag.StringVar(
		&opts.RegistryHostOverrides, "registry-host-overrides",
		os.Getenv("PKO_REGISTRY_HOST_OVERRIDES"),
		registryHostOverrides)

	packageHashModifierInt, err := envToInt("PKO_PACKAGE_HASH_MODIFIER")
	if err != nil {
		return Options{}, err
	}

	tmpPackageHashModifier := flag.Int(
		"package-hash-modifier", packageHashModifierInt,
		packageHashModifier)
	flag.Parse()

	if *tmpPackageHashModifier != 0 {
		packageHashModifierInt32 := int32(*tmpPackageHashModifier)
		opts.PackageHashModifier = &packageHashModifierInt32
	}

	return opts, nil
}

// Parses an environment variable string value to integer value.
// Returns 0 in case the environment variable is unset.
func envToInt(env string) (int, error) {
	envStrValue := os.Getenv(env)

	if envStrValue == "" {
		return 0, nil
	}

	parsedIntValue, err := strconv.Atoi(envStrValue)
	if err != nil {
		return 0, fmt.Errorf("unable to parse environment variable '%s' as integer: %w", env, err)
	}

	return parsedIntValue, nil
}
