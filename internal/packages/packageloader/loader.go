package packageloader

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/internal/packages/packagecontent"
)

type (
	Loader struct {
		scheme                   *runtime.Scheme
		validators               []Validator
		transformers             []Transformer
		filesTransformers        []FilesTransformer
		packageAndFilesValidator []PackageAndFilesValidator
	}
	Option func(l *Loader)

	Validator interface {
		ValidatePackage(ctx context.Context, pkg *packagecontent.Package) error
	}

	PackageAndFilesValidator interface {
		ValidatePackageAndFiles(ctx context.Context, pkg *packagecontent.Package, files packagecontent.Files) error
	}

	Transformer interface {
		TransformPackage(ctx context.Context, pkg *packagecontent.Package) error
	}

	FilesTransformer interface {
		TransformPackageFiles(ctx context.Context, files packagecontent.Files) error
	}
)

func WithValidators(validators ...Validator) Option {
	return func(l *Loader) { l.validators = append(l.validators, validators...) }
}

func WithPackageAndFilesValidators(validators ...PackageAndFilesValidator) Option {
	return func(l *Loader) { l.packageAndFilesValidator = append(l.packageAndFilesValidator, validators...) }
}

func WithTransformers(transformers ...Transformer) Option {
	return func(l *Loader) { l.transformers = append(l.transformers, transformers...) }
}

func WithFilesTransformers(transformers ...FilesTransformer) Option {
	return func(l *Loader) { l.filesTransformers = append(l.filesTransformers, transformers...) }
}

func WithDefaults(l *Loader) {
	WithValidators(&ObjectDuplicateValidator{}, &ObjectGVKValidator{}, &ObjectLabelsValidator{}, &ObjectPhaseAnnotationValidator{})(l)
}

func New(scheme *runtime.Scheme, opts ...Option) *Loader {
	l := &Loader{scheme, []Validator{}, []Transformer{}, []FilesTransformer{}, []PackageAndFilesValidator{}}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// This modifies input file set.
func (l Loader) FromFiles(ctx context.Context, files packagecontent.Files, opts ...Option) (*packagecontent.Package, error) {
	if len(opts) != 0 {
		l = Loader{
			l.scheme,
			append([]Validator{}, l.validators...),
			append([]Transformer{}, l.transformers...),
			append([]FilesTransformer{}, l.filesTransformers...),
			append([]PackageAndFilesValidator{}, l.packageAndFilesValidator...),
		}

		for _, opt := range opts {
			opt(&l)
		}
	}

	for _, t := range l.filesTransformers {
		if err := t.TransformPackageFiles(ctx, files); err != nil {
			return nil, fmt.Errorf("transform files: %w", err)
		}
	}

	pkg, err := packagecontent.PackageFromFiles(ctx, l.scheme, files)
	if err != nil {
		return nil, fmt.Errorf("convert files to package: %w", err)
	}

	for _, t := range l.transformers {
		if err := t.TransformPackage(ctx, pkg); err != nil {
			return nil, fmt.Errorf("transform package: %w", err)
		}
	}

	for _, t := range l.validators {
		if err := t.ValidatePackage(ctx, pkg); err != nil {
			return nil, fmt.Errorf("validate package: %w", err)
		}
	}

	for _, t := range l.packageAndFilesValidator {
		if err := t.ValidatePackageAndFiles(ctx, pkg, files); err != nil {
			return nil, fmt.Errorf("validate package and files: %w", err)
		}
	}

	return pkg, nil
}
