package cmd

import (
	"fmt"

	"github.com/go-logr/logr"
)

const (
	OutputFormatHuman  = "human"
	OutputFormatDigest = "digest"
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

type WithComponent string

func (w WithComponent) ConfigureRenderPackage(c *RenderPackageConfig) {
	c.Component = string(w)
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

type WithHeaders []string

func (w WithHeaders) ConfigureTable(c *TableConfig) {
	c.Headers = []string(w)
}

type WithInsecure bool

func (w WithInsecure) ConfigureBuildFromSource(c *BuildFromSourceConfig) {
	c.Insecure = bool(w)
}

func (w WithInsecure) ConfigureGenerateLockData(c *GenerateLockDataConfig) {
	c.Insecure = bool(w)
}

func (w WithInsecure) ConfigureResolveDigest(c *ResolveDigestConfig) {
	c.Insecure = bool(w)
}

func (w WithInsecure) ConfigureValidatePackage(c *ValidatePackageConfig) {
	c.Insecure = bool(w)
}

type WithNamespace string

func (w WithNamespace) ConfigureGetPackage(c *GetPackageConfig) {
	c.Namespace = string(w)
}

func (w WithNamespace) ConfigureGetObjectDeployment(c *GetObjectDeploymentConfig) {
	c.Namespace = string(w)
}

type WithOutputPath string

func (w WithOutputPath) ConfigureBuildFromSource(c *BuildFromSourceConfig) {
	c.OutputPath = string(w)
}

type WithOutputFormat string

func (w WithOutputFormat) ConfigureBuildFromSource(c *BuildFromSourceConfig) {
	switch string(w) {
	case OutputFormatDigest, OutputFormatHuman:
		c.OutputFormat = string(w)
	default:
		panic(fmt.Sprintf("invalid output format: %s", w))
	}
}

type WithPackageLoader struct{ Loader PackageLoader }

func (w WithPackageLoader) ConfigureUpdate(c *UpdateConfig) {
	c.Loader = w.Loader
}

type WithPuller struct{ Pull PullFn }

func (w WithPuller) ConfigureValidate(c *ValidateConfig) {
	c.Pull = w.Pull
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
