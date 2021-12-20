package ocm

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
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

	if c.opts.Recorder != nil {
		timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			us := v * 1000000 // make microseconds
			c.opts.Recorder.ObserveOCMAPIRequests(us)
		}))
		defer timer.ObserveDuration()
	}

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
