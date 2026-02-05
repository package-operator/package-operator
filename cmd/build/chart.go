package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

// (Helm) Chart targets.
type Chart struct{}

func (ch *Chart) appToChartVersion(appVersion string) string {
	// appVersion has format `v1.15.0-14-g96e0a7ac`
	// helm expects chartVersion to look like `1.15.0`.
	// Take everything until the first `-` and trim the leading `v` off:
	return strings.TrimPrefix(
		strings.FieldsFunc(appVersion, func(r rune) bool { return r == '-' })[0],
		"v",
	)
}

func (ch *Chart) prepare(ctx context.Context, _ []string) error {
	self := run.Meth1(ch, ch.prepare, []string{})

	inDir := filepath.Join("config", "chart")
	chartDir := filepath.Join(cacheDir, "chart")
	chartFilePath := filepath.Join(chartDir, "Chart.yaml")

	// Remove chart directory, so the build process always starts from a clean slate.
	if err := os.RemoveAll(chartDir); err != nil {
		return err
	}

	if err := mgr.SerialDeps(ctx, self,
		run.Fn(func() error { return shr.Run(ctx, "cp", "-r", inDir, chartDir) }),
		run.Fn2(os.MkdirAll, filepath.Join(chartDir, "templates"), os.FileMode(0o755)),
		run.Meth(generate, generate.selfBootstrapJobHelm),
		run.Meth(generate, generate.helmValuesYaml),
		run.Meth3(generate, generate.generateChartYaml, chartFilePath, ch.appToChartVersion(appVersion), appVersion),
	); err != nil {
		return err
	}

	return nil
}

func (ch *Chart) build(ctx context.Context, _ []string) error {
	self := run.Meth1(ch, ch.build, []string{})

	if err := mgr.SerialDeps(ctx, self,
		run.Meth1(ch, ch.prepare, []string{}),
	); err != nil {
		return err
	}

	return shr.New(sh.WithWorkDir(cacheDir)).Run(ctx, "helm", "package", "chart")
}

func (ch *Chart) push(ctx context.Context, _ []string) error {
	self := run.Meth1(ch, ch.push, []string{})

	if err := mgr.SerialDeps(ctx, self,
		run.Meth1(ch, ch.build, []string{}),
	); err != nil {
		return err
	}

	tarballName := fmt.Sprintf("package-operator-%s.tgz", ch.appToChartVersion(appVersion))
	repoURL := fmt.Sprintf("oci://%s/helm-charts", imageRegistry())
	return shr.New(sh.WithWorkDir(cacheDir)).Run(ctx, "helm", "push", tarballName, repoURL)
}
