package main

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/package-operator/internal/testutil"
)

func Test_fetchPackageResourcesFromPackageDir(t *testing.T) {
	testCases := []struct {
		Name           string
		PackageDirPath string
		Files          []struct {
			name    string
			content string
			dirPath string
		}
		CleanupDirs    []string
		ExpectedErr    error
		ExpectedOutput []string
	}{
		{
			Name:           "Files properly set under the packageDir",
			ExpectedErr:    nil,
			ExpectedOutput: []string{path.Join("./test", "manifest.yaml"), path.Join("./test", "some-statefulset.yaml"), path.Join("./test", "ocs-deployment.yaml"), path.Join("./test/subdir", "certain-configmap.yaml"), path.Join("./test/subdir", "some-serviceaccount.yaml")},
			PackageDirPath: "./test",
			CleanupDirs:    []string{"./test"},
			Files: []struct {
				name    string
				content string
				dirPath string
			}{
				{
					name:    "manifest.yaml",
					dirPath: "./test",
					content: `apiVersion: manifests.package-operator.run/v1alpha1
					kind: PackageManifest
					catalog:
					  displayName: Cool Package
					  shortDescription: xxx xxx xxx
					  version: 0.2.4
					  iconFile: my-icon.png # relative file location within package
					  keywords:
					  - cool
					  provider:
						name: Example Corp
						url: example.com
					  links:
					  - name: Source Code
						url: https://example.com/example-corp/cool-package
					  maintainers:
					  - email: cool-package-people@example.com
						name: Cool package maintainers
					spec:
					  phases:
					  - name: pre-requisites
					  - name: main-stuff
					  availabilityProbes:
					  - probes:
						- condition:
							type: Available
							status: "True"
						- fieldsEqual:
							fieldA: .status.updatedReplicas
							fieldB: .status.replicas
						selector:
						  kind:
							group: apps
							kind: Deployment
					`,
				},
				{
					name:    "some-statefulset.yaml",
					dirPath: "./test",
					content: `apiVersion: apps/v1
					kind: StatefulSet
					metadata:
					  name: some-stateful-set-1"
					  annotations:
						package-operator.run/phase: "main-stuff"
					spec:
					  template:
						containers:
						  image: image-dep:v1
					`,
				},
				{
					name:    "ocs-deployment.yaml",
					dirPath: "./test",
					content: `apiVersion: apps/v1
					kind: Deployment
					metadata:
					  name: ocs-osd-controller-manager
					  annotations:
						package-operator.run/phase: "main-stuff"
					spec:
					  template:
						containers:
						  image: image-dep:v1
					`,
				},
				{
					name:    "certain-configmap.yaml",
					dirPath: "./test/subdir",
					content: `apiVersion: v1
					kind: ConfigMap
					metadata:
					  name: some-configmap
					  annotations:
						package-operator.run/phase: "pre-requisites"
					data:
					  foo: bar
					  hello: world
					`,
				},
				{
					name:    "some-serviceaccount.yaml",
					dirPath: "./test/subdir",
					content: `apiVersion: v1
					kind: ServiceAccount
					metadata:
					  name: some-service-account
					  annotations:
						package-operator.run/phase: "pre-requisites"
					`,
				},
			},
		},
		{
			Name:           "Resources outside the packageDir ignored",
			ExpectedErr:    nil,
			ExpectedOutput: []string{},
			PackageDirPath: "./test-1",
			CleanupDirs:    []string{"./test", "./test-1"},
			Files: []struct {
				name    string
				content string
				dirPath string
			}{
				{
					name:    "some-statefulset.yaml",
					dirPath: "./test/unexpected-dir",
					content: `apiVersion: apps/v1
					kind: StatefulSet
					metadata:
					  name: some-stateful-set-1"
					  annotations:
						package-operator.run/phase: "main-stuff"
					spec:
					  template:
						containers:
						  image: image-dep:v1
					`,
				},
				{
					name:    "ocs-deployment.yaml",
					dirPath: "./test/unexpected-dir",
					content: `apiVersion: apps/v1
					kind: Deployment
					metadata:
					  name: ocs-osd-controller-manager
					  annotations:
						package-operator.run/phase: "main-stuff"
					spec:
					  template:
						containers:
						  image: image-dep:v1
					`,
				},
				{
					name:    "certain-configmap.yaml",
					dirPath: "./test/unexpected-dir",
					content: `apiVersion: v1
					kind: ConfigMap
					metadata:
					  name: some-configmap
					  annotations:
						package-operator.run/phase: "pre-requisites"
					data:
					  foo: bar
					  hello: world
					`,
				},
				{
					name:    "some-serviceaccount.yaml",
					dirPath: "./test/unexpected-dir",
					content: `apiVersion: v1
					kind: ServiceAccount
					metadata:
					  name: some-service-account
					  annotations:
						package-operator.run/phase: "pre-requisites"
					`,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			// pre-flight checks
			require.NoDirExists(t, testCase.PackageDirPath)
			for _, file := range testCase.Files {
				require.NoDirExists(t, file.dirPath)
			}

			// setup packageDir
			err := os.MkdirAll(testCase.PackageDirPath, os.ModePerm)
			require.NoError(t, err)

			// setup files
			for _, file := range testCase.Files {
				err := os.MkdirAll(file.dirPath, os.ModePerm)
				require.NoError(t, err)

				filePath := path.Join(file.dirPath, file.name)
				err = os.WriteFile(filePath, []byte(file.content), 0600)
				require.NoError(t, err)
			}

			// test
			resourcePathsFound, err := fetchPackageResourcesFromPackageDir(testCase.PackageDirPath)
			if testCase.ExpectedErr != nil {
				require.ErrorIs(t, err, testCase.ExpectedErr)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, resourcePathsFound, testCase.ExpectedOutput)
			}

			// teardown
			for _, path := range testCase.CleanupDirs {
				err := os.RemoveAll(path)
				require.NoError(t, err)
			}
		})
	}
}

