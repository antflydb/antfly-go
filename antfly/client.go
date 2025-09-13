/*
Copyright 2025 The Antfly Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
//go:generate go tool oapi-codegen --config=cfg.yaml ../../openapi.yaml
package antfly

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
)

// Result types for SDK responses

// TableInfo represents information about a table
type TableInfo struct {
	Name          string         `json:"name"`
	Shards        map[string]any `json:"shards"`
	Indexes       map[string]any `json:"indexes"`
	Schema        *TableSchema   `json:"schema,omitempty"`
	StorageStatus *StorageStatus `json:"storage_status,omitempty"`
}

// IndexInfo represents information about an index
type IndexInfo struct {
	Name   string         `json:"name"`
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
	Status map[string]any `json:"status,omitempty"`
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Inserted int `json:"inserted"`
	Deleted  int `json:"deleted"`
	Failed   int `json:"failed"`
}

// BackupResult represents the result of a backup operation
type BackupResult struct {
	BackupID string `json:"backup_id"`
	Location string `json:"location"`
	Status   string `json:"status,omitempty"`
}

// RestoreResult represents the result of a restore operation
type RestoreResult struct {
	BackupID string `json:"backup_id"`
	Location string `json:"location"`
	Status   string `json:"status,omitempty"`
}

// AntflyClient is a client for interacting with the Antfly API
type AntflyClient struct {
	client     *Client
	httpClient *http.Client
	baseURL    string
}

// NewAntflyClient creates a new Antfly client
func NewAntflyClient(baseURL string, httpClient *http.Client) (*AntflyClient, error) {
	client, err := NewClient(baseURL, WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &AntflyClient{
		client:     client,
		httpClient: httpClient,
		baseURL:    baseURL,
	}, err
}

// sendRequest sends an HTTP request to the specified endpoint with the given content type.
func (c *AntflyClient) sendRequest(ctx context.Context, method, url, contentType string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading http response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return respBody, fmt.Errorf("received non-OK status %d from %s: %s", resp.StatusCode, url, string(respBody))
	}
	return respBody, nil
}

// CreateTable creates a new table
func (c *AntflyClient) CreateTable(ctx context.Context, tableName string, req CreateTableRequest) error {
	resp, err := c.client.CreateTable(ctx, tableName, req)
	if err != nil {
		return fmt.Errorf("creating table: %w", err)
	}
	if resp.StatusCode >= 300 {
		err := readErrorResponse(resp)
		if strings.Contains(err.Error(), "already exists") {
			return errors.New("table already exists")
		}
		return fmt.Errorf("creating table: %w", err)
	}
	return nil
}

func readErrorResponse(resp *http.Response) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading http response: %w", err)
	}
	return fmt.Errorf("received status %d: %s", resp.StatusCode, string(respBody))
}

// DropTable drops an existing table
func (c *AntflyClient) DropTable(ctx context.Context, tableName string) error {
	resp, err := c.client.DropTable(ctx, tableName)
	if err != nil {
		return fmt.Errorf("dropping table: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("dropping table: %w", readErrorResponse(resp))
	}

	return nil
}

func (c *AntflyClient) GetTable(ctx context.Context, tableName string) (*TableStatus, error) {
	resp, err := c.client.GetTable(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("getting table: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dropping table: %w", readErrorResponse(resp))
	}
	// Parse the response
	var table TableStatus
	if err := decoder.NewStreamDecoder(resp.Body).Decode(&table); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &table, nil
}

// ListTables lists all tables
func (c *AntflyClient) ListTables(ctx context.Context) ([]TableInfo, error) {
	resp, err := c.client.ListTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("listing tables: %w", readErrorResponse(resp))
	}

	// Parse the response
	var tables []TableInfo
	if err := decoder.NewStreamDecoder(resp.Body).Decode(&tables); err != nil {
		return nil, fmt.Errorf("parsing list tables response: %w", err)
	}

	return tables, nil
}

// CreateIndex creates a new index on a table
func (c *AntflyClient) CreateIndex(ctx context.Context, tableName, indexName string, config IndexConfig) error {
	resp, err := c.client.CreateIndex(ctx, tableName, indexName, config)
	if err != nil {
		return fmt.Errorf("creating index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("creating index: %w", readErrorResponse(resp))
	}
	return nil
}

// DropIndex drops an index from a table
func (c *AntflyClient) DropIndex(ctx context.Context, tableName, indexName string) error {
	resp, err := c.client.DropIndex(ctx, tableName, indexName)
	if err != nil {
		return fmt.Errorf("dropping index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("dropping index: %w", readErrorResponse(resp))
	}
	return nil
}

// ListIndexes lists all indexes for a table
func (c *AntflyClient) ListIndexes(ctx context.Context, tableName string) ([]IndexInfo, error) {
	resp, err := c.client.ListIndexes(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("listing indexes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("listing indexes: %w", readErrorResponse(resp))
	}
	// Parse the response
	var indexes []IndexInfo
	if err := decoder.NewStreamDecoder(resp.Body).Decode(&indexes); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return indexes, nil
}

// GetIndex gets a specific index for a table
func (c *AntflyClient) GetIndex(ctx context.Context, tableName, indexName string) (*IndexInfo, error) {
	resp, err := c.client.GetIndex(ctx, tableName, indexName)
	if err != nil {
		return nil, fmt.Errorf("getting index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("getting index: %w", readErrorResponse(resp))
	}
	// Parse the response
	var index IndexInfo
	if err := decoder.NewStreamDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &index, nil
}

// Backup backs up a table
func (c *AntflyClient) Backup(ctx context.Context, tableName, backupID, location string) (*BackupResult, error) {
	if tableName == "" {
		return nil, fmt.Errorf("empty table name")
	}
	backupURL, _ := url.JoinPath(c.baseURL, "table", tableName, "backup")
	requestBody := map[string]string{
		"backup_id": backupID,
		"location":  location,
	}
	bodyBytes, err := sonic.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling backup request: %w", err)
	}

	respBody, err := c.sendRequest(ctx, http.MethodPost, backupURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		// API might return 201 Created or similar success codes as well.
		if !strings.Contains(err.Error(), "received non-OK status 201") && !strings.Contains(err.Error(), "received non-OK status 202") {
			return nil, fmt.Errorf("backup request failed: %w", err)
		}
	}

	result := &BackupResult{
		BackupID: backupID,
		Location: location,
		Status:   "initiated",
	}

	// Try to parse response if available
	if len(respBody) > 0 {
		_ = sonic.Unmarshal(respBody, result)
	}

	return result, nil
}

// Restore restores a table from a backup
func (c *AntflyClient) Restore(ctx context.Context, tableName, backupID, location string) (*RestoreResult, error) {
	if tableName == "" {
		return nil, fmt.Errorf("empty table name")
	}
	restoreURL, _ := url.JoinPath(c.baseURL, "table", tableName, "restore")
	requestBody := map[string]string{
		"backup_id": backupID,
		"location":  location,
	}
	bodyBytes, err := sonic.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling restore request: %w", err)
	}

	respBody, err := c.sendRequest(ctx, http.MethodPost, restoreURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		// Restore API might return 202 Accepted.
		if !strings.Contains(err.Error(), "received non-OK status 202") {
			return nil, fmt.Errorf("restoring failed: %w", err)
		}
	}

	result := &RestoreResult{
		BackupID: backupID,
		Location: location,
		Status:   "initiated",
	}

	// Try to parse response if available
	if len(respBody) > 0 {
		_ = sonic.Unmarshal(respBody, result)
	}

	return result, nil
}

// Query executes queries against a table
func (c *AntflyClient) Query(ctx context.Context, opts ...QueryRequest) (*QueryResponses, error) {
	queryURL, _ := url.JoinPath(c.baseURL, "query")

	request := bytes.NewBuffer(nil)
	e := encoder.NewStreamEncoder(request)
	for _, opt := range opts {
		// Validate options
		if len(opt.Indexes) > 0 && opt.SemanticSearch == "" {
			return nil, errors.New("semantic_search required when indexes are specified")
		}
		if len(opt.Indexes) > 0 && opt.Offset > 0 {
			return nil, errors.New("offset not available when indexes are specified")
		}

		if err := e.Encode(opt); err != nil {
			return nil, fmt.Errorf("marshalling query: %w", err)
		}
	}

	// Log the the request we're running as a cURL command
	// log.Printf("curl -XPOST %s -H \"Content-type: application/json\" -d '%s'", queryURL, request)

	respBody, err := c.sendRequest(ctx, http.MethodPost, queryURL, "application/json", request)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	var result QueryResponses
	if err := sonic.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing result: %w", err)
	}

	return &result, nil
}

// Batch performs a batch operation on a table
func (c *AntflyClient) Batch(ctx context.Context, tableName string, request BatchRequest) (*BatchResult, error) {
	tableSpecificURL, err := url.JoinPath(c.baseURL, "table", tableName)
	if err != nil {
		return nil, fmt.Errorf("creating table specific URL: %w", err)
	}
	batchURL, err := url.JoinPath(tableSpecificURL, "batch")
	if err != nil {
		return nil, fmt.Errorf("creating batch URL: %w", err)
	}

	batchBody, err := sonic.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshalling batch request: %w", err)
	}

	respBody, err := c.sendRequest(ctx, http.MethodPost, batchURL, "application/json", bytes.NewBuffer(batchBody))
	if err != nil {
		return nil, fmt.Errorf("batch operation failed: %w", err)
	}

	var result BatchResult
	if len(respBody) > 0 {
		if err := sonic.Unmarshal(respBody, &result); err != nil {
			// If unmarshaling fails, return a basic result
			result = BatchResult{
				Inserted: len(request.Inserts),
				Deleted:  len(request.Deletes),
			}
		}
	} else {
		// No response body, return counts from request
		result = BatchResult{
			Inserted: len(request.Inserts),
			Deleted:  len(request.Deletes),
		}
	}

	return &result, nil
}
