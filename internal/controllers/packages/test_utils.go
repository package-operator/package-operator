package packages

import (
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

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

var _ ownerStrategy = (*mockOwnerStrategy)(nil)

type mockOwnerStrategy struct {
	mock.Mock
}

func (s *mockOwnerStrategy) EnqueueRequestForOwner(
	ownerType client.Object, isController bool,
) handler.EventHandler {
	return nil
}

func (s *mockOwnerStrategy) SetControllerReference(owner, obj metav1.Object) error {
	args := s.Called(owner, obj)
	return args.Error(0)
}

func (s *mockOwnerStrategy) IsOwner(owner, obj metav1.Object) bool {
	return false
}

func (s *mockOwnerStrategy) ReleaseController(obj metav1.Object) {
}
