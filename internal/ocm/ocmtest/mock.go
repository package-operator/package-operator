package ocmtest

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/openshift/addon-operator/internal/ocm"
)

type Client struct {
	mock.Mock
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) PatchUpgradePolicy(
	ctx context.Context,
	req ocm.UpgradePolicyPatchRequest,
) (ocm.UpgradePolicyPatchResponse, error) {
	args := c.Called(ctx, req)
	return args.Get(0).(ocm.UpgradePolicyPatchResponse),
		args.Error(1)
}

func (c *Client) GetCluster(
	ctx context.Context,
	req ocm.ClusterGetRequest,
) (ocm.ClusterGetResponse, error) {
	args := c.Called(ctx, req)
	return args.Get(0).(ocm.ClusterGetResponse),
		args.Error(1)
}
