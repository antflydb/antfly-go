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

package antfly

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/antflydb/antfly-go/antfly/oapi"
	"github.com/antflydb/antfly-go/antfly/query"
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

	// Aggregations to compute
	Aggregations map[string]AggregationRequest `json:"aggregations,omitempty"`

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
	Pruner Pruner `json:"pruner,omitzero"`

	// SemanticSearch text to use for semantic similarity search
	SemanticSearch string `json:"semantic_search,omitempty"`

	// DocumentRenderer optional Go template string for rendering document content to the prompt
	DocumentRenderer string `json:"document_renderer,omitempty"`

	// GraphSearches declarative graph queries to execute after full-text/vector searches.
	// Results can reference search results using node selectors like $full_text_results.
	GraphSearches map[string]oapi.GraphQuery `json:"graph_searches,omitempty"`
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
		Aggregations:     q.Aggregations,
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
		GraphSearches:    q.GraphSearches,
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
	q.Aggregations = oapiReq.Aggregations
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
	q.GraphSearches = oapiReq.GraphSearches

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

	// Generator model configuration for generation.
	// Mutually exclusive with Chain. Either Generator or Chain must be provided.
	Generator GeneratorConfig `json:"generator,omitzero"`

	// Chain of generators with retry/fallback semantics.
	// Mutually exclusive with Generator. Either Generator or Chain must be provided.
	// Each link can specify retry configuration and a condition for trying the next generator.
	Chain []ChainLink `json:"chain,omitempty,omitzero"`

	// Prompt is a Handlebars template for customizing the user prompt sent to the generator.
	// You can use Handlebars template syntax to customize the prompt, including loops and conditionals.
	Prompt string `json:"prompt,omitempty"`

	// SystemPrompt optional system prompt to guide the summarization
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Eval optional evaluation configuration. When provided, runs evaluators on the query results
	// and includes scores in the response.
	Eval *oapi.EvalConfig `json:"eval,omitempty"`

	// WithStreaming Enable SSE streaming of results instead of JSON response
	WithStreaming bool `json:"with_streaming,omitempty,omitzero"`
}

// AnswerAgentRequest represents an answer agent request.
// The answer agent classifies queries, transforms them for optimal semantic search, executes provided queries, and generates answers.
type AnswerAgentRequest struct {
	// Query is the user's natural language query (required)
	Query string `json:"query"`

	// AgentKnowledge is background knowledge that guides the agent's understanding of the domain.
	// Similar to CLAUDE.md, this provides context that applies to all steps
	// (classification, retrieval, and answer generation).
	AgentKnowledge string `json:"agent_knowledge,omitempty"`

	// Generator is the default model configuration for all pipeline steps.
	// Mutually exclusive with Chain. Either Generator or Chain must be provided.
	Generator GeneratorConfig `json:"generator,omitzero"`

	// Chain of generators with retry/fallback semantics.
	// Mutually exclusive with Generator. Either Generator or Chain must be provided.
	// Each link can specify retry configuration and a condition for trying the next generator.
	Chain []ChainLink `json:"chain,omitempty,omitzero"`

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

	// Eval is the configuration for inline evaluation of query results
	Eval EvalConfig `json:"eval,omitzero"`
}

// RAGOptions contains optional parameters for RAG requests
type RAGOptions struct {
	// Callback is called for each chunk of the streaming response
	// If not provided, chunks are written to a default buffer
	Callback func(chunk string) error
}

// AnswerAgentError represents an error received during streaming from the answer agent.
// Errors can contain additional context like the table name and HTTP status code.
type AnswerAgentError struct {
	// Error is the error message
	Error string `json:"error"`

	// Status is the HTTP status code (optional, present for query execution errors)
	Status int32 `json:"status,omitempty"`

	// Table is the table name where the error occurred (optional, present for query execution errors)
	Table string `json:"table,omitempty"`
}

// AnswerAgentOptions contains optional parameters for answer agent requests
type AnswerAgentOptions struct {
	// OnClassification is called when the classification and transformation result is received
	// Receives the full ClassificationTransformationResult with route_type, strategy, semantic_query, etc.
	OnClassification func(result *ClassificationTransformationResult) error

	// OnReasoning is called for each chunk of reasoning during classification (if steps.classification.with_reasoning is enabled)
	OnReasoning func(chunk string) error

	// OnHit is called for each search result hit
	OnHit func(hit *Hit) error

	// OnAnswer is called for each chunk of the answer text
	OnAnswer func(chunk string) error

	// OnFollowupQuestion is called for each follow-up question (if steps.followup.enabled is true)
	OnFollowupQuestion func(question string) error

	// OnError is called when an error event is received during streaming.
	// If the callback returns nil, the error is still returned from AnswerAgent.
	// If the callback returns an error, that error is returned instead.
	OnError func(err *AnswerAgentError) error
}
