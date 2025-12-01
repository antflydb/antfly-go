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

	EmbedderProvider         = oapi.EmbedderProvider
	GeneratorProvider        = oapi.GeneratorProvider
	EmbedderConfig           = oapi.EmbedderConfig
	GeneratorConfig          = oapi.GeneratorConfig
	OllamaEmbedderConfig     = oapi.OllamaEmbedderConfig
	OpenAIEmbedderConfig     = oapi.OpenAIEmbedderConfig
	GoogleEmbedderConfig     = oapi.GoogleEmbedderConfig
	BedrockEmbedderConfig    = oapi.BedrockEmbedderConfig
	VertexEmbedderConfig     = oapi.VertexEmbedderConfig
	OllamaGeneratorConfig    = oapi.OllamaGeneratorConfig
	OpenAIGeneratorConfig    = oapi.OpenAIGeneratorConfig
	GoogleGeneratorConfig    = oapi.GoogleGeneratorConfig
	BedrockGeneratorConfig   = oapi.BedrockGeneratorConfig
	VertexGeneratorConfig    = oapi.VertexGeneratorConfig
	AnthropicGeneratorConfig = oapi.AnthropicGeneratorConfig
	RerankerConfig           = oapi.RerankerConfig
	OllamaRerankerConfig     = oapi.OllamaRerankerConfig
	TermiteRerankerConfig    = oapi.TermiteRerankerConfig
	RerankerProvider         = oapi.RerankerProvider
	Pruner                   = oapi.Pruner

	// Chunker config types
	ChunkerProvider      = oapi.ChunkerProvider
	ChunkerConfig        = oapi.ChunkerConfig
	ChunkingStrategy     = oapi.ChunkingStrategy
	TermiteChunkerConfig = oapi.TermiteChunkerConfig
	AntflyChunkerConfig  = oapi.AntflyChunkerConfig

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

	// Validation types
	ValidationError  = oapi.ValidationError
	ValidationResult = oapi.ValidationResult

	// LinearMerge types
	LinearMergePageStatus = oapi.LinearMergePageStatus
	LinearMergeRequest    = oapi.LinearMergeRequest
	LinearMergeResult     = oapi.LinearMergeResult
	FailedOperation       = oapi.FailedOperation
	KeyRange              = oapi.KeyRange
	SyncLevel             = oapi.SyncLevel

	// AI Agent types
	AnswerAgentResult                  = oapi.AnswerAgentResult
	ClassificationTransformationResult = oapi.ClassificationTransformationResult
	RouteType                          = oapi.RouteType
	QueryStrategy                      = oapi.QueryStrategy
	SemanticQueryMode                  = oapi.SemanticQueryMode
	AnswerAgentSteps                   = oapi.AnswerAgentSteps
	ClassificationStepConfig           = oapi.ClassificationStepConfig
	AnswerStepConfig                   = oapi.AnswerStepConfig
	FollowupStepConfig                 = oapi.FollowupStepConfig
	ConfidenceStepConfig               = oapi.ConfidenceStepConfig
	RetryConfig                        = oapi.RetryConfig
	ChainLink                          = oapi.ChainLink
	ChainCondition                     = oapi.ChainCondition
)

// Re-export chunking strategy constants
const (
	ChunkingStrategyFixed      = oapi.ChunkingStrategyFixed
	ChunkingStrategyChonkyOnnx = oapi.ChunkingStrategyChonkyOnnx
)

// BatchRequest represents a batch operation request with flexible insert types.
// Unlike the oapi.BatchRequest, this version allows Inserts to accept any type
// (including structs) which will be automatically marshaled.
type BatchRequest struct {
	// Deletes List of keys to delete.
	Deletes []string `json:"deletes,omitempty"`

	// Inserts Map of key to document. Documents can be any type (map, struct, etc.)
	// and will be automatically marshaled to JSON.
	Inserts map[string]any `json:"inserts,omitempty"`

	// SyncLevel Synchronization level for the batch operation:
	// - "propose": Wait for Raft proposal acceptance (fastest, default)
	// - "write": Wait for Pebble KV write
	// - "full_text": Wait for full-text index WAL write (slowest, most durable)
	// - "aknn": Wait for vector index write with best-effort synchronous embedding (falls back to async on timeout)
	SyncLevel SyncLevel `json:"sync_level,omitempty"`
}

