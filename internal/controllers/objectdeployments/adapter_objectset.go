package objectdeployments

import (
	"fmt"

	"github.com/stretchr/testify/mock"

	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil/adaptermocks"
	"package-operator.run/internal/utils"
)

type objectSetGetter interface {
	getActivelyReconciledObjects() []objectIdentifier
	getObjects() ([]objectIdentifier, error)
}

func newObjectSetGetter(objectSet adapters.ObjectSetAccessor) objectSetGetter {
	switch os := objectSet.(type) {
	case *adapters.ObjectSetAdapter:
		return &defaultObjectSetGetter{objectSet}
	case *adapters.ClusterObjectSetAdapter:
		return &defaultObjectSetGetter{objectSet}
	case *adaptermocks.ObjectSetMock:
		return &objectSetGetterMock{objectSet: os}
	default:
		panic("invalid objectSet type")
	}
}

type defaultObjectSetGetter struct {
	objectSet adapters.ObjectSetAccessor
}

func (os *defaultObjectSetGetter) getActivelyReconciledObjects() []objectIdentifier {
	res := make([]objectIdentifier, 0)
	if os.objectSet.IsSpecArchived() {
		// If an objectset is archived, it doesn't actively
		// reconcile anything, we just return an empty list
		return []objectIdentifier{}
	}

	if os.objectSet.GetStatusControllerOf() == nil {
		// ActivelyReconciledObjects status is not reported yet
		return nil
	}

	for _, reconciledObj := range os.objectSet.GetStatusControllerOf() {
		currentObj := objectSetObjectIdentifier{
			kind:      reconciledObj.Kind,
			group:     reconciledObj.Group,
			name:      reconciledObj.Name,
			namespace: reconciledObj.Namespace,
		}
		res = append(res, currentObj)
	}
	return res
}

func (os *defaultObjectSetGetter) getObjects() ([]objectIdentifier, error) {
	objects := utils.GetObjectsFromPhases(os.objectSet.GetSpecPhases())
	result := make([]objectIdentifier, len(objects))
	for i := range objects {
		unstructuredObj := objects[i].Object
		var objNamespace string
		if len(unstructuredObj.GetNamespace()) == 0 {
			objNamespace = os.objectSet.ClientObject().GetNamespace()
		} else {
			objNamespace = unstructuredObj.GetNamespace()
		}
		result[i] = objectSetObjectIdentifier{
			name:      unstructuredObj.GetName(),
			namespace: objNamespace,
			group:     unstructuredObj.GroupVersionKind().Group,
			kind:      unstructuredObj.GroupVersionKind().Kind,
		}
	}
	return result, nil
}

type objectSetGetterMock struct {
	objectSet *adaptermocks.ObjectSetMock
	mock.Mock
}

func (os *objectSetGetterMock) getActivelyReconciledObjects() []objectIdentifier {
	args := os.objectSet.MethodCalled("GetActivelyReconciledObjects")
	return args.Get(0).([]objectIdentifier)
}

func (os *objectSetGetterMock) getObjects() ([]objectIdentifier, error) {
	args := os.objectSet.MethodCalled("GetObjects")
	return args.Get(0).([]objectIdentifier), args.Error(1)
}

type objectIdentifier interface {
	UniqueIdentifier() string
}

type objectSetObjectIdentifier struct {
	kind      string
	name      string
	namespace string
	group     string
}

func (o objectSetObjectIdentifier) UniqueIdentifier() string {
	return fmt.Sprintf("%s/%s/%s/%s", o.group, o.kind, o.namespace, o.name)
}

type objectSetsByRevisionAscending []adapters.ObjectSetAccessor

func (a objectSetsByRevisionAscending) Len() int      { return len(a) }
func (a objectSetsByRevisionAscending) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a objectSetsByRevisionAscending) Less(i, j int) bool {
	iObj := a[i]
	jObj := a[j]

	return iObj.GetSpecRevision() < jObj.GetSpecRevision()
}
