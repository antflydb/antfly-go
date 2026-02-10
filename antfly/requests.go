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
	"fmt"

	"github.com/antflydb/antfly-go/libaf/json"
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

	// Transforms Array of transform operations for in-place document updates using MongoDB-style operators.
	// Transform operations allow you to modify documents without read-modify-write races:
	// - Operations are applied atomically on the server
	// - Multiple operations per document are applied in sequence
	// - Supports numeric operations ($inc, $mul, $min, $max), array operations ($push, $pull), and more
	Transforms []Transform `json:"transforms,omitempty"`

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

	// Transformed Number of documents successfully transformed
	Transformed int `json:"transformed,omitempty"`

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
	GraphSearches map[string]GraphQuery `json:"graph_searches,omitempty"`

	// Join configuration for joining data from another table.
	// Supports inner, left, and right joins with automatic strategy selection.
	Join JoinClause `json:"join,omitempty"`
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
		Join:             q.Join,
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
	q.Join = oapiReq.Join

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


