package repositories

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/utils"
)

const (
	repoName = "bar"
	repoNs   = "foo"
	repoImg  = "some-image"
)

var errInternal = errors.New("something broke down")

func TestBuilders(t *testing.T) {
	t.Parallel()
	log := testr.New(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	reqType := types.NamespacedName{Namespace: repoNs, Name: repoName}

	cli := prepareClientMock(reqType, found, true, false)
	str := prepareStoreMock(reqType, true)

	rc := NewRepositoryController(cli, log, scheme, str)
	require.True(t, rc.newRepository(scheme).IsNamespaced())

	crc := NewClusterRepositoryController(cli, log, scheme, str)
	require.False(t, crc.newRepository(scheme).IsNamespaced())
}

func TestRepositoryController(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name               string
		clientBehaviour    clientBehaviour
		imageMatchesHash   bool
		errStatusUpdate    bool
		retrieverError     mockRetrieverError
		repoStored         bool
		expectRetrieveCall bool
		expectStoreCall    bool
		expectDeleteCall   bool
		expectError        bool
	}{
		{
			"not present",
			found, false, false, none, false,
			true, true, false, false,
		},
		{
			"not present api error",
			apiError, false, false, none, false,
			false, false, false, true,
		},
		{
			"not present pull error",
			found, false, false, pull, false,
			true, false, false, false,
		},
		{
			"not present load error",
			found, false, false, load, false,
			true, false, false, false,
		},
		{
			"not present status update error",
			found, false, true, none, false,
			true, true, false, true,
		},
		{
			"already present",
			found, true, false, none, true,
			false, false, false, false,
		},
		{
			"outdated",
			found, false, false, none, true,
			true, true, false, false,
		},
		{
			"outdated pull error",
			found, false, false, pull, true,
			true, false, false, false,
		},
		{
			"outdated load error",
			found, false, false, load, true,
			true, false, false, false,
		},
		{
			"deleted",
			absent, false, false, none, true,
			false, false, true, false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			log := testr.New(t)

			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)

			reqType := types.NamespacedName{Namespace: repoNs, Name: repoName}

			cli := prepareClientMock(reqType, test.clientBehaviour, test.imageMatchesHash, test.errStatusUpdate)
			ret := prepareRetrieverMock(test.retrieverError)
			str := prepareStoreMock(reqType, test.repoStored)

			c := newGenericRepositoryController(adapters.NewGenericRepository, cli, log, scheme, ret, str)
			require.NotNil(t, c)

			res, err := c.Reconcile(ctx, ctrl.Request{NamespacedName: reqType})

			if test.clientBehaviour != apiError {
				str.AssertCalled(t, "Contains", reqType)
			}
			if test.expectRetrieveCall {
				ret.AssertCalled(t, "Retrieve", mock.Anything, repoImg)
			} else {
				ret.AssertNotCalled(t, "Retrieve", mock.Anything, repoImg)
			}
			if test.expectStoreCall {
				str.AssertCalled(t, "Store", mock.Anything, reqType)
			} else {
				str.AssertNotCalled(t, "Store", mock.Anything, reqType)
			}
			if test.expectDeleteCall {
				str.AssertCalled(t, "Delete", reqType)
			} else {
				str.AssertNotCalled(t, "Delete", reqType)
			}

			if test.expectError {
				require.ErrorContains(t, err, errInternal.Error())
				return
			}

			require.NoError(t, err)
			if test.retrieverError == pull {
				require.NotZero(t, res.RequeueAfter)
			} else {
				require.Zero(t, res.RequeueAfter)
			}
		})
	}
}

type clientBehaviour int

const (
	found clientBehaviour = iota
	absent
	apiError
)

