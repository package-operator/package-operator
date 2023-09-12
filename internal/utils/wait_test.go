package utils_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apps "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"package-operator.run/internal/testutil"
	"package-operator.run/internal/utils"
)

func TestConditionFnNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := testutil.NewClient()
	obj := &apps.Deployment{ObjectMeta: v1.ObjectMeta{Name: "thename", Namespace: "thenamespace"}}
	cond := utils.ConditionFnNotFound(c, obj)

	c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil)
	done, err := cond(ctx)
	require.NoError(t, err)
	require.False(t, done)

	targetErr := errors.New("cheese error") //nolint:goerr113
	c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(targetErr)
	done, err = cond(ctx)
	require.ErrorIs(t, err, targetErr)
	require.False(t, done)

	c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().
		Return(k8serrors.NewNotFound(schema.GroupResource{}, ""))
	done, err = cond(ctx)
	require.NoError(t, err)
	require.True(t, done)
}
