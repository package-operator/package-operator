package preflight_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/preflight"
)

type sub struct{ mock.Mock }

func (s *sub) Check(
	ctx context.Context, owner client.Object, obj client.Object,
) (violations []preflight.Violation, err error) {
	ret := s.Called(ctx, owner, obj)

	return ret.Get(0).([]preflight.Violation), ret.Error(1)
}

type mapper struct{ mock.Mock }

func (m *mapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	ret := m.Called(gk, versions)

	return ret.Get(0).(*meta.RESTMapping), ret.Error(1)
}

func TestAPIExistenceExists(t *testing.T) {
	t.Parallel()

	m := &mapper{}
	s := &sub{}
	ctx := t.Context()
	owner := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "blorb"}}
	obj := &appsv1.Deployment{TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "3"}}

	checkVios := []preflight.Violation{{Position: "12321"}}
	checkErr := errors.New("smth") //nolint: goerr113

	m.On("RESTMapping", schema.GroupKind{Group: "", Kind: "kind"}, []string{"3"}).Once().Return(&meta.RESTMapping{}, nil)
	s.On("Check", ctx, owner, obj).Once().Return(checkVios, checkErr)

	violations, err := preflight.NewAPIExistence(m, s).Check(ctx, owner, obj)

	require.ErrorIs(t, err, checkErr)
	require.Equal(t, checkVios, violations)
}

func TestAPIExistenceNotExists(t *testing.T) {
	t.Parallel()

	m := &mapper{}
	s := &sub{}
	ctx := t.Context()
	owner := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "blorb"}}
	obj := &appsv1.Deployment{TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "3"}}

	checkVios := []preflight.Violation{{Position: "kind /", Error: "/3, Kind=kind not registered on the api server."}}

	m.On(
		"RESTMapping", schema.GroupKind{Group: "", Kind: "kind"}, []string{"3"},
	).Once().Return(&meta.RESTMapping{}, &meta.NoResourceMatchError{})
	violations, err := preflight.NewAPIExistence(m, s).Check(ctx, owner, obj)

	require.NoError(t, err)
	require.Equal(t, checkVios, violations)
}

func TestAPIExistenceErr(t *testing.T) {
	t.Parallel()

	m := &mapper{}
	s := &sub{}
	ctx := t.Context()
	owner := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "blorb"}}
	obj := &appsv1.Deployment{TypeMeta: metav1.TypeMeta{Kind: "kind", APIVersion: "3"}}

	checkErr := errors.New("smth") //nolint: goerr113

	m.On(
		"RESTMapping", schema.GroupKind{Group: "", Kind: "kind"}, []string{"3"},
	).Once().Return(&meta.RESTMapping{}, checkErr)

	violations, err := preflight.NewAPIExistence(m, s).Check(ctx, owner, obj)

	require.ErrorIs(t, err, checkErr)
	require.Empty(t, violations)
}
