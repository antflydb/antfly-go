package admin

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// InternalClient provides access to internal admin APIs for cluster management
type InternalClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewInternalClient creates a new internal admin client
func NewInternalClient(baseURL string, httpClient *http.Client) *InternalClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: time.Second * 90,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     time.Minute,
				DisableKeepAlives:   false,
			},
		}
	}
	return &InternalClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// AddMetadataPeer adds a new peer to the metadata raft cluster
func (c *InternalClient) AddMetadataPeer(nodeID uint64, raftURL string) error {
	// Make POST request to internal endpoint
	url := fmt.Sprintf("%s/_internal/v1/peer/%d", c.baseURL, nodeID)
	resp, err := c.httpClient.Post(url, "application/octet-stream", strings.NewReader(raftURL))
	if err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add peer: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// RemoveMetadataPeer removes a peer from the metadata raft cluster
func (c *InternalClient) RemoveMetadataPeer(nodeID uint64) error {
	// Make DELETE request to internal endpoint
	url := fmt.Sprintf("%s/_internal/v1/peer/%d", c.baseURL, nodeID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove peer: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// MetadataStatus represents the status of the metadata raft cluster
type MetadataStatus struct {
	Leader  uint64
	Members map[uint64]string // nodeID -> raft URL
}

// GetMetadataStatus retrieves the current status of the metadata raft cluster
// TODO: This needs an actual endpoint on the server side to return raft status
func (c *InternalClient) GetMetadataStatus() (*MetadataStatus, error) {
	// This is a placeholder - we'll need to implement an endpoint on the server
	// that returns raft cluster status (leader, members, etc.)
	return nil, fmt.Errorf("not yet implemented - requires server-side endpoint")
}
