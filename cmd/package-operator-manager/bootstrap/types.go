package bootstrap

import (
	"context"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageloader"
)

type packageLoader interface {
	FromFiles(
		ctx context.Context, files packagecontent.Files,
		opts ...packageloader.Option,
	) (*packagecontent.Package, error)
}

type bootstrapperPullImageFn func(
	ctx context.Context, image string,
	pkgType corev1alpha1.PackageType,
) (packagecontent.Files, error)
