package command_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func TestBuildOutput(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp("", "pko-*.tar.gz")
	assert.Nil(t, err)

	defer func() { assert.Nil(t, os.Remove(f.Name())) }()
	defer func() { assert.Nil(t, f.Close()) }()

	wd, err := os.Getwd()
	assert.Nil(t, err)
	packagePath := filepath.Join(wd, "../../../config/packages/test-stub")

	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"build", packagePath, "--tag", "chicken:oldest", "--output", f.Name()})

	assert.Nil(t, cmd.Execute())
	assert.Len(t, stdout.String(), 0)
	assert.Len(t, stderr.String(), 0)

	i, err := tarball.ImageFromPath(f.Name(), nil)
	assert.Nil(t, err)
	_, err = i.Manifest()
	assert.Nil(t, err)
}

func TestBuildEmptySource(t *testing.T) {
	t.Parallel()
	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"build", ""})

	assert.NotNil(t, cmd.Execute())
}

func TestBuildNoSource(t *testing.T) {
	t.Parallel()
	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"build"})

	assert.NotNil(t, cmd.Execute())
}

func TestBuildPushWOTags(t *testing.T) {
	t.Parallel()
	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"build", ".", "--push"})

	assert.NotNil(t, cmd.Execute())
}

func TestBuildOutputWOTags(t *testing.T) {
	t.Parallel()
	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"build", ".", "--output /tmp/yes"})

	assert.NotNil(t, cmd.Execute())
}

func TestBuildInvalidTag(t *testing.T) {
	t.Parallel()
	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"build", ".", "--tag", "bread:a:b"})

	assert.NotNil(t, cmd.Execute())
}
