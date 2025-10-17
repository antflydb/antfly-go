/*
Copyright 2025 The Antfly Contributors

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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/antflydb/antfly-go/antfly/oapi"
	"github.com/antflydb/antfly-go/antfly/query"
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
)

// Re-export commonly used types from oapi package
type (
	// Table and Index types
	CreateTableRequest = oapi.CreateTableRequest
	TableStatus        = oapi.TableStatus
	TableSchema        = oapi.TableSchema
	IndexConfig        = oapi.IndexConfig
	IndexStatus        = oapi.IndexStatus
	IndexType          = oapi.IndexType

	// Index config types
	EmbeddingIndexConfig = oapi.EmbeddingIndexConfig
	BleveIndexV2Config   = oapi.BleveIndexV2Config
	BleveIndexV2Stats    = oapi.BleveIndexV2Stats

	// Model config types
	ModelConfig    = oapi.ModelConfig
	Provider       = oapi.Provider
	OllamaConfig   = oapi.OllamaConfig
	OpenAIConfig   = oapi.OpenAIConfig
	GoogleConfig   = oapi.GoogleConfig
	BedrockConfig  = oapi.BedrockConfig
	RerankerConfig = oapi.RerankerConfig

	// Batch types
	BatchRequest = oapi.BatchRequest

	// Query response types
	QueryResponses = oapi.QueryResponses
	QueryResult    = oapi.QueryResult
	Hits           = oapi.QueryHits
	Hit            = oapi.QueryHit
	FacetOption    = oapi.FacetOption
	FacetResult    = oapi.FacetResult

	// Other types
	AntflyType     = oapi.AntflyType
	MergeStrategy  = oapi.MergeStrategy
	DocumentSchema = oapi.DocumentSchema
)

// Constants from oapi
const (
	// IndexType values
	BleveV2  = oapi.BleveV2
	VectorV2 = oapi.VectorV2

	// Provider values
	Ollama  = oapi.Ollama
	Openai  = oapi.Openai
	Gemini  = oapi.Gemini
	Bedrock = oapi.Bedrock
	Mock    = oapi.Mock

	// MergeStrategy values
	Rrf      = oapi.Rrf
	Failover = oapi.Failover
)

// BatchResult represents the result of a batch operation with detailed failure information
type BatchResult struct {
	// Deleted Number of documents successfully deleted
	Deleted int `json:"deleted,omitempty"`

	// Inserted Number of documents successfully inserted
	Inserted int `json:"inserted,omitempty"`

	// Failed List of failed operations with error details
	Failed []struct {
		// Error message for this failure
		Error string `json:"error,omitempty"`

		// Id The document ID that failed
		Id string `json:"id,omitempty"`
	} `json:"failed,omitempty"`
}

// QueryRequest represents a query request with strongly-typed query fields.
// This is the SDK-friendly version of oapi.QueryRequest with Query types instead of json.RawMessage.
type QueryRequest struct {
	// Table name to query
	Table string `json:"table,omitempty"`

	// Analyses specifies analysis operations to perform
	Analyses *oapi.Analyses `json:"analyses,omitempty"`

	// Count whether to return only the count of matching documents
	Count bool `json:"count,omitempty"`

	// DistanceOver minimum distance for semantic similarity search
	DistanceOver *float32 `json:"distance_over,omitempty"`

	// DistanceUnder maximum distance for semantic similarity search
	DistanceUnder *float32 `json:"distance_under,omitempty"`

	// Embeddings raw embeddings to use for semantic searches (the keys are the indexes to use for the queries)
	Embeddings map[string][]float32 `json:"embeddings,omitempty"`

	// ExclusionQuery strongly-typed Bleve search query for exclusions
	ExclusionQuery *query.Query `json:"-"`

	// Facets to compute
	Facets map[string]FacetOption `json:"facets,omitempty"`

	// Fields list of fields to include in the results
	Fields []string `json:"fields,omitempty"`

	// FilterPrefix for filtering by key prefix
	FilterPrefix []byte `json:"filter_prefix,omitempty"`

	// FilterQuery strongly-typed Bleve search query for filtering
	FilterQuery *query.Query `json:"-"`

	// FullTextSearch strongly-typed Bleve search query for full-text search
	FullTextSearch *query.Query `json:"-"`

	// Indexes to search (required for semantic search)
	Indexes []string `json:"indexes,omitempty"`

	// Limit maximum number of results to return or topk for semantic_search
	Limit int `json:"limit,omitempty"`

	// MergeStrategy for combining results from semantic_search and full_text_search
	// rrf: Reciprocal Rank Fusion
	// failover: Use full_text_search if embedding generation fails
	MergeStrategy MergeStrategy `json:"merge_strategy,omitempty"`

	// Offset number of results to skip for pagination (only available for full_text_search queries)
	Offset int `json:"offset,omitempty"`

	// OrderBy specifies fields to order by: field name -> descending (true) or ascending (false)
	OrderBy map[string]bool `json:"order_by,omitempty"`

	// Reranker configuration for reranking results
	Reranker *RerankerConfig `json:"reranker,omitempty"`

	// SemanticSearch text to use for semantic similarity search
	SemanticSearch string `json:"semantic_search,omitempty"`
}

// AntflyClient is a client for interacting with the Antfly API
type AntflyClient struct {
	client     *oapi.Client
	httpClient *http.Client
	baseURL    string
}

// NewAntflyClient creates a new Antfly client
func NewAntflyClient(baseURL string, httpClient *http.Client) (*AntflyClient, error) {
	client, err := oapi.NewClient(baseURL, oapi.WithHTTPClient(httpClient))
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
	defer resp.Body.Close()
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
func (c *AntflyClient) ListTables(ctx context.Context) ([]TableStatus, error) {
	resp, err := c.client.ListTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("listing tables: %w", readErrorResponse(resp))
	}

	// Parse the response
	var tables []TableStatus
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
func (c *AntflyClient) ListIndexes(ctx context.Context, tableName string) (map[string]IndexStatus, error) {
	resp, err := c.client.ListIndexes(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("listing indexes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("listing indexes: %w", readErrorResponse(resp))
	}
	// Parse the response
	var indexes map[string]IndexStatus
	if err := decoder.NewStreamDecoder(resp.Body).Decode(&indexes); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return indexes, nil
}

// GetIndex gets a specific index for a table
func (c *AntflyClient) GetIndex(ctx context.Context, tableName, indexName string) (*IndexStatus, error) {
	resp, err := c.client.GetIndex(ctx, tableName, indexName)
	if err != nil {
		return nil, fmt.Errorf("getting index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("getting index: %w", readErrorResponse(resp))
	}
	// Parse the response
	var index IndexStatus
	if err := decoder.NewStreamDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &index, nil
}

// Backup backs up a table
func (c *AntflyClient) Backup(ctx context.Context, tableName, backupID, location string) error {
	if tableName == "" {
		return fmt.Errorf("empty table name")
	}
	backupURL, _ := url.JoinPath(c.baseURL, "table", tableName, "backup")
	requestBody := map[string]string{
		"backup_id": backupID,
		"location":  location,
	}
	bodyBytes, err := sonic.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshalling backup request: %w", err)
	}

	_, err = c.sendRequest(ctx, http.MethodPost, backupURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		// API might return 201 Created or similar success codes as well.
		if !strings.Contains(err.Error(), "received non-OK status 201") && !strings.Contains(err.Error(), "received non-OK status 202") {
			return fmt.Errorf("backup request failed: %w", err)
		}
	}

	return nil
}

// Restore restores a table from a backup
func (c *AntflyClient) Restore(ctx context.Context, tableName, backupID, location string) error {
	if tableName == "" {
		return fmt.Errorf("empty table name")
	}
	restoreURL, _ := url.JoinPath(c.baseURL, "table", tableName, "restore")
	requestBody := map[string]string{
		"backup_id": backupID,
		"location":  location,
	}
	bodyBytes, err := sonic.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshalling restore request: %w", err)
	}

	_, err = c.sendRequest(ctx, http.MethodPost, restoreURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		// Restore API might return 202 Accepted.
		if !strings.Contains(err.Error(), "received non-OK status 202") {
			return fmt.Errorf("restoring failed: %w", err)
		}
	}

	return nil
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

		// Convert SDK QueryRequest to oapi.QueryRequest
		oapiReq := oapi.QueryRequest{
			Table:          opt.Table,
			Analyses:       opt.Analyses,
			Count:          opt.Count,
			DistanceOver:   opt.DistanceOver,
			DistanceUnder:  opt.DistanceUnder,
			Embeddings:     opt.Embeddings,
			Facets:         opt.Facets,
			Fields:         opt.Fields,
			FilterPrefix:   opt.FilterPrefix,
			Indexes:        opt.Indexes,
			Limit:          opt.Limit,
			MergeStrategy:  opt.MergeStrategy,
			Offset:         opt.Offset,
			OrderBy:        opt.OrderBy,
			Reranker:       opt.Reranker,
			SemanticSearch: opt.SemanticSearch,
		}

		// Marshal query fields to json.RawMessage
		if opt.FilterQuery != nil {
			filterQueryJSON, err := json.Marshal(opt.FilterQuery)
			if err != nil {
				return nil, fmt.Errorf("marshalling filter_query: %w", err)
			}
			oapiReq.FilterQuery = filterQueryJSON
		}
		if opt.FullTextSearch != nil {
			fullTextSearchJSON, err := json.Marshal(opt.FullTextSearch)
			if err != nil {
				return nil, fmt.Errorf("marshalling full_text_search: %w", err)
			}
			oapiReq.FullTextSearch = fullTextSearchJSON
		}
		if opt.ExclusionQuery != nil {
			exclusionQueryJSON, err := json.Marshal(opt.ExclusionQuery)
			if err != nil {
				return nil, fmt.Errorf("marshalling exclusion_query: %w", err)
			}
			oapiReq.ExclusionQuery = exclusionQueryJSON
		}

		if err := e.Encode(oapiReq); err != nil {
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

// LookupKey looks up a document by its key
func (c *AntflyClient) LookupKey(ctx context.Context, tableName, key string) (map[string]any, error) {
	resp, err := c.client.LookupKey(ctx, tableName, key)
	if err != nil {
		return nil, fmt.Errorf("looking up key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("looking up key: %w", readErrorResponse(resp))
	}

	// Parse the response
	var document map[string]any
	if err := decoder.NewStreamDecoder(resp.Body).Decode(&document); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return document, nil
}

func NewModelConfig(config any) (*ModelConfig, error) {
	var provider Provider
	modelConfig := &ModelConfig{}
	switch v := config.(type) {
	case OllamaConfig:
		provider = Ollama
		modelConfig.FromOllamaConfig(v)
	case OpenAIConfig:
		provider = Openai
		modelConfig.FromOpenAIConfig(v)
	case GoogleConfig:
		provider = Gemini
		modelConfig.FromGoogleConfig(v)
	case BedrockConfig:
		provider = Bedrock
		modelConfig.FromBedrockConfig(v)
	default:
		return nil, fmt.Errorf("unknown model config type: %T", v)
	}

	modelConfig.Provider = provider
	return modelConfig, nil
}

func NewIndexConfig(name string, config any) (*IndexConfig, error) {
	var t IndexType
	idxConfig := &IndexConfig{
		Name: name,
	}
	switch v := config.(type) {
	case EmbeddingIndexConfig:
		t = VectorV2
		idxConfig.FromEmbeddingIndexConfig(v)
	case BleveIndexV2Config:
		t = BleveV2
		idxConfig.FromBleveIndexV2Config(v)
	default:
		return nil, fmt.Errorf("unsupported index config type: %T", config)
	}
	idxConfig.Type = t

	return idxConfig, nil

}
