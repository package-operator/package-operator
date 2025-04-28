//go:build integration

package kubectlpackage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"package-operator.run/cmd/kubectl-package/repocmd"

	"github.com/google/go-containerregistry/pkg/crane"

	"github.com/stretchr/testify/assert"

	"github.com/spf13/cobra"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages"
)

const (
	authRegistryHost  = "localhost:5002"
	plainRegistryHost = "localhost:5001"
)

var (
	appVersion                      string
	nonExistingImage                string
	unauthenticatedImage            string
	testStubImage                   string
	testStubPackageImage            string
	testStubPackageImageDigest      string
	testStubMultiPackageImage       string
	testStubMultiPackageImageDigest string
)

func init() {
	appVersion = os.Getenv("PKO_TEST_VERSION")
	if len(appVersion) == 0 {
		panic("PKO_TEST_VERSION not set!")
	}
	nonExistingImage = img(plainRegistryHost, "package-operator/foobar", "vX.Y.Z")
	unauthenticatedImage = img(authRegistryHost, "package-operator/unauthenticated", "v1.0.0")
	testStubImage = img(plainRegistryHost, "package-operator/test-stub", appVersion)
	testStubPackageImage = img(plainRegistryHost, "package-operator/test-stub-package", appVersion)
	testStubMultiPackageImage = img(plainRegistryHost, "package-operator/test-stub-multi-package", appVersion)
	var err error
	testStubPackageImageDigest, err = crane.Digest(testStubPackageImage)
	if err != nil {
		panic(err)
	}
	testStubMultiPackageImageDigest, err = crane.Digest(testStubMultiPackageImage)
	if err != nil {
		panic(err)
	}
}

func img(host, path, tag string) string {
	return fmt.Sprintf("%s/%s:%s", host, path, tag)
}

func TestRepoCmdCorrectRun(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const testDataDir = "testdata"
	_, err := os.Stat(testDataDir)
	if os.IsNotExist(err) {
		require.NoError(t, os.Mkdir("testdata", os.ModePerm))
	}

	testRepoFile := filepath.Join(testDataDir, "repo.yaml")

	cmd := newCmd("init", testRepoFile, "test-repo")
	require.NoError(t, cmd.Execute())

	// check that the file exists
	_, err = os.Stat(testRepoFile)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := os.Remove(testRepoFile); err != nil {
			panic(err)
		}
	})

	// add existing package images
	cmd = newCmd("add", testRepoFile, testStubPackageImage, "1.0.0", "1.1.0")
	require.NoError(t, cmd.Execute())
	cmd = newCmd("add", testRepoFile, testStubMultiPackageImage, "1.0.0")
	require.NoError(t, cmd.Execute())

	// add non existing container image triggers error
	cmd = newCmd("add", testRepoFile, nonExistingImage, "1.0.0")
	require.ErrorContains(t, cmd.Execute(), "pull package image")

	// add container image which is not a package operator package triggers error
	cmd = newCmd("add", testRepoFile, testStubImage, "1.0.1")
	require.ErrorContains(t, cmd.Execute(), "raw package from package image")

	idx := assertIdx(ctx, t, testRepoFile, 2)
	assertTestStubEntry(t, idx)
	assertTestStubMultiEntry(t, idx)

	// remove existing package image
	cmd = newCmd("remove", testRepoFile, testStubMultiPackageImage)
	require.NoError(t, cmd.Execute())

	// remove non existing package image triggers error
	cmd = newCmd("remove", testRepoFile, nonExistingImage)
	require.ErrorContains(t, cmd.Execute(), "pull package image")

	// remove container image which is not a package operator package triggers error
	cmd = newCmd("remove", testRepoFile, testStubImage)
	require.ErrorContains(t, cmd.Execute(), "raw package from package image")

	idx = assertIdx(ctx, t, testRepoFile, 1)
	assertTestStubEntry(t, idx)

	// currently we don't want to push to a real container registry each unit test run
	// but if the error is about a 401 code, this means all the previous logic works
	cmd = newCmd("push", testRepoFile, unauthenticatedImage)
	require.ErrorContains(t, cmd.Execute(), "unexpected status code 401 Unauthorized")
}

