package packagestructure

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages/packagebytes"
)

type fileMapLoader interface {
	FromFolder(ctx context.Context, path string) (packagebytes.FileMap, error)
}

type packageManifestLoader interface {
	FromFileMap(ctx context.Context, fm packagebytes.FileMap) (
		*manifestsv1alpha1.PackageManifest, error)
}

type manifestMapLoader interface {
	FromFileMap(ctx context.Context, fileMap packagebytes.FileMap) (
		ManifestMap, error)
}

type LoaderOptions struct {
	bytesTransformers    packagebytes.TransformerList
	bytesValidators      packagebytes.ValidatorList
	manifestTransformers TransformerList
	manifestValidators   ValidatorList
}

type LoaderOption func(opt *LoaderOptions)

func WithByteValidators(validators ...packagebytes.Validator) LoaderOption {
	return func(opt *LoaderOptions) {
		opt.bytesValidators = append(opt.bytesValidators, validators...)
	}
}

func WithByteTransformers(transformer ...packagebytes.Transformer) LoaderOption {
	return func(opt *LoaderOptions) {
		opt.bytesTransformers = append(opt.bytesTransformers, transformer...)
	}
}

func WithManifestValidators(validators ...Validator) LoaderOption {
	return func(opt *LoaderOptions) {
		opt.manifestValidators = append(opt.manifestValidators, validators...)
	}
}

func WithManifestTransformers(transformer ...Transformer) LoaderOption {
	return func(opt *LoaderOptions) {
		opt.manifestTransformers = append(opt.manifestTransformers, transformer...)
	}
}

type Loader struct {
	fileMapLoader         fileMapLoader
	packageManifestLoader packageManifestLoader
	manifestMapLoader     manifestMapLoader

	opts LoaderOptions
}

func NewLoader(scheme *runtime.Scheme, opts ...LoaderOption) *Loader {
	l := &Loader{
		fileMapLoader:         packagebytes.NewLoader(),
		packageManifestLoader: NewPackageManifestLoader(scheme),
		manifestMapLoader:     NewManifestMapLoader(),
	}
	for _, opt := range opts {
		opt(&l.opts)
	}
	return l
}

func (l *Loader) Load(ctx context.Context, path string, opts ...LoaderOption) (*PackageContent, error) {
	options := l.opts // copy struct
	for _, opt := range opts {
		opt(&options)
	}

	fm, err := l.fileMapLoader.FromFolder(ctx, path)
	if err != nil {
		return nil, err
	}

	// Parse PackageManifest
	manifest, err := l.packageManifestLoader.FromFileMap(ctx, fm)
	if err != nil {
		return nil, err
	}
	packageContent := &PackageContent{PackageManifest: manifest}

	// Byte based transformations and validations
	if err := options.bytesTransformers.Transform(ctx, fm); err != nil {
		return nil, err
	}
	if err := options.bytesValidators.Validate(ctx, fm); err != nil {
		return nil, err
	}

	packageContent.Manifests, err = l.manifestMapLoader.FromFileMap(ctx, fm)
	if err != nil {
		return nil, err
	}

	// Object based transformations and validations
	if err := options.manifestTransformers.Transform(ctx, packageContent); err != nil {
		return nil, err
	}
	if err := options.manifestValidators.Validate(ctx, packageContent); err != nil {
		return nil, err
	}

	return packageContent, nil
}
