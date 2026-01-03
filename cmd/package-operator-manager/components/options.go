package components

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// Flags.
const (
	metricsAddrFlagDescription        = "The address the metric endpoint binds to."
	pprofAddrFlagDescription          = "The address the pprof web endpoint binds to."
	namespaceFlagDescription          = "The namespace the operator is deployed into."
	serviceAccountNameFlagDescription = "Name of the service-account this operator is running under. " +
		"Used to resolve potentially attached ImagePullSecrets."
	serviceAccountNamespaceFlagDescription = "Namespace of the service-account this operator is running under. " +
		"Used to resolve potentially attached ImagePullSecrets."
	leaderElectionFlagDescription = "Enable leader election for controller manager. " +
		"Enabling this will ensure there is only one active controller manager."
	probeAddrFlagDescription   = "The address the probe endpoint binds to."
	versionFlagDescription     = "print version information and exit."
	loadPackageFlagDescription = "(internal) runs the package-loader sub-component" +
		" to load a package mounted at /package"
	selfBootstrapFlagDescription = "(internal) bootstraps Package Operator" +
		" with Package Operator using the given Package Operator Package Image"
	registryHostOverrides = "List of registry host overrides to change during image pulling. " +
		"e.g. quay.io=localhost:123,<original-host>=<new-host>"
	packageOperatorPackageImage = "Image pointing to a package operator package. " +
		"This image is currently used with the HyperShift integration to spin up the remote-phase-manager " +
		"and hosted-cluster-manager for every HostedCluster"
	packageHashModifier             = "An additional value used for the generation of a package's unpackedHash."
	subCmpntAffinityFlagDescription = "Pod affinity settings used in PKO deployed subcomponents, " +
		"like remote-phase-manager."
	subCmpntTolerationsFlagDescription = "Pod tolerations settings used in PKO deployed subcomponents, " +
		"like remote-phase-manager."
	objectTemplateOptionalResourceRetryIntervalFlagDescription = "The interval at which the controller will retry " +
		"getting optional source resource for an ObjectTemplate."
	objectTemplateResourceRetryIntervalFlagDescription = "The interval at which the controller will retry " +
		"getting source resource for an ObjectTemplate."
	imagePrefixOverrides = "List of image prefix overrides to change during image pulling. " +
		"e.g. quay.io/foo=quay.io/bar/qux,<source-prefix>=<target-prefix>. If multiple prefixes match" +
		"an image address, the most specific match wins."
)

type Options struct {
	MetricsAddr                 string
	PPROFAddr                   string
	Namespace                   string
	ServiceAccountNamespace     string
	ServiceAccountName          string
	EnableLeaderElection        bool
	ProbeAddr                   string
	RegistryHostOverrides       string
	PackageHashModifier         *int32
	PackageOperatorPackageImage string
	ImagePrefixOverrides        string
	LogLevel                    int

	// sub commands
	SelfBootstrap       string
	SelfBootstrapConfig string
	PrintVersion        io.Writer

	// Sub component Settings
	SubComponentAffinity    *corev1.Affinity
	SubComponentTolerations []corev1.Toleration

	// Controller configuration
	ObjectTemplateOptionalResourceRetryInterval time.Duration
	ObjectTemplateResourceRetryInterval         time.Duration
}

func ProvideOptions() (opts Options, err error) {
	printVersion := false

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
	flag.StringVar(
		&opts.ServiceAccountNamespace, "service-account-namespace",
		os.Getenv("PKO_SERVICE_ACCOUNT_NAMESPACE"),
		serviceAccountNamespaceFlagDescription)
	flag.StringVar(
		&opts.ServiceAccountName, "service-account-name",
		os.Getenv("PKO_SERVICE_ACCOUNT_NAME"),
		serviceAccountNameFlagDescription)
	flag.BoolVar(
		&opts.EnableLeaderElection, "enable-leader-election",
		true,
		leaderElectionFlagDescription)
	flag.StringVar(
		&opts.ProbeAddr, "health-probe-bind-address", ":8081", probeAddrFlagDescription)
	flag.BoolVar(
		&printVersion, "version", false,
		versionFlagDescription)
	flag.StringVar(
		&opts.PackageOperatorPackageImage, "package-operator-package-image",
		os.Getenv("PKO_PACKAGE_OPERATOR_PACKAGE_IMAGE"),
		packageOperatorPackageImage)
	flag.StringVar(
		&opts.SelfBootstrap, "self-bootstrap", "", selfBootstrapFlagDescription)
	flag.StringVar(
		&opts.SelfBootstrapConfig, "self-bootstrap-config", os.Getenv("PKO_CONFIG"), "")
	flag.StringVar(
		&opts.RegistryHostOverrides, "registry-host-overrides",
		os.Getenv("PKO_REGISTRY_HOST_OVERRIDES"),
		registryHostOverrides)

	flag.DurationVar(
		&opts.ObjectTemplateResourceRetryInterval,
		"object-template-resource-retry-interval",
		time.Second*30, objectTemplateResourceRetryIntervalFlagDescription)
	flag.DurationVar(
		&opts.ObjectTemplateOptionalResourceRetryInterval,
		"object-template-optional-resource-retry-interval",
		time.Second*60, objectTemplateOptionalResourceRetryIntervalFlagDescription)
	flag.StringVar(
		&opts.ImagePrefixOverrides, "image-prefix-overrides",
		os.Getenv("PKO_IMAGE_PREFIX_OVERRIDES"),
		imagePrefixOverrides)
	var (
		subComponentAffinityJSON    string
		subComponentTolerationsJSON string
	)
	flag.StringVar(
		&subComponentAffinityJSON, "sub-component-affinity",
		os.Getenv("PKO_SUB_COMPONENT_AFFINITY"),
		subCmpntAffinityFlagDescription,
	)
	flag.StringVar(
		&subComponentTolerationsJSON, "sub-component-tolerations",
		os.Getenv("PKO_SUB_COMPONENT_TOLERATIONS"),
		subCmpntAffinityFlagDescription,
	)
	defaultLogLevel := 1
	if lvl, err := strconv.Atoi(os.Getenv("LOG_LEVEL")); err == nil {
		defaultLogLevel = lvl
	}
	flag.IntVar(
		&opts.LogLevel, "log-level", defaultLogLevel,
		"Log level. Default is -1 (warn). Higher numbers increase verbosity (e.g., 0 = info, 1 = debug)")

	if len(subComponentAffinityJSON) > 0 {
		if err := json.Unmarshal([]byte(subComponentAffinityJSON), &opts.SubComponentAffinity); err != nil {
			return Options{}, err
		}
	}
	if len(subComponentTolerationsJSON) > 0 {
		if err := json.Unmarshal([]byte(subComponentTolerationsJSON), &opts.SubComponentTolerations); err != nil {
			return Options{}, err
		}
	}

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

	if printVersion {
		opts.PrintVersion = os.Stderr
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
