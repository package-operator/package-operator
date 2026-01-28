package hostedclusterpackages

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

func TestNewPackageStates(t *testing.T) {
	t.Parallel()

	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
	}

	ps := newPackageStates(hcpkg)

	require.NotNil(t, ps)
	assert.Equal(t, hcpkg, ps.hcpkg)
	assert.NotNil(t, ps.hcToPackage)
	assert.NotNil(t, ps.hostedClusters)
	assert.NotNil(t, ps.needsUpdate)
	assert.Equal(t, 0, ps.unavailablePkgs)
}

func TestPackageStates_Add(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		hcpkg                *corev1alpha1.HostedClusterPackage
		hc                   *hypershiftv1beta1.HostedCluster
		pkg                  *corev1alpha1.Package
		expectedUnavailable  int
		expectedNeedsUpdate  bool
		expectedPartitionKey string
	}{
		{
			name: "add available package with matching spec",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Template: corev1alpha1.PackageTemplateSpec{
						Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
					},
				},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hc",
					Namespace: "default",
					UID:       "hc-uid-1",
				},
			},
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pkg",
					Namespace:  "default",
					UID:        "pkg-uid-1",
					Generation: 1,
				},
				Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
				Status: corev1alpha1.PackageStatus{
					Conditions: []metav1.Condition{
						{
							Type:               corev1alpha1.PackageAvailable,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
						},
					},
				},
			},
			expectedUnavailable:  0,
			expectedNeedsUpdate:  false,
			expectedPartitionKey: defaultPartitionGroup,
		},
		{
			name: "add unavailable package",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Template: corev1alpha1.PackageTemplateSpec{
						Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
					},
				},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hc",
					Namespace: "default",
					UID:       "hc-uid-1",
				},
			},
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pkg",
					Namespace:  "default",
					UID:        "pkg-uid-1",
					Generation: 1,
				},
				Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
				Status: corev1alpha1.PackageStatus{
					Conditions: []metav1.Condition{
						{
							Type:               corev1alpha1.PackageAvailable,
							Status:             metav1.ConditionFalse,
							ObservedGeneration: 1,
						},
					},
				},
			},
			expectedUnavailable:  1,
			expectedNeedsUpdate:  false,
			expectedPartitionKey: defaultPartitionGroup,
		},
		{
			name: "add package needing update",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Template: corev1alpha1.PackageTemplateSpec{
						Spec: corev1alpha1.PackageSpec{Image: "test-image:v2"},
					},
				},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hc",
					Namespace: "default",
					UID:       "hc-uid-1",
				},
			},
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pkg",
					Namespace:  "default",
					UID:        "pkg-uid-1",
					Generation: 1,
				},
				Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
				Status: corev1alpha1.PackageStatus{
					Conditions: []metav1.Condition{
						{
							Type:               corev1alpha1.PackageAvailable,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
						},
					},
				},
			},
			expectedUnavailable:  0,
			expectedNeedsUpdate:  true,
			expectedPartitionKey: defaultPartitionGroup,
		},
		{
			name: "add package with partition label",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Template: corev1alpha1.PackageTemplateSpec{
						Spec: corev1alpha1.PackageSpec{Image: "test-image:v2"},
					},
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
					},
				},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hc",
					Namespace: "default",
					UID:       "hc-uid-1",
					Labels: map[string]string{
						"risk-group": "low-risk",
					},
				},
			},
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pkg",
					Namespace:  "default",
					UID:        "pkg-uid-1",
					Generation: 1,
				},
				Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
				Status: corev1alpha1.PackageStatus{
					Conditions: []metav1.Condition{
						{
							Type:               corev1alpha1.PackageAvailable,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
						},
					},
				},
			},
			expectedUnavailable:  0,
			expectedNeedsUpdate:  true,
			expectedPartitionKey: "low-risk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ps := newPackageStates(tt.hcpkg)
			ps.Add(tt.hc, tt.pkg)

			assert.Equal(t, tt.expectedUnavailable, ps.unavailablePkgs)
			assert.Equal(t, tt.pkg, ps.hcToPackage[tt.hc.UID])
			assert.Equal(t, tt.hc, ps.hostedClusters[tt.hc.UID])

			partitionKey := ps.partitionKey(tt.hc)
			assert.Equal(t, tt.expectedPartitionKey, partitionKey)

			if tt.expectedNeedsUpdate {
				assert.Contains(t, ps.needsUpdate[partitionKey], tt.pkg)
			} else {
				assert.NotContains(t, ps.needsUpdate[partitionKey], tt.pkg)
			}
		})
	}
}

