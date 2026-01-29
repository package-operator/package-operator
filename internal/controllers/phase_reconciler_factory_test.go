package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/managedcachemocks"
	"package-operator.run/internal/testutil/ownerhandlingmocks"
)

func TestNewPhaseReconcilerFactory(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	uncachedClient := testutil.NewClient()
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	preflightChecker := &preflightCheckerMock{}

	factory := NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, preflightChecker)

	require.NotNil(t, factory)
	assert.IsType(t, phaseReconcilerFactory{}, factory)
}

func TestPhaseReconcilerFactory_New(t *testing.T) {
	t.Parallel()

	t.Run("creates new phase reconciler", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, preflightChecker)
		reconciler := factory.New(accessor)

		require.NotNil(t, reconciler)
		assert.IsType(t, &phaseReconciler{}, reconciler)
	})

	t.Run("creates reconciler with correct dependencies", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, preflightChecker)
		reconciler := factory.New(accessor)

		pr, ok := reconciler.(*phaseReconciler)
		require.True(t, ok, "reconciler should be of type *phaseReconciler")

		assert.Equal(t, scheme, pr.scheme)
		assert.Equal(t, accessor, pr.accessor)
		assert.Equal(t, uncachedClient, pr.uncachedClient)
		assert.Equal(t, ownerStrategy, pr.ownerStrategy)
		assert.NotNil(t, pr.adoptionChecker)
		assert.NotNil(t, pr.patcher)
		assert.Equal(t, preflightChecker, pr.preflightChecker)
	})

	t.Run("creates adoption checker", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, preflightChecker)
		reconciler := factory.New(accessor)

		pr, ok := reconciler.(*phaseReconciler)
		require.True(t, ok)

		adoptionChecker, ok := pr.adoptionChecker.(*defaultAdoptionChecker)
		require.True(t, ok, "adoption checker should be of type *defaultAdoptionChecker")

		assert.Equal(t, ownerStrategy, adoptionChecker.ownerStrategy)
		assert.Equal(t, scheme, adoptionChecker.scheme)
	})

	t.Run("creates patcher", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, preflightChecker)
		reconciler := factory.New(accessor)

		pr, ok := reconciler.(*phaseReconciler)
		require.True(t, ok)

		patcher, ok := pr.patcher.(*defaultPatcher)
		require.True(t, ok, "patcher should be of type *defaultPatcher")

		assert.Equal(t, accessor, patcher.writer)
	})

	t.Run("multiple calls create separate instances", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		accessor1 := &managedcachemocks.AccessorMock{}
		accessor2 := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, preflightChecker)
		reconciler1 := factory.New(accessor1)
		reconciler2 := factory.New(accessor2)

		require.NotNil(t, reconciler1)
		require.NotNil(t, reconciler2)

		// Verify they are different instances
		assert.NotSame(t, reconciler1, reconciler2)

		// Verify they have different accessors
		pr1, ok := reconciler1.(*phaseReconciler)
		require.True(t, ok)
		pr2, ok := reconciler2.(*phaseReconciler)
		require.True(t, ok)

		assert.Equal(t, accessor1, pr1.accessor)
		assert.Equal(t, accessor2, pr2.accessor)
		assert.NotSame(t, pr1.accessor, pr2.accessor)
	})
}

func TestPhaseReconcilerFactoryInterface(t *testing.T) {
	t.Parallel()

	t.Run("implements PhaseReconcilerFactory interface", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}

		_ = NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, preflightChecker)
	})
}

func TestPhaseReconcilerFactory_WithNilDependencies(t *testing.T) {
	t.Parallel()

	t.Run("handles nil scheme", func(t *testing.T) {
		t.Parallel()

		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(nil, uncachedClient, ownerStrategy, preflightChecker)
		reconciler := factory.New(accessor)

		require.NotNil(t, reconciler)
		pr, ok := reconciler.(*phaseReconciler)
		require.True(t, ok)
		assert.Nil(t, pr.scheme)
	})

	t.Run("handles nil uncachedClient", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(scheme, nil, ownerStrategy, preflightChecker)
		reconciler := factory.New(accessor)

		require.NotNil(t, reconciler)
		pr, ok := reconciler.(*phaseReconciler)
		require.True(t, ok)
		assert.Nil(t, pr.uncachedClient)
	})

	t.Run("handles nil ownerStrategy", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		preflightChecker := &preflightCheckerMock{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(scheme, uncachedClient, nil, preflightChecker)
		reconciler := factory.New(accessor)

		require.NotNil(t, reconciler)
		pr, ok := reconciler.(*phaseReconciler)
		require.True(t, ok)
		assert.Nil(t, pr.ownerStrategy)
	})

	t.Run("handles nil preflightChecker", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, nil)
		reconciler := factory.New(accessor)

		require.NotNil(t, reconciler)
		pr, ok := reconciler.(*phaseReconciler)
		require.True(t, ok)
		assert.Nil(t, pr.preflightChecker)
	})

	t.Run("handles nil accessor in New", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}

		factory := NewPhaseReconcilerFactory(scheme, uncachedClient, ownerStrategy, preflightChecker)
		reconciler := factory.New(nil)

		require.NotNil(t, reconciler)
		pr, ok := reconciler.(*phaseReconciler)
		require.True(t, ok)
		assert.Nil(t, pr.accessor)
	})
}