// Constants from oapi
const (
	// IndexType values
	IndexTypeFullTextV0 = oapi.IndexTypeFullTextV0
	IndexTypeAknnV0     = oapi.IndexTypeAknnV0

	// Provider values
	EmbedderProviderOllama     = oapi.EmbedderProviderOllama
	EmbedderProviderOpenai     = oapi.EmbedderProviderOpenai
	EmbedderProviderGemini     = oapi.EmbedderProviderGemini
	EmbedderProviderBedrock    = oapi.EmbedderProviderBedrock
	EmbedderProviderVertex     = oapi.EmbedderProviderVertex
	EmbedderProviderMock       = oapi.EmbedderProviderMock
	GeneratorProviderOllama    = oapi.GeneratorProviderOllama
	GeneratorProviderOpenai    = oapi.GeneratorProviderOpenai
	GeneratorProviderGemini    = oapi.GeneratorProviderGemini
	GeneratorProviderBedrock   = oapi.GeneratorProviderBedrock
	GeneratorProviderVertex    = oapi.GeneratorProviderVertex
	GeneratorProviderAnthropic = oapi.GeneratorProviderAnthropic
	GeneratorProviderMock      = oapi.GeneratorProviderMock
	RerankerProviderOllama     = oapi.RerankerProviderOllama
	RerankerProviderTermite    = oapi.RerankerProviderTermite

	// MergeStrategy values
	MergeStrategyRrf      = oapi.MergeStrategyRrf
	MergeStrategyFailover = oapi.MergeStrategyFailover

	// LinearMergePageStatus values
	LinearMergePageStatusSuccess = oapi.LinearMergePageStatusSuccess
	LinearMergePageStatusPartial = oapi.LinearMergePageStatusPartial
	LinearMergePageStatusError   = oapi.LinearMergePageStatusError

	// SyncLevel values
	SyncLevelPropose  = oapi.SyncLevelPropose
	SyncLevelWrite    = oapi.SyncLevelWrite
	SyncLevelFullText = oapi.SyncLevelFullText
	SyncLevelAknn     = oapi.SyncLevelAknn

	// RouteType values
	RouteTypeQuestion = oapi.RouteTypeQuestion
	RouteTypeSearch   = oapi.RouteTypeSearch

	// QueryStrategy values
	QueryStrategySimple    = oapi.QueryStrategySimple
	QueryStrategyDecompose = oapi.QueryStrategyDecompose
	QueryStrategyStepBack  = oapi.QueryStrategyStepBack
	QueryStrategyHyde      = oapi.QueryStrategyHyde

	// SemanticQueryMode values
	SemanticQueryModeRewrite      = oapi.SemanticQueryModeRewrite
	SemanticQueryModeHypothetical = oapi.SemanticQueryModeHypothetical

	// ChainCondition values
	ChainConditionAlways     = oapi.ChainConditionAlways
	ChainConditionOnError    = oapi.ChainConditionOnError
	ChainConditionOnTimeout  = oapi.ChainConditionOnTimeout
	ChainConditionOnRateLimit = oapi.ChainConditionOnRateLimit
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

	// Pruner configuration for pruning search results based on score quality
	Pruner Pruner `json:"pruner,omitempty,omitzero"`

	// SemanticSearch text to use for semantic similarity search
	SemanticSearch string `json:"semantic_search,omitempty"`

	// DocumentRenderer optional Go template string for rendering document content to the prompt
	DocumentRenderer string `json:"document_renderer,omitempty"`
}

