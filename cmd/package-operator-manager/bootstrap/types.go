package bootstrap

import (
	"context"

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
	ctx context.Context, image string) (packagecontent.Files, error)
