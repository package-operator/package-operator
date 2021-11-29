package ocm

import (
	"context"
	"fmt"
	"net/http"
)

type UpgradePolicyValue string

const (
	UpgradePolicyValueStarted   UpgradePolicyValue = "started"
	UpgradePolicyValueCompleted UpgradePolicyValue = "completed"
)

type UpgradePolicyPatchRequest struct {
	ID          string             `json:"id"`
	Value       UpgradePolicyValue `json:"value"`
	Description string             `json:"description"`
}

type UpgradePolicyPatchResponse struct{}

func (c *Client) PatchUpgradePolicy(
	ctx context.Context,
	req UpgradePolicyPatchRequest,
) (res UpgradePolicyPatchResponse, err error) {
	return res, c.do(ctx, http.MethodPatch, fmt.Sprintf(
		"api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state",
		c.opts.ClusterID,
		req.ID,
	),
		req,
		&res,
	)
}

type UpgradePolicyGetRequest struct {
	ID string `json:"id"`
}

type UpgradePolicyGetResponse struct {
	Value       UpgradePolicyValue `json:"value"`
	Description string             `json:"description"`
}

func (c *Client) GetUpgradePolicy(
	ctx context.Context,
	req UpgradePolicyGetRequest,
) (res UpgradePolicyGetResponse, err error) {
	return res, c.do(ctx, http.MethodGet, fmt.Sprintf(
		"api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state",
		c.opts.ClusterID,
		req.ID,
	),
		req,
		&res,
	)
}
