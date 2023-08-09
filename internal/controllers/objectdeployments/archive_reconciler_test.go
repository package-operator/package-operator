package objectdeployments

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/testutil"
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
		prevs := make([]genericObjectSet, 2)
		for i := range prevs {
			obj := &genericObjectSetMock{}
			obj.AssertNotCalled(t, "IsAvailable")
			prevs[i] = obj
		}

		objectDeployment := &genericObjectDeploymentMock{}
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
		arch1 := &genericObjectSetMock{}
		arch2 := &genericObjectSetMock{}
		latestAvailable := &genericObjectSetMock{}

		// Return same revision number for all
		arch1.On("GetRevision").Return(int64(1))
		arch2.On("GetRevision").Return(int64(1))
		latestAvailable.On("GetRevision").Return(int64(1))

		// Set empty client objects for all
		// so that creation timestamp is also same
		arch1.On("ClientObject").Return(&unstructured.Unstructured{})
		arch2.On("ClientObject").Return(&unstructured.Unstructured{})
		latestAvailable.On("ClientObject").Return(&unstructured.Unstructured{})

		arch1.On("IsArchived").Return(false)
		arch2.On("IsArchived").Return(false)
		latestAvailable.On("IsAvailable").Return(true)
		prevs := []genericObjectSet{
			arch1,
			arch2,
		}
		objectDeployment := &genericObjectDeploymentMock{}
		res, err := r.Reconcile(ctx, latestAvailable, prevs, objectDeployment)
		require.NoError(t, err)
		assert.True(t, res.IsZero(), "unexpected requeue")
		testClient.AssertNotCalled(t, "Update")
		latestAvailable.AssertCalled(t, "IsAvailable")
		arch1.AssertNotCalled(t, "IsStatusPaused")
		arch1.AssertNotCalled(t, "IsSpecPaused")
		arch1.AssertNotCalled(t, "SetPaused")
		arch2.AssertNotCalled(t, "IsStatusPaused")
		arch2.AssertNotCalled(t, "IsSpecPaused")
		arch2.AssertNotCalled(t, "SetPaused")
	})

	t.Run("when the latest revision becomes available, all intermediate revisions are paused first", func(t *testing.T) {
		t.Parallel()
		testPauseAndArchivalWhenLatestIsAvailable(t, false)
	})

	t.Run("when the latest revision becomes available, all intermediate revisions are archived(if they are already paused)",
		func(t *testing.T) {
			t.Parallel()
			testPauseAndArchivalWhenLatestIsAvailable(t, true)
		})

	t.Run("archives intermediate revision/s if they are not available and dont actively reconcile anything present in later revisions",
		func(t *testing.T) {
			t.Parallel()
			testPauseAndArchivalIntermediateRevisions(t, false)
			testPauseAndArchivalIntermediateRevisions(t, true)
		})

	t.Run("Doesnt archive anything if there are errors when pausing revision/s to be archived", func(t *testing.T) {
		t.Parallel()
		// setup Objectdeployment
		objectDeployment := &genericObjectDeploymentMock{}
		revisionLimit := int32(10)
		objectDeployment.On("GetRevisionHistoryLimit").Return(&revisionLimit)

		// Setup revisions

		// Unavailable
		latestRevision := makeObjectSetMock(
			8,
			"",
			makeControllerOfObjects("a", "b"),
			makeObjects("a", "b"),
			nil,
			false,
			false,
			false,
			Unavailable,
		)

		prevs := []*genericObjectSetMock{
			// should be archived
			makeObjectSetMock(7, "", makeControllerOfObjects("c"), makeObjects("c", "a", "d"), nil, false, false, false, Unavailable),
			// Should not be archived
			makeObjectSetMock(6, "", makeControllerOfObjects("d", "e"), makeObjects("c", "a", "d", "e"), nil, false, false, false, Unavailable),
			// Should be archived
			makeObjectSetMock(5, "", makeControllerOfObjects("f"), makeObjects("f", "g"), nil, false, false, false, Unavailable),
			// Should not be archived
			makeObjectSetMock(4, "", makeControllerOfObjects("g"), makeObjects("g", "h"), nil, false, false, false, Unavailable),
			// No common objects but should not be archived as its available
			makeObjectSetMock(3, "", makeControllerOfObjects("x", "y"), makeObjects("x", "y"), nil, false, false, false, available),
			// Even though the rev 5, 6 have common objects, we expect them to be paused/archived as rev 4 is available
			makeObjectSetMock(2, "", makeControllerOfObjects("p"), makeObjects("p", "q"), nil, false, false, false, Unavailable),
			makeObjectSetMock(1, "", makeControllerOfObjects("q"), makeObjects("q", "z"), nil, false, false, false, Unavailable),
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

		prevCasted := make([]genericObjectSet, len(prevs))
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

func assertShouldNotBeArchived(t *testing.T, obj *genericObjectSetMock) {
	t.Helper()
	obj.AssertNotCalled(t, "SetArchived")
}

func assertShouldBeArchived(t *testing.T, obj *genericObjectSetMock) {
	t.Helper()
	obj.AssertNumberOfCalls(t, "SetArchived", 1)
	obj.AssertCalled(t, "SetArchived")
}

func assertShouldBePaused(t *testing.T, obj *genericObjectSetMock) {
	t.Helper()
	obj.AssertNumberOfCalls(t, "SetPaused", 1)
	obj.AssertCalled(t, "SetPaused")
}

func assertShouldNotBePaused(t *testing.T, obj *genericObjectSetMock) {
	t.Helper()
	obj.AssertNotCalled(t, "SetPaused")
}

func testPauseAndArchivalIntermediateRevisions(t *testing.T, alreadyPaused bool) {
	t.Helper()
	// setup Objectdeployment
	objectDeployment := &genericObjectDeploymentMock{}
	revisionLimit := int32(10)
	objectDeployment.On("GetRevisionHistoryLimit").Return(&revisionLimit)

	// Setup client
	client := testutil.NewClient()
	client.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Setup revisions

	// Unavailable
	latestRevision := makeObjectSetMock(
		8,
		"",
		makeControllerOfObjects("a", "b"),
		makeObjects("a", "b"),
		nil,
		false,
		false,
		false,
		Unavailable,
	)

	prevs := []*genericObjectSetMock{
		// should be archived
		makeObjectSetMock(7, "", makeControllerOfObjects("c"), makeObjects("c", "a", "d"), nil, alreadyPaused, false, false, Unavailable),
		// Should not be archived
		makeObjectSetMock(6, "", makeControllerOfObjects("d", "e"), makeObjects("c", "a", "d", "e"), nil, alreadyPaused, false, false, Unavailable),
		// Should be archived
		makeObjectSetMock(5, "", makeControllerOfObjects("f"), makeObjects("f", "g"), nil, alreadyPaused, false, false, Unavailable),
		// Should not be archived
		makeObjectSetMock(4, "", makeControllerOfObjects("g"), makeObjects("g", "h"), nil, alreadyPaused, false, false, Unavailable),
		// No common objects but should not be archived as its available
		makeObjectSetMock(3, "", makeControllerOfObjects("x", "y"), makeObjects("x", "y"), nil, alreadyPaused, false, false, available),
		// Even though the rev 5, 6 have common objects, we expect them to be paused/archived as rev 4 is available
		makeObjectSetMock(2, "", makeControllerOfObjects("p"), makeObjects("p", "q"), nil, alreadyPaused, false, false, Unavailable),
		makeObjectSetMock(1, "", makeControllerOfObjects("q"), makeObjects("q", "z"), nil, alreadyPaused, false, false, Unavailable),
	}

	prevCasted := make([]genericObjectSet, len(prevs))
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

	assertPausedOrArchived := func(alreadyPaused bool, revision *genericObjectSetMock) {
		if alreadyPaused {
			assertShouldBeArchived(t, revision)
		} else {
			assertShouldBePaused(t, revision)
		}
	}

	assertNotPausedOrArchived := func(alreadyPaused bool, revision *genericObjectSetMock) {
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
	objectDeployment := &genericObjectDeploymentMock{}
	revisionLimit := int32(10)
	objectDeployment.On("GetRevisionHistoryLimit").Return(&revisionLimit)

	// Setup client
	client := testutil.NewClient()
	client.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Setup revisions
	latestAvailableRevision := makeObjectSetMock(5, "", nil, nil, nil, false, false, false, true)

	prevs := []*genericObjectSetMock{
		makeObjectSetMock(3, "", nil, nil, nil, alreadyPaused, false, false, false),
		// This intermediate is already archived so the reconciler
		// should leave it alone.
		makeObjectSetMock(1, "", nil, nil, nil, alreadyPaused, false, true, false),
		makeObjectSetMock(2, "", nil, nil, nil, alreadyPaused, false, false, false),
	}

	prevCasted := make([]genericObjectSet, len(prevs))
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
	latestAvailableRevision.AssertNotCalled(t, "SetPaused")
	latestAvailableRevision.AssertNotCalled(t, "SetArchived")

	// prevs[0],prevs[2] is paused/archived
	if alreadyPaused {
		prevs[0].AssertCalled(t, "SetArchived")
		prevs[0].AssertNumberOfCalls(t, "SetArchived", 1)

		prevs[2].AssertCalled(t, "SetArchived")
		prevs[2].AssertNumberOfCalls(t, "SetArchived", 1)
	} else {
		prevs[0].AssertCalled(t, "SetPaused")
		prevs[0].AssertNumberOfCalls(t, "SetPaused", 1)

		prevs[2].AssertCalled(t, "SetPaused")
		prevs[2].AssertNumberOfCalls(t, "SetPaused", 1)
	}

	// Since prevs[1] is already archived, it is left alone
	prevs[1].AssertNotCalled(t, "SetPaused")
	prevs[1].AssertNotCalled(t, "SetArchived")
}

func testDeleteArchive(t *testing.T) {
	t.Helper()
	// setup Objectdeployment
	objectDeployment := &genericObjectDeploymentMock{}
	revisionLimit := int32(3)
	objectDeployment.On("GetRevisionHistoryLimit").Return(&revisionLimit)

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
	latestAvailableRevision := makeObjectSetMock(5, "", nil, nil, nil, false, false, false, true)

	prevs := []*genericObjectSetMock{
		// Already archived
		makeObjectSetMock(1, "", nil, nil, nil, true, false, true, false),
		makeObjectSetMock(2, "", nil, nil, nil, true, false, true, false),
		makeObjectSetMock(2, "", nil, nil, nil, true, false, true, false),
		// Paused and ready to be archived
		makeObjectSetMock(4, "", nil, nil, nil, true, false, false, false),
	}

	prevCasted := make([]genericObjectSet, len(prevs))
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

	prevs[3].AssertCalled(t, "SetArchived")
	prevs[3].AssertNumberOfCalls(t, "SetArchived", 1)

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

func makeObjectSetMock(
	revision int,
	hash string,
	activeReconciled []objectIdentifier,
	objects []objectIdentifier,
	getObjectsErr error,
	isStatusPaused bool,
	isSpecPaused bool,
	isArchived bool,
	isAvailable bool,
) *genericObjectSetMock {
	mock := &genericObjectSetMock{}
	mock.On("GetRevision").Return(int64(revision))
	clientObj := &unstructured.Unstructured{}
	clientObj.SetAnnotations(map[string]string{
		"important_for_mock_to_not_confuse_calls": fmt.Sprint(revision),
		// Use revision as the hash in tests
		ObjectSetHashAnnotation: hash,
	})
	mock.On("ClientObject").Return(clientObj)
	mock.On("GetActivelyReconciledObjects").Return(activeReconciled)
	mock.On("GetObjects").Return(objects, getObjectsErr)
	mock.On("IsStatusPaused").Return(isStatusPaused)
	mock.On("IsSpecPaused").Return(isSpecPaused)
	mock.On("IsAvailable").Return(isAvailable)
	mock.On("IsArchived").Return(isArchived)
	mock.On("SetPaused").Return()
	mock.On("SetArchived").Return()
	return mock
}
