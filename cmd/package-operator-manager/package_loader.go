package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/package-operator/cmd/package-operator-manager/components"
	"package-operator.run/package-operator/internal/packages/packagedeploy"
	"package-operator.run/package-operator/internal/packages/packageimport"
)

const packageFolderPath = "/package"

type packageLoader struct {
	log        logr.Logger
	scheme     *runtime.Scheme
	restConfig *rest.Config

	// options
	loadPackage string
}

func newPackageLoader(
	log logr.Logger, scheme *runtime.Scheme,
	restConfig *rest.Config, opts components.Options,
) *packageLoader {
	return &packageLoader{
		log:        log.WithName("package-loader"),
		scheme:     scheme,
		restConfig: restConfig,

		loadPackage: opts.LoadPackage,
	}
}

var errInvalidLoadPackageArgument = errors.New("invalid argument to --load-package, expected NamespaceName")

func (pl *packageLoader) Start(ctx context.Context) error {
	log := pl.log
	ctx = logr.NewContext(ctx, log)

	namespace, name, found := strings.Cut(pl.loadPackage, string(types.Separator))
	if !found {
		return errInvalidLoadPackageArgument
	}

	packageKey := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	c, err := client.New(pl.restConfig, client.Options{
		Scheme: pl.scheme,
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	var packageDeployer *packagedeploy.PackageDeployer
	if len(packageKey.Namespace) > 0 {
		// Package API
		packageDeployer = packagedeploy.NewPackageDeployer(c, pl.scheme)
	} else {
		// ClusterPackage API
		packageDeployer = packagedeploy.NewClusterPackageDeployer(c, pl.scheme)
	}

	files, err := packageimport.Folder(ctx, packageFolderPath)
	if err != nil {
		return err
	}

	return packageDeployer.Load(ctx, packageKey, files)
}
