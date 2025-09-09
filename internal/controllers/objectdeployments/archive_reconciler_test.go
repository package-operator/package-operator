package objectdeployments

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/adaptermocks"
)

const (
	Unavailable bool = false
	available   bool = true
)

func Test_ArchivalReconciler(t *testing.T) {
	t.Parallel()

	t.Run("Doesnt do anything if current revision is not present", func(t *testing.T) {
		t.Parallel()
		testClient := testutil.NewClient()

		r := archiveReconciler{
			client: testClient,
		}

		ctx := context.Background()
		prevs := make([]adapters.ObjectSetAccessor, 2)
		for i := range prevs {
			obj := &adaptermocks.ObjectSetMock{}
			obj.AssertNotCalled(t, "IsSpecAvailable")
			prevs[i] = obj
		}

		objectDeployment := &adaptermocks.ObjectDeploymentMock{}
		res, err := r.Reconcile(ctx, nil, prevs, objectDeployment)
		require.NoError(t, err)
		assert.True(t, res.IsZero(), "unexpected requeue")
	})

	t.Run("Doesnt archive anything if archival candidates dont follow the revision ordering", func(t *testing.T) {
		t.Parallel()
		testClient := testutil.NewClient()

		r := archiveReconciler{
			client: testClient,
		}

		ctx := context.Background()
		arch1 := &adaptermocks.ObjectSetMock{}
		arch2 := &adaptermocks.ObjectSetMock{}
		latestAvailable := &adaptermocks.ObjectSetMock{}

		// Return same revision number for all
		arch1.On("GetSpecRevision").Return(int64(1))
		arch2.On("GetSpecRevision").Return(int64(1))
		latestAvailable.On("GetSpecRevision").Return(int64(1))

		// Set empty client objects for all
		// so that creation timestamp is also same
		arch1.On("ClientObject").Return(&unstructured.Unstructured{})
		arch2.On("ClientObject").Return(&unstructured.Unstructured{})
		latestAvailable.On("ClientObject").Return(&unstructured.Unstructured{})

		arch1.On("IsSpecArchived").Return(false)
		arch2.On("IsSpecArchived").Return(false)
		latestAvailable.On("IsSpecAvailable").Return(true)
		prevs := []adapters.ObjectSetAccessor{
			arch1,
			arch2,
		}
		objectDeployment := &adaptermocks.ObjectDeploymentMock{}
		res, err := r.Reconcile(ctx, latestAvailable, prevs, objectDeployment)
		require.NoError(t, err)
		assert.True(t, res.IsZero(), "unexpected requeue")
		testClient.AssertNotCalled(t, "Update")
		latestAvailable.AssertCalled(t, "IsSpecAvailable")
		arch1.AssertNotCalled(t, "IsStatusPaused")
		arch1.AssertNotCalled(t, "IsSpecPaused")
		arch1.AssertNotCalled(t, "SetSpecPaused")
		arch2.AssertNotCalled(t, "IsStatusPaused")
		arch2.AssertNotCalled(t, "IsSpecPaused")
		arch2.AssertNotCalled(t, "SetSpecPaused")
	})

	t.Run(
		"when the latest revision becomes available, all intermediate revisions are paused first", func(t *testing.T) {
			t.Parallel()
			testPauseAndArchivalWhenLatestIsAvailable(t, false)
		})

	t.Run(
		"when the latest revision becomes available, all intermediate revisions are archived(if they are already paused)",
		func(t *testing.T) {
			t.Parallel()
			testPauseAndArchivalWhenLatestIsAvailable(t, true)
		})

	t.Run(
		"archive intermediate revision/s if they aren't available and don't reconcile anything present in later revisions",
		func(t *testing.T) {
			t.Parallel()
			testPauseAndArchivalIntermediateRevisions(t, false)
			testPauseAndArchivalIntermediateRevisions(t, true)
		})

	t.Run(
		"does not archive anything if there are errors when pausing revision/s to be archived", func(t *testing.T) {
			t.Parallel()
			// setup Objectdeployment
			objectDeployment := &adaptermocks.ObjectDeploymentMock{}
			revisionLimit := int32(10)
			objectDeployment.On("GetSpecRevisionHistoryLimit").Return(&revisionLimit)

			// Setup revisions

			// Unavailable
			latestRevision := newObjectSetMock(
				8,
				makeControllerOfObjects("a", "b"),
				makeObjects("a", "b"),
				false,
				false,
				Unavailable,
			)

			prevs := []*adaptermocks.ObjectSetMock{
				// should be archived
				newObjectSetMock(
					7, makeControllerOfObjects("c"), makeObjects("c", "a", "d"),
					false, false, Unavailable,
				),
				// Should not be archived
				newObjectSetMock(
					6, makeControllerOfObjects("d", "e"), makeObjects("c", "a", "d", "e"),
					false, false, Unavailable,
				),
				// Should be archived
				newObjectSetMock(5, makeControllerOfObjects("f"), makeObjects("f", "g"),
					false, false, Unavailable),
				// Should not be archived
				newObjectSetMock(
					4, makeControllerOfObjects("g"), makeObjects("g", "h"),
					false, false, Unavailable,
				),
				// No common objects but should not be archived as its available
				newObjectSetMock(
					3, makeControllerOfObjects("x", "y"), makeObjects("x", "y"),
					false, false, available,
				),
				// Even though the rev 5, 6 have common objects, we expect them to be paused/archived as rev 4 is available
				newObjectSetMock(
					2, makeControllerOfObjects("p"), makeObjects("p", "q"),
					false, false, Unavailable,
				),
				newObjectSetMock(
					1, makeControllerOfObjects("q"), makeObjects("q", "z"),
					false, false, Unavailable,
				),
			}

			// Setup client

			// Return an error when updating to pause revision 5
			client := testutil.NewClient()
			client.On("Update",
				mock.Anything,
				prevs[2].ClientObject(),
				mock.Anything,
			).Return(errors.New("Failed to update revision 5 for pausing"))

			// No errors on other updates
			client.On("Update",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(nil)

			prevCasted := make([]adapters.ObjectSetAccessor, len(prevs))
			for i := range prevs {
				prevCasted[i] = prevs[i]
			}

			// Invoke reconciler
			r := archiveReconciler{
				client: client,
			}
			res, err := r.Reconcile(context.Background(), latestRevision, prevCasted, objectDeployment)
			require.ErrorContains(t, err, "Failed to update revision 5 for pausing")
			assert.True(t, res.IsZero(), "Unexpected requeue")

			// Revision 7 should be paused
			assertShouldBePaused(t, prevs[0])

			// Revision 5's setpause method is called, but the following update fails.
			// So remaining eligible revisions are not paused.
			assertShouldBePaused(t, prevs[2])

			// Remaining not paused
			for _, rev := range prevs[3:] {
				assertShouldNotBePaused(t, rev)
			}

			// None of them should be archived
			for _, rev := range prevs {
				assertShouldNotBeArchived(t, rev)
			}
		})

	t.Run("It deletes older revisions over the revisionhistorylimit", func(t *testing.T) {
		t.Parallel()
		testDeleteArchive(t)
	})
}

// t(-_-t).
func contains(source []int, obj int) bool {
	for _, item := range source {
		if item == obj {
			return true
		}
	}
	return false
}

func assertShouldNotBeArchived(t *testing.T, obj *adaptermocks.ObjectSetMock) {
	t.Helper()
	obj.AssertNotCalled(t, "SetSpecArchived")
}

func assertShouldBeArchived(t *testing.T, obj *adaptermocks.ObjectSetMock) {
	t.Helper()
	obj.AssertNumberOfCalls(t, "SetSpecArchived", 1)
	obj.AssertCalled(t, "SetSpecArchived")
}

func assertShouldBePaused(t *testing.T, obj *adaptermocks.ObjectSetMock) {
	t.Helper()
	obj.AssertNumberOfCalls(t, "SetSpecPaused", 1)
	obj.AssertCalled(t, "SetSpecPaused")
}

func assertShouldNotBePaused(t *testing.T, obj *adaptermocks.ObjectSetMock) {
	t.Helper()
	obj.AssertNotCalled(t, "SetSpecPaused")
}

func testPauseAndArchivalIntermediateRevisions(t *testing.T, alreadyPaused bool) {
	t.Helper()
	// setup Objectdeployment
	objectDeployment := &adaptermocks.ObjectDeploymentMock{}
	revisionLimit := int32(10)
	objectDeployment.On("GetSpecRevisionHistoryLimit").Return(&revisionLimit)

	// Setup client
	client := testutil.NewClient()
	client.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Setup revisions

	// Unavailable
	latestRevision := newObjectSetMock(
		8,
		makeControllerOfObjects("a", "b"),
		makeObjects("a", "b"),
		false,
		false,
		Unavailable,
	)

	prevs := []*adaptermocks.ObjectSetMock{
		// should be archived
		newObjectSetMock(
			7, makeControllerOfObjects("c"), makeObjects("c", "a", "d"),
			alreadyPaused, false, Unavailable,
		),
		// Should not be archived
		newObjectSetMock(
			6, makeControllerOfObjects("d", "e"), makeObjects("c", "a", "d", "e"),
			alreadyPaused, false, Unavailable,
		),
		// Should be archived
		newObjectSetMock(
			5, makeControllerOfObjects("f"), makeObjects("f", "g"),
			alreadyPaused, false, Unavailable,
		),
		// Should not be archived
		newObjectSetMock(
			4, makeControllerOfObjects("g"), makeObjects("g", "h"),
			alreadyPaused, false, Unavailable,
		),
		// No common objects but should not be archived as its available
		newObjectSetMock(
			3, makeControllerOfObjects("x", "y"), makeObjects("x", "y"),
			alreadyPaused, false, available,
		),
		// Even though the rev 5, 6 have common objects, we expect them to be paused/archived as rev 4 is available
		newObjectSetMock(
			2, makeControllerOfObjects("p"), makeObjects("p", "q"),
			alreadyPaused, false, Unavailable,
		),
		newObjectSetMock(
			1, makeControllerOfObjects("q"), makeObjects("q", "z"),
			alreadyPaused, false, Unavailable,
		),
	}

	prevCasted := make([]adapters.ObjectSetAccessor, len(prevs))
	for i := range prevs {
		prevCasted[i] = prevs[i]
	}

	// Invoke reconciler
	r := archiveReconciler{
		client: client,
	}
	res, err := r.Reconcile(context.Background(), latestRevision, prevCasted, objectDeployment)
	require.NoError(t, err)
	assert.True(t, res.IsZero(), "Unexpected requeue")

	// Client assertions
	client.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
	client.AssertNumberOfCalls(t, "Update", 4)

	// ---------------------------------------------------------------------------------------------------
	// Revision assertions
	// ---------------------------------------------------------------------------------------------------

	// Latest revision is left alone
	latestRevision.AssertNotCalled(t, "IsStatusPaused")
	latestRevision.AssertNotCalled(t, "IsSpecPaused")
	assertShouldNotBeArchived(t, latestRevision)
	assertShouldNotBePaused(t, latestRevision)

	expectedRevisionsToBeArchivedOrPaused := []int{0, 2, 5, 6}

	assertPausedOrArchived := func(alreadyPaused bool, revision *adaptermocks.ObjectSetMock) {
		if alreadyPaused {
			assertShouldBeArchived(t, revision)
		} else {
			assertShouldBePaused(t, revision)
		}
	}

	assertNotPausedOrArchived := func(alreadyPaused bool, revision *adaptermocks.ObjectSetMock) {
		if alreadyPaused {
			assertShouldNotBeArchived(t, revision)
		} else {
			assertShouldNotBePaused(t, revision)
		}
	}

	for revNumber, rev := range prevs {
		if contains(expectedRevisionsToBeArchivedOrPaused, revNumber) {
			assertPausedOrArchived(alreadyPaused, rev)
		} else {
			assertNotPausedOrArchived(alreadyPaused, rev)
		}
	}
}

func testPauseAndArchivalWhenLatestIsAvailable(t *testing.T, alreadyPaused bool) {
	t.Helper()
	// setup Objectdeployment
	objectDeployment := &adaptermocks.ObjectDeploymentMock{}
	revisionLimit := int32(10)
	objectDeployment.On("GetSpecRevisionHistoryLimit").Return(&revisionLimit)

	// Setup client
	client := testutil.NewClient()
	client.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Setup revisions
	latestAvailableRevision := newObjectSetMock(5, nil, nil, false, false, true)

	prevs := []*adaptermocks.ObjectSetMock{
		newObjectSetMock(3, nil, nil, alreadyPaused, false, false),
		// This intermediate is already archived so the reconciler
		// should leave it alone.
		newObjectSetMock(1, nil, nil, alreadyPaused, true, false),
		newObjectSetMock(2, nil, nil, alreadyPaused, false, false),
	}

	prevCasted := make([]adapters.ObjectSetAccessor, len(prevs))
	for i := range prevs {
		prevCasted[i] = prevs[i]
	}

	// Invoke reconciler
	r := archiveReconciler{
		client: client,
	}
	res, err := r.Reconcile(context.Background(), latestAvailableRevision, prevCasted, objectDeployment)
	require.NoError(t, err)
	assert.True(t, res.IsZero(), "Unexpected requeue")

	// Client assertions
	client.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
	client.AssertNumberOfCalls(t, "Update", 2)

	// ---------------------------------------------------------------------------------------------------
	// Revision assertions
	// ---------------------------------------------------------------------------------------------------

	// Latest available is left alone
	latestAvailableRevision.AssertNotCalled(t, "IsStatusPaused")
	latestAvailableRevision.AssertNotCalled(t, "IsSpecPaused")
	latestAvailableRevision.AssertNotCalled(t, "SetSpecPaused")
	latestAvailableRevision.AssertNotCalled(t, "SetSpecArchived")

	// prevs[0],prevs[2] is paused/archived
	if alreadyPaused {
		prevs[0].AssertCalled(t, "SetSpecArchived")
		prevs[0].AssertNumberOfCalls(t, "SetSpecArchived", 1)

		prevs[2].AssertCalled(t, "SetSpecArchived")
		prevs[2].AssertNumberOfCalls(t, "SetSpecArchived", 1)
	} else {
		prevs[0].AssertCalled(t, "SetSpecPaused")
		prevs[0].AssertNumberOfCalls(t, "SetSpecPaused", 1)

		prevs[2].AssertCalled(t, "SetSpecPaused")
		prevs[2].AssertNumberOfCalls(t, "SetSpecPaused", 1)
	}

	// Since prevs[1] is already archived, it is left alone
	prevs[1].AssertNotCalled(t, "SetSpecPaused")
	prevs[1].AssertNotCalled(t, "SetSpecArchived")
}

func testDeleteArchive(t *testing.T) {
	t.Helper()
	// setup Objectdeployment
	objectDeployment := &adaptermocks.ObjectDeploymentMock{}
	revisionLimit := int32(3)
	objectDeployment.On("GetSpecRevisionHistoryLimit").Return(&revisionLimit)

	// Setup client
	client := testutil.NewClient()
	client.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// No errors on delete
	client.On("Delete",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Setup revisions
	latestAvailableRevision := newObjectSetMock(5, nil, nil, false, false, true)

	prevs := []*adaptermocks.ObjectSetMock{
		// Already archived
		newObjectSetMock(1, nil, nil, true, true, false),
		newObjectSetMock(2, nil, nil, true, true, false),
		newObjectSetMock(2, nil, nil, true, true, false),
		// Paused and ready to be archived
		newObjectSetMock(4, nil, nil, true, false, false),
	}

	prevCasted := make([]adapters.ObjectSetAccessor, len(prevs))
	for i := range prevs {
		prevCasted[i] = prevs[i]
	}

	// Invoke reconciler
	r := archiveReconciler{
		client: client,
	}
	res, err := r.Reconcile(context.Background(), latestAvailableRevision, prevCasted, objectDeployment)
	require.NoError(t, err)
	assert.True(t, res.IsZero(), "Unexpected requeue")

	// Client assertions
	client.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
	client.AssertNumberOfCalls(t, "Update", 1)

	prevs[3].AssertCalled(t, "SetSpecArchived")
	prevs[3].AssertNumberOfCalls(t, "SetSpecArchived", 1)

	client.AssertCalled(t,
		"Delete",
		mock.Anything,
		prevs[0].ClientObject(),
		mock.Anything,
	)
}

func makeObjectIdentifiers(ids ...string) []objectIdentifier {
	res := make([]objectIdentifier, len(ids))
	for i, id := range ids {
		res[i] = objectSetObjectIdentifier{
			name: id,
		}
	}
	return res
}

func makeObjects(ids ...string) []objectIdentifier {
	return makeObjectIdentifiers(ids...)
}

func makeControllerOfObjects(ids ...string) []objectIdentifier {
	return makeObjectIdentifiers(ids...)
}

func newObjectSetMock(
	revision int,
	activeReconciled []objectIdentifier,
	objects []objectIdentifier,
	isStatusPaused bool,
	isSpecArchived bool,
	isSpecAvailable bool,
) *adaptermocks.ObjectSetMock {
	mock := &adaptermocks.ObjectSetMock{}
	mock.On("GetStatusRevision").Return(int64(revision))
	mock.On("GetSpecRevision").Return(int64(revision))
	clientObj := &unstructured.Unstructured{}
	clientObj.SetAnnotations(map[string]string{
		"important_for_mock_to_not_confuse_calls": strconv.Itoa(revision),
		// Use revision as the hash in tests
		ObjectSetHashAnnotation: "",
	})
	mock.On("ClientObject").Return(clientObj)
	mock.On("GetActivelyReconciledObjects").Return(activeReconciled)
	mock.On("GetObjects").Return(objects, nil)
	mock.On("IsStatusPaused").Return(isStatusPaused)
	mock.On("IsSpecPaused").Return(false)
	mock.On("IsSpecAvailable").Return(isSpecAvailable)
	mock.On("IsSpecArchived").Return(isSpecArchived)
	mock.On("SetSpecPaused").Return()
	mock.On("SetSpecArchived").Return()
	return mock
}
