package ocm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type ClusterGetRequest struct{}

type ClusterGetResponse struct {
	Items []Cluster `json:"items"`
}

type Cluster struct {
	Id         string `json:"id"`
	ExternalId string `json:"external_id"`
}

func (c *Client) GetCluster(
	ctx context.Context,
	req ClusterGetRequest,
) (res ClusterGetResponse, err error) {
	urlParams := url.Values{}
	urlParams.Add("search",
		fmt.Sprintf("external_id = '%s'", c.opts.ClusterExternalID))

	return res, c.do(ctx, http.MethodGet, fmt.Sprintf(
		"/api/clusters_mgmt/v1/clusters",
	),
		urlParams,
		req,
		&res,
	)
}