// MarshalJSON implements custom JSON marshalling for QueryRequest.
// It converts the strongly-typed *query.Query fields to json.RawMessage
// for compatibility with the OAPI layer.
func (q QueryRequest) MarshalJSON() ([]byte, error) {
	// Convert SDK QueryRequest to oapi.QueryRequest
	oapiReq := oapi.QueryRequest{
		Table:            q.Table,
		Analyses:         q.Analyses,
		Count:            q.Count,
		DistanceOver:     q.DistanceOver,
		DistanceUnder:    q.DistanceUnder,
		Embeddings:       q.Embeddings,
		Facets:           q.Facets,
		Fields:           q.Fields,
		FilterPrefix:     q.FilterPrefix,
		Indexes:          q.Indexes,
		Limit:            q.Limit,
		MergeStrategy:    q.MergeStrategy,
		Offset:           q.Offset,
		OrderBy:          q.OrderBy,
		Reranker:         q.Reranker,
		Pruner:           q.Pruner,
		SemanticSearch:   q.SemanticSearch,
		DocumentRenderer: q.DocumentRenderer,
	}

	// Marshal query fields to json.RawMessage
	var err error
	if q.FilterQuery != nil {
		oapiReq.FilterQuery, err = json.Marshal(q.FilterQuery)
		if err != nil {
			return nil, fmt.Errorf("marshalling filter_query: %w", err)
		}
	}
	if q.FullTextSearch != nil {
		oapiReq.FullTextSearch, err = json.Marshal(q.FullTextSearch)
		if err != nil {
			return nil, fmt.Errorf("marshalling full_text_search: %w", err)
		}
	}
	if q.ExclusionQuery != nil {
		oapiReq.ExclusionQuery, err = json.Marshal(q.ExclusionQuery)
		if err != nil {
			return nil, fmt.Errorf("marshalling exclusion_query: %w", err)
		}
	}

	return json.Marshal(oapiReq)
}

// UnmarshalJSON implements custom JSON unmarshalling for QueryRequest.
// It converts json.RawMessage fields back to strongly-typed *query.Query.
func (q *QueryRequest) UnmarshalJSON(data []byte) error {
	// Unmarshal into oapi.QueryRequest
	var oapiReq oapi.QueryRequest
	if err := json.Unmarshal(data, &oapiReq); err != nil {
		return err
	}

	// Copy simple fields
	q.Table = oapiReq.Table
	q.Analyses = oapiReq.Analyses
	q.Count = oapiReq.Count
	q.DistanceOver = oapiReq.DistanceOver
	q.DistanceUnder = oapiReq.DistanceUnder
	q.Embeddings = oapiReq.Embeddings
	q.Facets = oapiReq.Facets
	q.Fields = oapiReq.Fields
	q.FilterPrefix = oapiReq.FilterPrefix
	q.Indexes = oapiReq.Indexes
	q.Limit = oapiReq.Limit
	q.MergeStrategy = oapiReq.MergeStrategy
	q.Offset = oapiReq.Offset
	q.OrderBy = oapiReq.OrderBy
	q.Reranker = oapiReq.Reranker
	q.Pruner = oapiReq.Pruner
	q.SemanticSearch = oapiReq.SemanticSearch
	q.DocumentRenderer = oapiReq.DocumentRenderer

	// Unmarshal query fields (only if not null and not empty)
	if len(oapiReq.FilterQuery) > 0 && !bytes.Equal(oapiReq.FilterQuery, []byte("null")) {
		q.FilterQuery = new(query.Query)
		if err := json.Unmarshal(oapiReq.FilterQuery, q.FilterQuery); err != nil {
			return fmt.Errorf("unmarshalling filter_query: %w", err)
		}
	}
	if len(oapiReq.FullTextSearch) > 0 && !bytes.Equal(oapiReq.FullTextSearch, []byte("null")) {
		q.FullTextSearch = new(query.Query)
		if err := json.Unmarshal(oapiReq.FullTextSearch, q.FullTextSearch); err != nil {
			return fmt.Errorf("unmarshalling full_text_search: %w", err)
		}
	}
	if len(oapiReq.ExclusionQuery) > 0 && !bytes.Equal(oapiReq.ExclusionQuery, []byte("null")) {
		q.ExclusionQuery = new(query.Query)
		if err := json.Unmarshal(oapiReq.ExclusionQuery, q.ExclusionQuery); err != nil {
			return fmt.Errorf("unmarshalling exclusion_query: %w", err)
		}
	}

	return nil
}

