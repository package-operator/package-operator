package repocmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/spf13/cobra"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages"
)

func TestRepoCmdCorrectRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

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
	cmd = newCmd("add", testRepoFile, "quay.io/package-operator/test-stub-package:v1.10.0", "1.10.0", "1.11.0")
	require.NoError(t, cmd.Execute())
	cmd = newCmd("add", testRepoFile, "quay.io/package-operator/test-stub-multi-package:v1.10.0", "1.10.0")
	require.NoError(t, cmd.Execute())

	// add non existing container image triggers error
	cmd = newCmd("add", testRepoFile, "quay.io/package-operator/foobar:vX.Y.Z", "1.0.0")
	require.ErrorContains(t, cmd.Execute(), "pull package image")

	// add container image which is not a package operator package triggers error
	cmd = newCmd("add", testRepoFile, "quay.io/package-operator/test-stub:v1.9.3", "1.9.3")
	require.ErrorContains(t, cmd.Execute(), "raw package from package image")

	idx := assertIdx(ctx, t, testRepoFile, 2)
	assertTestStubEntry(t, idx)
	assertTestStubMultiEntry(t, idx)

	// remove existing package image
	cmd = newCmd("remove", testRepoFile, "quay.io/package-operator/test-stub-multi-package:v1.10.0")
	require.NoError(t, cmd.Execute())

	// remove non existing package image triggers error
	cmd = newCmd("remove", testRepoFile, "quay.io/package-operator/foobar:vX.Y.Z")
	require.ErrorContains(t, cmd.Execute(), "pull package image")

	// remove container image which is not a package operator package triggers error
	cmd = newCmd("remove", testRepoFile, "quay.io/package-operator/test-stub:v1.9.3")
	require.ErrorContains(t, cmd.Execute(), "raw package from package image")

	idx = assertIdx(ctx, t, testRepoFile, 1)
	assertTestStubEntry(t, idx)

	// currently we don't want to push to a real container registry each unit test run
	// but if the error is about a 401 code, this means all the previous logic works
	cmd = newCmd("push", testRepoFile, "quay.io/package-operator/non-existing-repo:v1.0.0")
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
	require.ErrorContains(t, cmd.Execute(), "package reference: could not parse reference: @$%!(NOVERB)")
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
	require.ErrorContains(t, cmd.Execute(), "given package reference: could not parse reference: @$%!(NOVERB)")
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
		"pull repository image: parsing reference \"@$%\": could not parse reference: @$%!(NOVERB)")
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
	cmd := NewCmd()
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

	dig, err := idx.GetDigest("test-stub",
		"199355dc900272b86ec5fb691bd20bea44a5dfb6a376aafb3f8beac035fc4cea")
	require.NoError(t, err)
	assert.Contains(t, dig.Data.Versions, "v1.10.0")
	assert.Contains(t, dig.Data.Versions, "v1.11.0")
}

func assertTestStubMultiEntry(t *testing.T, idx *packages.RepositoryIndex) {
	t.Helper()

	assert.Len(t, idx.ListEntries("test-stub-multi"), 1)

	vrs, err := idx.ListVersions("test-stub-multi")
	require.NoError(t, err)
	assert.Len(t, vrs, 1)

	dig, err := idx.GetDigest("test-stub-multi",
		"b1d675be4210169a23aaadb5cf6bd9294b4455704305c80e3bd2913d4a66137a")
	require.NoError(t, err)
	assert.Contains(t, dig.Data.Versions, "v1.10.0")
}
