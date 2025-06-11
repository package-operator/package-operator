//go:build integration

package packageoperator

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

const (
	packageName      = "image-prefix-override"
	packageNamespace = "default"
)

func TestImagePrefixOverride(t *testing.T) {
	// Mirror the workload image from /src to /mirror
	require.NoError(t, crane.Copy(
		imageURLFromHost(TestStubImageSrc),
		imageURLFromHost(TestStubImageMirror),
	))
	// Mirror the package image to /mirror
	require.NoError(t, crane.Copy(
		imageURLFromHost(SuccessTestImagePrefixOverride),
		imageURLFromHost(SuccessTestImagePrefixOverrideMirror),
	))

	// Deploy the mirrored package.
	meta := metav1.ObjectMeta{
		Name:      packageName,
		Namespace: packageNamespace,
	}
	spec := corev1alpha1.PackageSpec{
		Image: SuccessTestImagePrefixOverrideMirror,
	}
	ctx := logr.NewContext(context.Background(), testr.New(t))
	testPkg := newPackage(meta, spec, true)

	deploy := &corev1alpha1.ObjectDeployment{}
	requireDeployPackage(ctx, t, testPkg, deploy)

	// Check package annotation on the objectdeployment is the mirror.
	objectDeployment := &corev1alpha1.ObjectDeployment{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: packageName, Namespace: packageNamespace,
	}, objectDeployment))
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue))

	assert.Equal(t, SuccessTestImagePrefixOverrideMirror,
		objectDeployment.GetAnnotations()[manifestsv1alpha1.PackageSourceImageAnnotation])

	// Check the deployment uses the mirror workload image.
	deployList := &appsv1.DeploymentList{}
	require.NoError(t,
		Client.List(ctx, deployList, client.MatchingLabels{manifestsv1alpha1.PackageInstanceLabel: packageName}))
	deployment := deployList.Items[0]

	// The resolved workload image in the deployment uses the digest instead of the tag.
	// So get the digest of the source image to verify.
	imageWithDigest, err := imageWithDigest(TestStubImageSrc)
	require.NoError(t, err)
	mirroredWorkloadImage := strings.Replace(imageWithDigest, "src", "mirror", 1)
	assert.Equal(t, mirroredWorkloadImage, deployment.Spec.Template.Spec.Containers[0].Image)
}

// This function replaces the tag in an image with the digest.
func imageWithDigest(image string) (string, error) {
	digest, err := crane.Digest(imageURLFromHost(image))
	if err != nil {
		return "", err
	}
	image = strings.Split(image, ":")[0]
	return image + "@" + digest, nil
}

// And converts to a url reachable from the host.
func imageURLFromHost(image string) string {
	replace := strings.Replace(image, ImageRegistry, "localhost:5001/package-operator", 1)
	return replace
}
