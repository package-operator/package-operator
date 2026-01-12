package buildcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	internalcmd "package-operator.run/internal/cmd"
)

func TestBuildOutput(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp("", "pko-*.tar.gz")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()
	defer func() { require.NoError(t, f.Close()) }()

	wd, err := os.Getwd()
	require.NoError(t, err)
	packagePath := filepath.Join(wd, "testdata")

	factory := &builderFactoryMock{}
	factory.On("Builder").Return(internalcmd.NewBuild())

	cmd := NewCmd(factory)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{packagePath, "--tag", "chicken:oldest", "--output", f.Name()})

	require.NoError(t, cmd.Execute())
	require.Equal(t, "Package built successfully!", stdout.String())
	require.Empty(t, stderr.String())

	i, err := tarball.ImageFromPath(f.Name(), nil)
	require.NoError(t, err)
	_, err = i.Manifest()
	require.NoError(t, err)
}

func TestBuildEmptySource(t *testing.T) {
	t.Parallel()

	factory := &builderFactoryMock{}
	factory.On("Builder").Return(internalcmd.NewBuild())

	cmd := NewCmd(factory)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{""})

	require.Error(t, cmd.Execute())
}

func TestBuildNoSource(t *testing.T) {
	t.Parallel()

	factory := &builderFactoryMock{}
	factory.On("Builder").Return(internalcmd.NewBuild())

	cmd := NewCmd(factory)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	require.Error(t, cmd.Execute())
}

func TestBuildPushWOTags(t *testing.T) {
	t.Parallel()
	factory := &builderFactoryMock{}
	factory.On("Builder").Return(internalcmd.NewBuild())

	cmd := NewCmd(factory)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{".", "--push"})

	require.Error(t, cmd.Execute())
}

func TestBuildOutputWOTags(t *testing.T) {
	t.Parallel()

	factory := &builderFactoryMock{}
	factory.On("Builder").Return(internalcmd.NewBuild())

	cmd := NewCmd(factory)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{".", "--output /tmp/yes"})

	require.Error(t, cmd.Execute())
}

func TestBuildInvalidTag(t *testing.T) {
	t.Parallel()

	factory := &builderFactoryMock{}
	factory.On("Builder").Return(internalcmd.NewBuild())

	cmd := NewCmd(factory)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{".", "--tag", "bread:a:b"})

	require.Error(t, cmd.Execute())
}

func TestBuildInvalidLabel(t *testing.T) {
	t.Parallel()

	factory := &builderFactoryMock{}
	factory.On("Builder").Return(internalcmd.NewBuild())

	cmd := NewCmd(factory)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{".", "--label", "test="})

	require.Error(t, cmd.Execute())
}

func TestBuildWithPath(t *testing.T) {
	t.Parallel()

	wd, err := os.Getwd()
	require.NoError(t, err)
	packagePath := filepath.Join(wd, "testdata")

	factory := &builderFactoryMock{}
	factory.On("Builder").Return(internalcmd.NewBuild())

	cmd := NewCmd(factory)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{packagePath})

	require.NoError(t, cmd.Execute())
	require.Equal(t, "Package built successfully!", stdout.String())
	require.Empty(t, stderr.String())
}

type builderFactoryMock struct {
	mock.Mock
}

func (m *builderFactoryMock) Builder() Builder {
	args := m.Called()

	return args.Get(0).(Builder)
}