// RAGRequest represents a RAG request with strongly-typed query fields.
// This is the SDK-friendly version of oapi.RAGRequest with QueryRequest types instead of oapi.QueryRequest.
type RAGRequest struct {
	// Queries to execute for retrieval
	Queries []QueryRequest `json:"queries"`

	// Summarizer model configuration for generation
	Summarizer GeneratorConfig `json:"summarizer"`

	// SystemPrompt optional system prompt to guide the summarization
	SystemPrompt string `json:"system_prompt,omitempty"`

	// WithStreaming Enable SSE streaming of results instead of JSON response
	WithStreaming bool `json:"with_streaming,omitempty,omitzero"`
}

// AnswerAgentRequest represents an answer agent request.
// The answer agent classifies queries, transforms them for optimal semantic search, executes provided queries, and generates answers.
type AnswerAgentRequest struct {
	// Query is the user's natural language query (required)
	Query string `json:"query"`

	// Generator is the default model configuration for all pipeline steps (required)
	// This is the simple configuration - just set this and everything works with sensible defaults
	Generator GeneratorConfig `json:"generator"`

	// Queries is the array of query requests to execute with the transformed query (required)
	// The transformed semantic search query will be applied to each QueryRequest
	Queries []QueryRequest `json:"queries"`

	// Steps is optional advanced per-step configuration
	// Override the default generator for specific steps, configure step-specific options,
	// or set up generator chains with retry/fallback
	Steps *AnswerAgentSteps `json:"steps,omitempty"`

	// WithStreaming enables SSE streaming of results instead of JSON response (default: true)
	WithStreaming bool `json:"with_streaming,omitempty"`

	// MaxContextTokens is the maximum total tokens allowed for retrieved document context
	// When set, documents are pruned (lowest-ranked first) to fit within this budget
	MaxContextTokens int `json:"max_context_tokens,omitempty"`

	// ReserveTokens is the number of tokens to reserve for system prompt, answer generation, and overhead
	// Defaults to 4000 if MaxContextTokens is set
	ReserveTokens int `json:"reserve_tokens,omitempty"`
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
	resp, err := c.client.ListTables(ctx, &oapi.ListTablesParams{})
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
	// Parse the response - API returns an array, we convert to a map keyed by index name
	var indexList []IndexStatus
	if err := decoder.NewStreamDecoder(resp.Body).Decode(&indexList); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	// Convert array to map keyed by index name
	indexes := make(map[string]IndexStatus, len(indexList))
	for _, idx := range indexList {
		indexes[idx.Config.Name] = idx
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

	req := oapi.BackupRequest{
		BackupId: backupID,
		Location: location,
	}

	resp, err := c.client.BackupTable(ctx, tableName, req)
	if err != nil {
		return fmt.Errorf("backup request failed: %w", err)
	}
	defer resp.Body.Close()

	// API might return 201 Created or 202 Accepted
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("backup failed: %w", readErrorResponse(resp))
	}

	return nil
}

// Restore restores a table from a backup
func (c *AntflyClient) Restore(ctx context.Context, tableName, backupID, location string) error {
	if tableName == "" {
		return fmt.Errorf("empty table name")
	}

	req := oapi.RestoreRequest{
		BackupId: backupID,
		Location: location,
	}

	resp, err := c.client.RestoreTable(ctx, tableName, req)
	if err != nil {
		return fmt.Errorf("restore request failed: %w", err)
	}
	defer resp.Body.Close()

	// Restore API might return 202 Accepted
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("restore failed: %w", readErrorResponse(resp))
	}

	return nil
}