func TestPackageStates_Missing(t *testing.T) {
	t.Parallel()

	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
	}
	hc := &hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hc",
			Namespace: "default",
			UID:       "hc-uid-1",
		},
	}

	ps := newPackageStates(hcpkg)
	ps.Missing(hc)

	assert.Equal(t, 1, ps.unavailablePkgs)
	assert.Equal(t, hc, ps.hostedClusters[hc.UID])
	assert.Nil(t, ps.hcToPackage[hc.UID])
}

func TestPackageStates_ListHostedClustersMissingPackage(t *testing.T) {
	t.Parallel()

	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
		Spec: corev1alpha1.HostedClusterPackageSpec{
			Template: corev1alpha1.PackageTemplateSpec{
				Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
			},
		},
	}

	hcMissing := &hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hc-missing",
			Namespace: "default",
			UID:       "hc-uid-missing",
		},
	}

	hcWithPackage := &hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hc-with-package",
			Namespace: "default",
			UID:       "hc-uid-with-package",
		},
	}

	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
			UID:       "pkg-uid-1",
		},
		Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
	}

	ps := newPackageStates(hcpkg)
	ps.Missing(hcMissing)
	ps.Add(hcWithPackage, pkg)

	missing := ps.ListHostedClustersMissingPackage()

	assert.Len(t, missing, 1)
	assert.Equal(t, "hc-missing", missing[0].Name)
}

func TestPackageStates_DisruptionBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		hcpkg           *corev1alpha1.HostedClusterPackage
		unavailablePkgs int
		expectedBudget  int
	}{
		{
			name: "instant strategy - budget disabled",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						Instant: &corev1alpha1.HostedClusterPackageStrategyInstant{},
					},
				},
			},
			unavailablePkgs: 0,
			expectedBudget:  -1,
		},
		{
			name: "no strategy - budget disabled",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
			},
			unavailablePkgs: 0,
			expectedBudget:  -1,
		},
		{
			name: "rolling upgrade - no unavailable packages",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
							MaxUnavailable: 3,
						},
					},
				},
			},
			unavailablePkgs: 0,
			expectedBudget:  3,
		},
		{
			name: "rolling upgrade - some unavailable packages",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
							MaxUnavailable: 5,
						},
					},
				},
			},
			unavailablePkgs: 2,
			expectedBudget:  3,
		},
		{
			name: "rolling upgrade - at max unavailable",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
							MaxUnavailable: 3,
						},
					},
				},
			},
			unavailablePkgs: 3,
			expectedBudget:  0,
		},
		{
			name: "rolling upgrade - over max unavailable",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
							MaxUnavailable: 2,
						},
					},
				},
			},
			unavailablePkgs: 5,
			expectedBudget:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ps := newPackageStates(tt.hcpkg)
			ps.unavailablePkgs = tt.unavailablePkgs

			budget := ps.DisruptionBudget()

			assert.Equal(t, tt.expectedBudget, budget)
		})
	}
}