func prepareClientMock(
	reqType types.NamespacedName, behaviour clientBehaviour,
	matchHash bool, errStatusUpdate bool,
) *testutil.CtrlClient {
	cli := testutil.NewClient()
	switch behaviour {
	case found:
		cli.On("Get", mock.Anything, reqType, mock.IsType(&v1alpha1.Repository{}), mock.Anything).
			Run(func(args mock.Arguments) {
				rep := args.Get(2).(*v1alpha1.Repository)
				rep.Spec.Image = repoImg
				if matchHash {
					rep.Status.UnpackedHash = utils.ComputeSHA256Hash(rep.Spec, nil)
				}
			}).
			Return(nil)
	case absent:
		cli.On("Get", mock.Anything, reqType, mock.IsType(&v1alpha1.Repository{}), mock.Anything).
			Return(apierrors.NewNotFound(schema.GroupResource{
				Group:    "package-operator.run",
				Resource: "Repository",
			}, reqType.Name))
	case apiError:
		cli.On("Get", mock.Anything, reqType, mock.IsType(&v1alpha1.Repository{}), mock.Anything).
			Return(apierrors.NewInternalError(errInternal))
	}

	if errStatusUpdate {
		cli.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).
			Return(apierrors.NewInternalError(errInternal))
	} else {
		cli.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)
	}

	return cli
}

func prepareRetrieverMock(err mockRetrieverError) *mockRetriever {
	ret := mockRetriever{}
	switch err {
	case none:
		ret.On("Retrieve", mock.Anything, repoImg).
			Return(&packages.RepositoryIndex{}, nil)
	case load:
		ret.On("Retrieve", mock.Anything, repoImg).
			Return(&packages.RepositoryIndex{}, ErrRepoRetrieverLoad)
	case pull:
		ret.On("Retrieve", mock.Anything, repoImg).
			Return(&packages.RepositoryIndex{}, ErrRepoRetrieverPull)
	}
	return &ret
}

func prepareStoreMock(reqType types.NamespacedName, contains bool) *mockStore {
	str := mockStore{}
	str.On("Contains", reqType).
		Return(contains)
	if contains {
		str.On("GetKeys").
			Return([]string{fmt.Sprintf("%s.%s", repoNs, repoName)})
		str.On("GetAll").
			Return([]packages.RepositoryIndex{{}})
		str.On("GetForNamespace", repoNs).
			Return([]packages.RepositoryIndex{{}})
		str.On("GetForNamespace", mock.Anything).
			Return([]packages.RepositoryIndex{})
	} else {
		str.On("GetKeys").
			Return([]string{})
		str.On("GetAll").
			Return([]packages.RepositoryIndex{})
		str.On("GetForNamespace", mock.Anything).
			Return([]packages.RepositoryIndex{})
	}
	str.On("Store", mock.Anything, reqType)
	str.On("Delete", reqType)
	return &str
}

type mockRetrieverError int

const (
	none mockRetrieverError = iota
	pull
	load
)

type mockRetriever struct {
	mock.Mock
}

func (m *mockRetriever) Retrieve(ctx context.Context, image string) (*packages.RepositoryIndex, error) {
	args := m.Called(ctx, image)
	return args.Get(0).(*packages.RepositoryIndex), args.Error(1)
}

type mockStore struct {
	mock.Mock
}

func (s *mockStore) Contains(nsName types.NamespacedName) bool {
	args := s.Called(nsName)
	return args.Bool(0)
}

func (s *mockStore) GetKeys() []string {
	args := s.Called()
	return args.Get(0).([]string)
}

func (s *mockStore) GetAll() []*packages.RepositoryIndex {
	args := s.Called()
	return args.Get(0).([]*packages.RepositoryIndex)
}

func (s *mockStore) GetForNamespace(namespace string) []*packages.RepositoryIndex {
	args := s.Called(namespace)
	return args.Get(0).([]*packages.RepositoryIndex)
}

func (s *mockStore) Store(idx *packages.RepositoryIndex, nsName types.NamespacedName) {
	s.Called(idx, nsName)
}

func (s *mockStore) Delete(nsName types.NamespacedName) {
	s.Called(nsName)
}
