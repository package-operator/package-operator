package cmd

import (
	"github.com/go-logr/logr"
)

type WithClock struct{ Clock Clock }

func (w WithClock) ConfigureUpdate(c *UpdateConfig) {
	c.Clock = w.Clock
}

type WithClusterScope bool

func (w WithClusterScope) ConfigureRenderPackage(c *RenderPackageConfig) {
	c.ClusterScope = bool(w)
}

type WithConfigPath string

func (w WithConfigPath) ConfigureRenderPackage(c *RenderPackageConfig) {
	c.ConfigPath = string(w)
}

type WithConfigTestcase string

func (w WithConfigTestcase) ConfigureRenderPackage(c *RenderPackageConfig) {
	c.ConfigTestcase = string(w)
}

type WithDigestResolver struct{ Resolver DigestResolver }

func (w WithDigestResolver) ConfigureBuild(c *BuildConfig) {
	c.Resolver = w.Resolver
}

func (w WithDigestResolver) ConfigureUpdate(c *UpdateConfig) {
	c.Resolver = w.Resolver
}

type WithLog struct{ Log logr.Logger }

func (w WithLog) ConfigureBuild(c *BuildConfig) {
	c.Log = w.Log
}

func (w WithLog) ConfigureTree(c *TreeConfig) {
	c.Log = w.Log
}

func (w WithLog) ConfigureUpdate(c *UpdateConfig) {
	c.Log = w.Log
}

func (w WithLog) ConfigureValidate(c *ValidateConfig) {
	c.Log = w.Log
}

type WithOutputPath string

func (w WithOutputPath) ConfigureBuildFromSource(c *BuildFromSourceConfig) {
	c.OutputPath = string(w)
}

type WithPackageLoader struct{ Loader PackageLoader }

func (w WithPackageLoader) ConfigureUpdate(c *UpdateConfig) {
	c.Loader = w.Loader
}

type WithPackagePuller struct{ Puller PackagePuller }

func (w WithPackagePuller) ConfigureValidate(c *ValidateConfig) {
	c.Puller = w.Puller
}

type WithPath string

func (w WithPath) ConfigureValidatePackage(c *ValidatePackageConfig) {
	c.Path = string(w)
}

type WithPush bool

func (w WithPush) ConfigureBuildFromSource(c *BuildFromSourceConfig) {
	c.Push = bool(w)
}

type WithRemoteReference string

func (w WithRemoteReference) ConfigureValidatePackage(c *ValidatePackageConfig) {
	c.RemoteReference = string(w)
}

type WithTags []string

func (w WithTags) ConfigureBuildFromSource(c *BuildFromSourceConfig) {
	c.Tags = append(c.Tags, w...)
}