func TestPackageStates_PartitionKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hcpkg       *corev1alpha1.HostedClusterPackage
		hc          *hypershiftv1beta1.HostedCluster
		expectedKey string
	}{
		{
			name: "no partition spec",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hc"},
			},
			expectedKey: defaultPartitionGroup,
		},
		{
			name: "partition spec but no labels on HC",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
					},
				},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hc"},
			},
			expectedKey: defaultPartitionGroup,
		},
		{
			name: "partition spec but missing label",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
					},
				},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-hc",
					Labels: map[string]string{"other": "value"},
				},
			},
			expectedKey: defaultPartitionGroup,
		},
		{
			name: "partition spec with matching label",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
					},
				},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-hc",
					Labels: map[string]string{"risk-group": "high-risk"},
				},
			},
			expectedKey: "high-risk",
		},
		{
			name: "partition spec with empty label value",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
					},
				},
			},
			hc: &hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-hc",
					Labels: map[string]string{"risk-group": ""},
				},
			},
			expectedKey: defaultPartitionGroup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ps := newPackageStates(tt.hcpkg)
			key := ps.partitionKey(tt.hc)

			assert.Equal(t, tt.expectedKey, key)
		})
	}
}

func TestPackageStates_PartitionList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		hcpkg         *corev1alpha1.HostedClusterPackage
		needsUpdate   map[string][]*corev1alpha1.Package
		expectedOrder []string
	}{
		{
			name: "no partition spec",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
			},
			needsUpdate: map[string][]*corev1alpha1.Package{
				defaultPartitionGroup: {{}},
			},
			expectedOrder: []string{defaultPartitionGroup},
		},
		{
			name: "alphanumeric ascending (default)",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
					},
				},
			},
			needsUpdate: map[string][]*corev1alpha1.Package{
				"low-risk":            {{}},
				"high-risk":           {{}},
				"medium-risk":         {{}},
				defaultPartitionGroup: {{}},
			},
			expectedOrder: []string{"high-risk", "low-risk", "medium-risk", defaultPartitionGroup},
		},
		{
			name: "alphanumeric ascending (explicit)",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
						Order: &corev1alpha1.HostedClusterPackagePartitionOrderSpec{
							AlphanumericAsc: &corev1alpha1.HostedClusterPackagePartitionOrderAlphanumericAsc{},
						},
					},
				},
			},
			needsUpdate: map[string][]*corev1alpha1.Package{
				"zone-c":              {{}},
				"zone-a":              {{}},
				"zone-b":              {{}},
				defaultPartitionGroup: {{}},
			},
			expectedOrder: []string{"zone-a", "zone-b", "zone-c", defaultPartitionGroup},
		},
		{
			name: "static ordering without wildcard",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
						Order: &corev1alpha1.HostedClusterPackagePartitionOrderSpec{
							Static: []string{"low-risk", "medium-risk", "high-risk"},
						},
					},
				},
			},
			needsUpdate: map[string][]*corev1alpha1.Package{
				"low-risk":    {{}},
				"high-risk":   {{}},
				"medium-risk": {{}},
			},
			expectedOrder: []string{"low-risk", "medium-risk", "high-risk"},
		},
		{
			name: "static ordering with wildcard at beginning",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
						Order: &corev1alpha1.HostedClusterPackagePartitionOrderSpec{
							Static: []string{"*", "low-risk", "medium-risk", "high-risk"},
						},
					},
				},
			},
			needsUpdate: map[string][]*corev1alpha1.Package{
				"low-risk":            {{}},
				"high-risk":           {{}},
				defaultPartitionGroup: {{}},
			},
			expectedOrder: []string{defaultPartitionGroup, "low-risk", "medium-risk", "high-risk"},
		},
		{
			name: "static ordering with wildcard in middle",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
						Order: &corev1alpha1.HostedClusterPackagePartitionOrderSpec{
							Static: []string{"low-risk", "*", "high-risk"},
						},
					},
				},
			},
			needsUpdate: map[string][]*corev1alpha1.Package{
				"low-risk":            {{}},
				"high-risk":           {{}},
				defaultPartitionGroup: {{}},
			},
			expectedOrder: []string{"low-risk", defaultPartitionGroup, "high-risk"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ps := newPackageStates(tt.hcpkg)
			ps.needsUpdate = tt.needsUpdate

			order := ps.partitionList()

			assert.Equal(t, tt.expectedOrder, order)
		})
	}
}

