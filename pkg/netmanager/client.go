package netmanager

import (
	"context"
	"net/url"
	"path"

	"github.com/binkynet/BinkyNet/model"
	restkit "github.com/pulcy/rest-kit"
)

type Client struct {
	c *restkit.RestClient
}

// NewClient creates a new client with given endpoint
func NewClient(endpoint string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, maskAny(err)
	}
	c := restkit.NewRestClient(u)
	return &Client{
		c: c,
	}, nil
}

// GetWorkerConfig requests the local worker configuration for a worker with given id.
func (c *Client) GetWorkerConfig(ctx context.Context, workerID string) (model.LocalWorkerConfig, error) {
	var result model.LocalWorkerConfig
	if err := c.c.Request("GET", path.Join("worker", workerID, "config"), nil, nil, &result); err != nil {
		return model.LocalWorkerConfig{}, maskAny(err)
	}
	return result, nil
}
