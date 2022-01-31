package integration_test

import (
	"fmt"
	"time"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

var (
	// Version: v0.1.0
	referenceAddonCatalogSourceImageWorking = "quay.io/osd-addons/reference-addon-index@sha256:58cb1c4478a150dc44e6c179d709726516d84db46e4e130a5227d8b76456b5bd"

	// The latest bundle in this index image deploys a version of our referene-addon where InstallPlan and CSV never succeed
	// because the deployed operator pod is deliberately broken through invalid readiness and liveness probes.
	// Version: v0.1.3
	referenceAddonCatalogSourceImageBroken = "quay.io/osd-addons/reference-addon-index@sha256:9e6306e310d585610d564412780d58ec54cb24a67d7cdabfc067ab733295010a"
	referenceAddonConfigEnvObjects         = []addonsv1alpha1.EnvObject{
		{Name: "TESTING1", Value: "TRUE"},
		{Name: "TESTING2", Value: "TRUE"},
	}

	defaultAddonDeletionTimeout     = 4 * time.Minute
	defaultAddonAvailabilityTimeout = 10 * time.Minute

	defaultPodDeletionTimeout     = 4 * time.Minute
	defaultPodAvailabilityTimeout = 10 * time.Minute
)

func addon_OwnNamespace_UpgradePolicyReporting() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "addon-aefigh1x",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-aefigh1x",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "namespace-eecu3ou1"},
				{Name: "namespace-jei9egh2"},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "namespace-eecu3ou1",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
						Channel:            "alpha",
						PackageName:        "reference-addon",
						Config: &addonsv1alpha1.SubscriptionConfig{
							EnvironmentVariables: referenceAddonConfigEnvObjects,
						},
					},
				},
			},
			UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicy{
				ID: "123-456-789",
			},
		},
	}
}

func addon_OwnNamespace() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "addon-oisafbo12",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-oisafbo12",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "namespace-onbgdions"},
				{Name: "namespace-pioghfndb"},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "namespace-onbgdions",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
						Channel:            "alpha",
						PackageName:        "reference-addon",
						Config: &addonsv1alpha1.SubscriptionConfig{
							EnvironmentVariables: referenceAddonConfigEnvObjects,
						},
					},
				},
			},
		},
	}
}

func addon_AllNamespaces() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-2425constance",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-2425constance",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{
					Name: "namespace-2425constance",
				},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
				OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "namespace-2425constance",
						PackageName:        "reference-addon",
						Channel:            "alpha",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
					},
				},
			},
		},
	}
}

var uuid = "c24cd15c-4353-4036-bd86-384046eb4ff8"

func addon_OwnNamespace_TestBrokenSubscription() *addonsv1alpha1.Addon {

	addonName := fmt.Sprintf("addon-%s", uuid)
	addonNamespace := fmt.Sprintf("namespace-%s", uuid)

	return &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: addonName,
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: addonName,
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: addonNamespace},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          addonNamespace,
						CatalogSourceImage: referenceAddonCatalogSourceImageBroken,
						PackageName:        "reference-addon",
						Channel:            "alpha",
					},
				},
			},
		},
	}
}

var (
	referenceAddonNamespace   = "reference-addon"
	referenceAddonName        = "reference-addon"
	referenceAddonDisplayName = "Reference Addon"
)

// taken from -
// https://gitlab.cee.redhat.com/service/managed-tenants-manifests/-/blob/c60fa3f0252d908b5f868994f8934d24bbaca5f4/stage/addon-reference-addon-SelectorSyncSet.yaml
func namespace_TestResourceAdoption() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"openshift.io/node-selector": "",
			},
			Name: referenceAddonNamespace,
		},
	}
}

func catalogsource_TestResourceAdoption() *operatorsv1alpha1.CatalogSource {
	return &operatorsv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referenceAddonName,
			Namespace: referenceAddonNamespace,
		},
		Spec: operatorsv1alpha1.CatalogSourceSpec{
			DisplayName: referenceAddonDisplayName,
			Image:       referenceAddonCatalogSourceImageWorking,
			Publisher:   "OSD Red Hat Addons",
			SourceType:  operatorsv1alpha1.SourceTypeGrpc,
		},
	}
}

func operatorgroup_TestResourceAdoption() *operatorsv1.OperatorGroup {
	return &operatorsv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referenceAddonName,
			Namespace: referenceAddonNamespace,
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{referenceAddonNamespace},
		},
	}
}

func subscription_TestResourceAdoption() *operatorsv1alpha1.Subscription {
	return &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referenceAddonName,
			Namespace: referenceAddonNamespace},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          referenceAddonName,
			CatalogSourceNamespace: referenceAddonNamespace,
			Channel:                referenceAddonNamespace,
			Package:                referenceAddonName,
		},
	}
}

func addon_TestResourceAdoption() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: referenceAddonName,
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: referenceAddonName,
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{
					Name: referenceAddonNamespace,
				},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          referenceAddonNamespace,
						PackageName:        referenceAddonNamespace,
						Channel:            "alpha",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
					},
				},
			},
		},
	}
}

func pod_metricsClient() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-metrics-client",
			Namespace: "addon-operator",
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "metrics-server-cert",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "playground-container",
					Image:   "registry.access.redhat.com/ubi8/ubi-minimal@sha256:16da4d4c5cb289433305050a06834b7328769f8a5257ad5b4a5006465a0379ff",
					Command: []string{"sh"},
					Args:    []string{"-c", "sleep infinity;"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "tls",
							MountPath: "/tmp/k8s-metrics-server/serving-certs/",
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}
}