func TestPackageStates_ListPackagesToUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		hcpkg            *corev1alpha1.HostedClusterPackage
		setupState       func(*packageStates)
		expectedPkgCount int
		expectedPkgUIDs  []types.UID
	}{
		{
			name: "no packages need update",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
			},
			setupState: func(_ *packageStates) {
				// Empty state
			},
			expectedPkgCount: 0,
		},
		{
			name: "instant strategy - return all packages",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						Instant: &corev1alpha1.HostedClusterPackageStrategyInstant{},
					},
				},
			},
			setupState: func(ps *packageStates) {
				ps.needsUpdate[defaultPartitionGroup] = []*corev1alpha1.Package{
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-1"}},
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-2"}},
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-3"}},
				}
			},
			expectedPkgCount: 3,
		},
		{
			name: "rolling upgrade - respect disruption budget",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
							MaxUnavailable: 2,
						},
					},
				},
			},
			setupState: func(ps *packageStates) {
				ps.needsUpdate[defaultPartitionGroup] = []*corev1alpha1.Package{
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-1"}},
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-2"}},
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-3"}},
				}
			},
			expectedPkgCount: 2,
		},
		{
			name: "rolling upgrade - no budget available",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
							MaxUnavailable: 2,
						},
					},
				},
			},
			setupState: func(ps *packageStates) {
				ps.unavailablePkgs = 2
				ps.needsUpdate[defaultPartitionGroup] = []*corev1alpha1.Package{
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-1"}},
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-2"}},
				}
			},
			expectedPkgCount: 0,
		},
		{
			name: "packages in processing queue are prioritized",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
							MaxUnavailable: 2,
						},
					},
				},
				Status: corev1alpha1.HostedClusterPackageStatus{
					Processing: []corev1alpha1.HostedClusterPackageRefStatus{
						{UID: "pkg-2"},
					},
				},
			},
			setupState: func(ps *packageStates) {
				ps.hcToPackage["hc-1"] = &corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{UID: "pkg-1"},
				}
				ps.hcToPackage["hc-2"] = &corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{UID: "pkg-2"},
				}
				ps.hcToPackage["hc-3"] = &corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{UID: "pkg-3"},
				}
				ps.needsUpdate[defaultPartitionGroup] = []*corev1alpha1.Package{
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-1"}},
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-2"}},
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-3"}},
				}
			},
			expectedPkgCount: 2,
			expectedPkgUIDs:  []types.UID{"pkg-2", "pkg-1"}, // pkg-2 should be first (from processing queue)
		},
		{
			name: "partitions processed in order",
			hcpkg: &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Strategy: corev1alpha1.HostedClusterPackageStrategy{
						RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
							MaxUnavailable: 2,
						},
					},
					Partition: &corev1alpha1.HostedClusterPackagePartitionSpec{
						LabelKey: "risk-group",
						Order: &corev1alpha1.HostedClusterPackagePartitionOrderSpec{
							Static: []string{"low-risk", "high-risk"},
						},
					},
				},
			},
			setupState: func(ps *packageStates) {
				ps.needsUpdate["low-risk"] = []*corev1alpha1.Package{
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-low-1"}},
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-low-2"}},
				}
				ps.needsUpdate["high-risk"] = []*corev1alpha1.Package{
					{ObjectMeta: metav1.ObjectMeta{UID: "pkg-high-1"}},
				}
			},
			expectedPkgCount: 2,
			expectedPkgUIDs:  []types.UID{"pkg-low-1", "pkg-low-2"}, // Only low-risk packages
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ps := newPackageStates(tt.hcpkg)
			tt.setupState(ps)

			packages := ps.ListPackagesToUpdate()

			assert.Len(t, packages, tt.expectedPkgCount)

			if len(tt.expectedPkgUIDs) > 0 {
				actualUIDs := make([]types.UID, len(packages))
				for i, pkg := range packages {
					actualUIDs[i] = pkg.UID
				}
				assert.Equal(t, tt.expectedPkgUIDs, actualUIDs)
			}
		})
	}
}
