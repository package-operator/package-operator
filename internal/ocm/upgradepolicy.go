package ocm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	urlParams := url.Values{}
	return res, c.do(ctx, http.MethodPatch, fmt.Sprintf(
		"api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state",
		c.opts.ClusterID,
		req.ID,
	),
		urlParams,
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
	urlParams := url.Values{}
	return res, c.do(ctx, http.MethodGet, fmt.Sprintf(
		"api/clusters_mgmt/v1/clusters/%s/upgrade_policies/%s/state",
		c.opts.ClusterID,
		req.ID,
	),
		urlParams,
		req,
		&res,
	)
}