type errWithStatusError struct {
	errMsg          string
	errStatusReason metav1.StatusReason
}

func (err errWithStatusError) Error() string {
	return err.errMsg
}
func (err errWithStatusError) Status() metav1.Status {
	return metav1.Status{Reason: err.errStatusReason}
}

func Test_ensureNamespace(t *testing.T) {
	testCases := []struct {
		Name                                 string
		NamespaceAlreadyExists               bool
		NamespaceToEnsure                    string
		ErrorThrownWhileCreatingTheNamespace error
		ExpectedErr                          error
	}{
		{
			Name:                                 "Namespace doesn't exist originally",
			NamespaceAlreadyExists:               false,
			NamespaceToEnsure:                    "foo",
			ErrorThrownWhileCreatingTheNamespace: nil,
			ExpectedErr:                          nil,
		},
		{
			Name:                                 "Namespace existed originally",
			NamespaceAlreadyExists:               true,
			NamespaceToEnsure:                    "foo",
			ErrorThrownWhileCreatingTheNamespace: errWithStatusError{errStatusReason: metav1.StatusReasonAlreadyExists},
			ExpectedErr:                          nil,
		},
		{
			Name:                                 "Some error occurred while creating the namespace",
			NamespaceAlreadyExists:               false,
			NamespaceToEnsure:                    "foo",
			ErrorThrownWhileCreatingTheNamespace: errWithStatusError{errMsg: "some error"},
			ExpectedErr:                          errWithStatusError{errMsg: "some error"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {

			clientMock := testutil.NewClient()
			namespaceToCreate := testCase.NamespaceToEnsure
			clientMock.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(testCase.ErrorThrownWhileCreatingTheNamespace)
			err := ensureNamespace(clientMock, namespaceToCreate)

			if testCase.ExpectedErr != nil {
				require.ErrorIs(t, testCase.ExpectedErr, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