// Query executes queries against a table
func (c *AntflyClient) Query(ctx context.Context, opts ...QueryRequest) (*QueryResponses, error) {
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

		// MarshalJSON now handles the conversion to oapi.QueryRequest automatically
		if err := e.Encode(opt); err != nil {
			return nil, fmt.Errorf("marshalling query: %w", err)
		}
	}

	resp, err := c.client.GlobalQueryWithBody(ctx, "application/json", request)
	if err != nil {
		return nil, fmt.Errorf("sending query request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("query failed: %w", readErrorResponse(resp))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var result QueryResponses
	if err := sonic.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing result: %w", err)
	}

	return &result, nil
}

// Batch performs a batch operation on a table
func (c *AntflyClient) Batch(ctx context.Context, tableName string, request BatchRequest) (*BatchResult, error) {
	batchBody, err := sonic.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshalling batch request: %w", err)
	}

	resp, err := c.client.BatchWithBody(ctx, tableName, "application/json", bytes.NewBuffer(batchBody))
	if err != nil {
		return nil, fmt.Errorf("batch operation failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("batch failed: %w", readErrorResponse(resp))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
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

// LinearMerge performs a stateless linear merge of sorted records from an external source.
// Records are upserted, and any Antfly records in the key range that are absent from the
// input are deleted. Supports progressive pagination for large datasets.
//
// WARNING: Not safe for concurrent merge operations with overlapping ranges.
// Designed as a sync/import API for single-client use.
func (c *AntflyClient) LinearMerge(ctx context.Context, tableName string, request LinearMergeRequest) (*LinearMergeResult, error) {
	resp, err := c.client.LinearMerge(ctx, tableName, request)
	if err != nil {
		return nil, fmt.Errorf("linear merge operation failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("linear merge failed: %w", readErrorResponse(resp))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var result LinearMergeResult
	if len(respBody) > 0 {
		if err := sonic.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parsing linear merge result: %w", err)
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

// RAGOptions contains optional parameters for RAG requests
type RAGOptions struct {
	// Callback is called for each chunk of the streaming response
	// If not provided, chunks are written to a default buffer
	Callback func(chunk string) error
}

// AnswerAgentOptions contains optional parameters for answer agent requests
type AnswerAgentOptions struct {
	// OnClassification is called when the classification and transformation result is received
	// Receives the full ClassificationTransformationResult with route_type, strategy, semantic_query, etc.
	OnClassification func(result *ClassificationTransformationResult) error

	// OnReasoning is called for each chunk of reasoning during classification (if steps.classification.with_reasoning is enabled)
	OnReasoning func(chunk string) error

	// OnHit is called for each search result hit
	OnHit func(hit string) error

	// OnAnswer is called for each chunk of the answer text
	OnAnswer func(chunk string) error

	// OnFollowupQuestion is called for each follow-up question (if steps.followup.enabled is true)
	OnFollowupQuestion func(question string) error
}

// RAG performs a RAG (Retrieval-Augmented Generation) query and streams the response
// Accepts a RAGRequest with one or more QueryRequests for single-table or multi-table RAG queries
// The callback function is called for each chunk of the streaming response
func (c *AntflyClient) RAG(ctx context.Context, ragReq RAGRequest, opts ...RAGOptions) (string, error) {
	ragURL, _ := url.JoinPath(c.baseURL, "rag")

	// Merge options
	var opt RAGOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Marshal RAGRequest - QueryRequest.MarshalJSON handles the conversion automatically
	ragBody, err := sonic.Marshal(ragReq)
	if err != nil {
		return "", fmt.Errorf("marshalling RAG request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ragURL, bytes.NewBuffer(ragBody))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// When streaming is enabled, expect SSE response instead of JSON
	if ragReq.WithStreaming {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending RAG request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("RAG request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// If streaming is disabled, read JSON response directly
	if !ragReq.WithStreaming {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("reading response body: %w", err)
		}
		return string(respBody), nil
	}

	// Use callback if provided, otherwise accumulate in a buffer
	var result strings.Builder
	callback := opt.Callback
	if callback == nil {
		callback = func(chunk string) error {
			result.WriteString(chunk)
			return nil
		}
	}

	// Read the SSE stream
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			// Parse SSE format: "data: <content>\n\n"
			lines := strings.SplitSeq(chunk, "\n")
			for line := range lines {
				if after, ok := strings.CutPrefix(line, "data: "); ok {
					data := after
					if err := callback(data); err != nil {
						return "", fmt.Errorf("callback error: %w", err)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading stream: %w", err)
		}
	}

	return result.String(), nil
}

// AnswerAgent performs an answer agent query with classification, query generation, and answer generation.
// The agent classifies the query, generates appropriate searches, executes them, and generates answers.
// Supports streaming responses with granular callbacks for different event types.
func (c *AntflyClient) AnswerAgent(ctx context.Context, req AnswerAgentRequest, opts ...AnswerAgentOptions) (*AnswerAgentResult, error) {
	answerURL, _ := url.JoinPath(c.baseURL, "agents", "answer")

	// Merge options
	var opt AnswerAgentOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Marshal request
	reqBody, err := sonic.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling answer agent request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, answerURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// When streaming is enabled, expect SSE response instead of JSON
	if req.WithStreaming {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending answer agent request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("answer agent request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// If streaming is disabled, read JSON response directly
	if !req.WithStreaming {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		var result AnswerAgentResult
		if err := sonic.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parsing answer agent result: %w", err)
		}
		return &result, nil
	}

	// Build result from streaming events
	result := &AnswerAgentResult{}
	var answerBuilder strings.Builder

	// Read the SSE stream
	buf := make([]byte, 4096)
	var currentEvent string
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			lines := strings.SplitSeq(chunk, "\n")
			for line := range lines {
				// Parse SSE event type
				if after, ok := strings.CutPrefix(line, "event: "); ok {
					currentEvent = strings.TrimSpace(after)
					continue
				}

				// Parse SSE data
				if after, ok := strings.CutPrefix(line, "data: "); ok {
					data := after

					switch currentEvent {
					case "classification":
						// Parse classification and transformation JSON
						var classData ClassificationTransformationResult
						if err := sonic.UnmarshalString(data, &classData); err == nil {
							result.ClassificationTransformation = classData
							if opt.OnClassification != nil {
								if err := opt.OnClassification(&classData); err != nil {
									return nil, fmt.Errorf("classification callback error: %w", err)
								}
							}
						}

					case "reasoning":
						// Reasoning chunks during classification step
						if opt.OnReasoning != nil {
							if err := opt.OnReasoning(data); err != nil {
								return nil, fmt.Errorf("reasoning callback error: %w", err)
							}
						}

					case "hit":
						if opt.OnHit != nil {
							if err := opt.OnHit(data); err != nil {
								return nil, fmt.Errorf("hit callback error: %w", err)
							}
						}

					case "answer":
						answerBuilder.WriteString(data)
						if opt.OnAnswer != nil {
							if err := opt.OnAnswer(data); err != nil {
								return nil, fmt.Errorf("answer callback error: %w", err)
							}
						}

					case "followup_question":
						result.FollowupQuestions = append(result.FollowupQuestions, data)
						if opt.OnFollowupQuestion != nil {
							if err := opt.OnFollowupQuestion(data); err != nil {
								return nil, fmt.Errorf("followup question callback error: %w", err)
							}
						}

					case "done":
						// Stream complete
						break

					case "error":
						return nil, fmt.Errorf("answer agent error: %s", data)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading stream: %w", err)
		}
	}

	// Set the accumulated answer
	if answerBuilder.Len() > 0 {
		result.Answer = answerBuilder.String()
	}

	return result, nil
}

func NewEmbedderConfig(config any) (*EmbedderConfig, error) {
	var provider EmbedderProvider
	modelConfig := &EmbedderConfig{}
	switch v := config.(type) {
	case OllamaEmbedderConfig:
		provider = EmbedderProviderOllama
		modelConfig.FromOllamaEmbedderConfig(v)
	case OpenAIEmbedderConfig:
		provider = EmbedderProviderOpenai
		modelConfig.FromOpenAIEmbedderConfig(v)
	case GoogleEmbedderConfig:
		provider = EmbedderProviderGemini
		modelConfig.FromGoogleEmbedderConfig(v)
	case BedrockEmbedderConfig:
		provider = EmbedderProviderBedrock
		modelConfig.FromBedrockEmbedderConfig(v)
	case VertexEmbedderConfig:
		provider = EmbedderProviderVertex
		modelConfig.FromVertexEmbedderConfig(v)
	default:
		return nil, fmt.Errorf("unknown model config type: %T", v)
	}

	modelConfig.Provider = provider
	return modelConfig, nil
}
func NewGeneratorConfig(config any) (*GeneratorConfig, error) {
	var provider GeneratorProvider
	modelConfig := &GeneratorConfig{}
	switch v := config.(type) {
	case oapi.OllamaGeneratorConfig:
		provider = oapi.GeneratorProviderOllama
		modelConfig.FromOllamaGeneratorConfig(v)
	case oapi.OpenAIGeneratorConfig:
		provider = oapi.GeneratorProviderOpenai
		modelConfig.FromOpenAIGeneratorConfig(v)
	case oapi.GoogleGeneratorConfig:
		provider = oapi.GeneratorProviderGemini
		modelConfig.FromGoogleGeneratorConfig(v)
	case oapi.BedrockGeneratorConfig:
		provider = oapi.GeneratorProviderBedrock
		modelConfig.FromBedrockGeneratorConfig(v)
	case oapi.VertexGeneratorConfig:
		provider = oapi.GeneratorProviderVertex
		modelConfig.FromVertexGeneratorConfig(v)
	case oapi.AnthropicGeneratorConfig:
		provider = oapi.GeneratorProviderAnthropic
		modelConfig.FromAnthropicGeneratorConfig(v)
	default:
		return nil, fmt.Errorf("unknown model config type: %T", v)
	}

	modelConfig.Provider = provider
	return modelConfig, nil
}

func NewRerankerConfig(config any) (*RerankerConfig, error) {
	var provider RerankerProvider
	rerankerConfig := &RerankerConfig{}
	switch v := config.(type) {
	case OllamaRerankerConfig:
		provider = RerankerProviderOllama
		rerankerConfig.FromOllamaRerankerConfig(v)
	case TermiteRerankerConfig:
		provider = RerankerProviderTermite
		rerankerConfig.FromTermiteRerankerConfig(v)
	default:
		return nil, fmt.Errorf("unknown reranker config type: %T", v)
	}

	rerankerConfig.Provider = provider
	return rerankerConfig, nil
}

func NewIndexConfig(name string, config any) (*IndexConfig, error) {
	var t IndexType
	idxConfig := &IndexConfig{
		Name: name,
	}
	switch v := config.(type) {
	case EmbeddingIndexConfig:
		t = IndexTypeAknnV0
		idxConfig.FromEmbeddingIndexConfig(v)
	case BleveIndexV2Config:
		t = IndexTypeFullTextV0
		idxConfig.FromBleveIndexV2Config(v)
	default:
		return nil, fmt.Errorf("unsupported index config type: %T", config)
	}
	idxConfig.Type = t

	return idxConfig, nil

}