func TestRepoCmdMalformedParams(t *testing.T) {
	t.Parallel()

	// init
	cmd := newCmd("init")
	require.ErrorContains(t, cmd.Execute(), "accepts 2 arg(s), received 0")
	cmd = newCmd("init", "foobar.yaml")
	require.ErrorContains(t, cmd.Execute(), "accepts 2 arg(s), received 1")
	cmd = newCmd("init", "", "foobar")
	require.ErrorContains(t, cmd.Execute(), "arguments invalid: file must be not empty")
	cmd = newCmd("init", "foobar.yaml", "")
	require.ErrorContains(t, cmd.Execute(), "arguments invalid: name must be not empty")

	// add
	cmd = newCmd("add")
	require.ErrorContains(t, cmd.Execute(), "requires at least 3 arg(s), only received 0")
	cmd = newCmd("add", "foobar.yaml")
	require.ErrorContains(t, cmd.Execute(), "requires at least 3 arg(s), only received 1")
	cmd = newCmd("add", "foobar.yaml", "@$%")
	require.ErrorContains(t, cmd.Execute(), "requires at least 3 arg(s), only received 2")
	cmd = newCmd("add", "foobar.yaml", "@$%", "1.0.0")
	require.ErrorContains(t, cmd.Execute(), "package reference: could not parse reference:")
	cmd = newCmd("add", "foobar.yaml", "quay.io/foo/bar:latest", "@$%")
	require.ErrorContains(t, cmd.Execute(), "version: col 1: starts with non-positive integer '@'")
	cmd = newCmd("add", "foobar.yaml", "quay.io/foo/bar:latest", "1.0.0", "@$%")
	require.ErrorContains(t, cmd.Execute(), "version: col 1: starts with non-positive integer '@'")
	cmd = newCmd("add", "foobar.yaml", "quay.io/foo/bar:latest", "1.0.0", "1.1.0")
	require.ErrorContains(t, cmd.Execute(), "open foobar.yaml: no such file or directory")

	// remove
	cmd = newCmd("remove")
	require.ErrorContains(t, cmd.Execute(), "accepts 2 arg(s), received 0")
	cmd = newCmd("remove", "foobar.yaml")
	require.ErrorContains(t, cmd.Execute(), "accepts 2 arg(s), received 1")
	cmd = newCmd("remove", "foobar.yaml", "@$%")
	require.ErrorContains(t, cmd.Execute(), "given package reference: could not parse reference:")
	cmd = newCmd("remove", "foobar.yaml", "quay.io/foo/bar:latest")
	require.ErrorContains(t, cmd.Execute(), "open foobar.yaml: no such file or directory")

	// pull
	cmd = newCmd("pull")
	require.ErrorContains(t, cmd.Execute(), "accepts 2 arg(s), received 0")
	cmd = newCmd("pull", "foobar.yaml")
	require.ErrorContains(t, cmd.Execute(), "accepts 2 arg(s), received 1")
	cmd = newCmd("pull", "foobar.yaml", "")
	require.ErrorContains(t, cmd.Execute(), "arguments invalid: tag must be not empty")
	cmd = newCmd("pull", "", "quay.io/foo/bar:latest")
	require.ErrorContains(t, cmd.Execute(), "arguments invalid: file must be not empty")
	cmd = newCmd("pull", "foobar.yaml", "@$%")
	require.ErrorContains(t, cmd.Execute(),
		"pull repository image: parsing reference \"@$%\": could not parse reference:")
	cmd = newCmd("pull", "foobar.yaml", "quay.io/foo/bar:latest")
	require.ErrorContains(t, cmd.Execute(),
		"pull repository image: GET https://quay.io/v2/foo/bar/manifests/latest: "+
			"UNAUTHORIZED: access to the requested resource is not authorized")

	// push
	cmd = newCmd("push")
	require.ErrorContains(t, cmd.Execute(), "accepts 2 arg(s), received 0")
	cmd = newCmd("push", "foobar.yaml")
	require.ErrorContains(t, cmd.Execute(), "accepts 2 arg(s), received 1")
	cmd = newCmd("push", "foobar.yaml", "")
	require.ErrorContains(t, cmd.Execute(), "arguments invalid: tag must be not empty")
	cmd = newCmd("push", "", "quay.io/foo/bar:latest")
	require.ErrorContains(t, cmd.Execute(), "arguments invalid: file must be not empty")
	cmd = newCmd("push", "foobar.yaml", "@$%")
	require.ErrorContains(t, cmd.Execute(), "read from file: open foobar.yaml: no such file or directory")
}

func newCmd(args ...string) *cobra.Command {
	cmd := repocmd.NewCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func assertIdx(ctx context.Context, t *testing.T, testRepoFile string, expectedEntries int) *packages.RepositoryIndex {
	t.Helper()

	idx, err := packages.LoadRepositoryFromFile(ctx, testRepoFile)
	require.NoError(t, err)
	assert.Equal(t, "test-repo", idx.Metadata().Name)
	assert.Len(t, idx.ListAllEntries(), expectedEntries)
	return idx
}

func assertTestStubEntry(t *testing.T, idx *packages.RepositoryIndex) {
	t.Helper()

	assert.Len(t, idx.ListEntries("test-stub"), 1)

	vrs, err := idx.ListVersions("test-stub")
	require.NoError(t, err)
	assert.Len(t, vrs, 2)

	dig, err := idx.GetDigest("test-stub", testStubPackageImageDigest[7:])
	require.NoError(t, err)
	assert.Contains(t, dig.Data.Versions, "v1.0.0")
	assert.Contains(t, dig.Data.Versions, "v1.1.0")
}

func assertTestStubMultiEntry(t *testing.T, idx *packages.RepositoryIndex) {
	t.Helper()

	assert.Len(t, idx.ListEntries("test-stub-multi"), 1)

	vrs, err := idx.ListVersions("test-stub-multi")
	require.NoError(t, err)
	assert.Len(t, vrs, 1)

	dig, err := idx.GetDigest("test-stub-multi", testStubMultiPackageImageDigest[7:])
	require.NoError(t, err)
	assert.Contains(t, dig.Data.Versions, "v1.0.0")
}
