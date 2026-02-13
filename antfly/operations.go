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
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/antflydb/antfly-go/libaf/json"
	"github.com/antflydb/antfly-go/antfly/oapi"
)

// readSSEEvents reads SSE events from a reader and yields (eventType, data) pairs.
// Events are parsed from "event: <type>" and "data: <content>" lines.
func readSSEEvents(r io.Reader) iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		buf := make([]byte, 4096)
		var partial string // buffer for incomplete lines across reads
		var currentEvent string
		for {
			n, err := r.Read(buf)
			if n > 0 {
				chunk := partial + string(buf[:n])
				lines := strings.Split(chunk, "\n")
				// Last element may be incomplete; save for next read
				partial = lines[len(lines)-1]
				for _, line := range lines[:len(lines)-1] {
					if after, ok := strings.CutPrefix(line, "event: "); ok {
						currentEvent = strings.TrimSpace(after)
					} else if after, ok := strings.CutPrefix(line, "data: "); ok {
						if !yield(currentEvent, after) {
							return
						}
					}
				}
			}
			if err != nil {
				return
			}
		}
	}
}

// Query executes queries against a table
func (c *AntflyClient) Query(ctx context.Context, opts ...QueryRequest) (*QueryResponses, error) {
	request := bytes.NewBuffer(nil)
	e := json.NewEncoder(request)
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
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing result: %w", err)
	}

	return &result, nil
}

// Batch performs a batch operation on a table
func (c *AntflyClient) Batch(ctx context.Context, tableName string, request BatchRequest) (*BatchResult, error) {
	batchBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshalling batch request: %w", err)
	}

	resp, err := c.client.BatchWriteWithBody(ctx, tableName, "application/json", bytes.NewBuffer(batchBody))
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
		if err := json.Unmarshal(respBody, &result); err != nil {
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
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parsing linear merge result: %w", err)
		}
	}

	return &result, nil
}

// LookupKey looks up a document by its key.
// Use LookupKeyWithFields if you need to specify which fields to return.
func (c *AntflyClient) LookupKey(ctx context.Context, tableName, key string) (map[string]any, error) {
	return c.LookupKeyWithFields(ctx, tableName, key, "")
}

// LookupKeyWithFields looks up a document by its key with optional field projection.
// The fields parameter is a comma-separated list of fields to include in the response.
// If empty, returns the full document. Supports:
// - Simple fields: "title,author"
// - Nested paths: "user.address.city"
// - Wildcards: "_chunks.*"
// - Exclusions: "-_chunks.*._embedding"
// - Special fields: "_embeddings,_summaries,_chunks"
func (c *AntflyClient) LookupKeyWithFields(ctx context.Context, tableName, key, fields string) (map[string]any, error) {
	var params *oapi.LookupKeyParams
	if fields != "" {
		params = &oapi.LookupKeyParams{Fields: fields}
	}
	resp, err := c.client.LookupKey(ctx, tableName, key, params)
	if err != nil {
		return nil, fmt.Errorf("looking up key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("looking up key: %w", readErrorResponse(resp))
	}

	// Parse the response
	var document map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return document, nil
}

// ScanKeys scans keys in a table within an optional key range.
// Returns keys and optionally document data based on the request parameters.
func (c *AntflyClient) ScanKeys(ctx context.Context, tableName string, request ScanKeysRequest) ([]map[string]any, error) {
	resp, err := c.client.ScanKeys(ctx, tableName, oapi.ScanKeysRequest(request))
	if err != nil {
		return nil, fmt.Errorf("scanning keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("scanning keys: %w", readErrorResponse(resp))
	}

	// Parse the response as array of documents
	var documents []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&documents); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return documents, nil
}

// RetrievalAgentOptions configures streaming callbacks for the retrieval agent
type RetrievalAgentOptions struct {
	OnDFAState         func(state string, iteration int) error
	OnHit              func(hit *Hit) error
	OnError            func(err *RetrievalAgentError) error
	OnTreeLevel        func(depth int, numNodes int) error        // Called for tree search level progress
	OnSufficiencyCheck func(sufficient bool, reason string) error // Called when sufficiency is evaluated
	OnTreeSearchStart  func(index string, maxDepth int, beamWidth int) error
	OnTreeSearchDone   func(collected int, depth int) error
}

// RetrievalAgentError represents an error from the retrieval agent
type RetrievalAgentError struct {
	Error string `json:"error"`
}

// RetrievalAgent performs DFA-based document retrieval with strategy selection and query refinement.
// The agent uses a state machine: clarify -> select_strategy -> refine_query -> execute.
// Supports streaming responses with callbacks for DFA state transitions and retrieved documents.
func (c *AntflyClient) RetrievalAgent(ctx context.Context, req RetrievalAgentRequest, opts ...RetrievalAgentOptions) (*RetrievalAgentResult, error) {
	// Merge options
	var opt RetrievalAgentOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling retrieval agent request: %w", err)
	}

	// Set Accept header based on streaming mode
	acceptHeader := func(_ context.Context, httpReq *http.Request) error {
		if req.Stream {
			httpReq.Header.Set("Accept", "text/event-stream")
		} else {
			httpReq.Header.Set("Accept", "application/json")
		}
		return nil
	}

	resp, err := c.client.RetrievalAgentWithBody(ctx, "application/json", bytes.NewBuffer(reqBody), acceptHeader)
	if err != nil {
		return nil, fmt.Errorf("sending retrieval agent request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("retrieval agent request failed: %w", readErrorResponse(resp))
	}

	// If streaming is disabled, read JSON response directly
	if !req.Stream {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		var result RetrievalAgentResult
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parsing retrieval agent result: %w", err)
		}
		return &result, nil
	}

	// Build result from streaming events
	result := &RetrievalAgentResult{}

	for eventType, data := range readSSEEvents(resp.Body) {
		switch eventType {
		case "dfa_state":
			var stateData struct {
				State     string `json:"state"`
				Iteration int    `json:"iteration"`
			}
			if json.UnmarshalString(data, &stateData) == nil && opt.OnDFAState != nil {
				if err := opt.OnDFAState(stateData.State, stateData.Iteration); err != nil {
					return nil, fmt.Errorf("dfa_state callback: %w", err)
				}
			}
		case "hit":
			var hitData Hit
			if json.UnmarshalString(data, &hitData) == nil && opt.OnHit != nil {
				if err := opt.OnHit(&hitData); err != nil {
					return nil, fmt.Errorf("hit callback: %w", err)
				}
			}
		case "tree_search_start":
			var treeStartData struct {
				Index     string `json:"index"`
				MaxDepth  int    `json:"max_depth"`
				BeamWidth int    `json:"beam_width"`
			}
			if json.UnmarshalString(data, &treeStartData) == nil && opt.OnTreeSearchStart != nil {
				if err := opt.OnTreeSearchStart(treeStartData.Index, treeStartData.MaxDepth, treeStartData.BeamWidth); err != nil {
					return nil, fmt.Errorf("tree_search_start callback: %w", err)
				}
			}
		case "tree_level":
			var levelData struct {
				Depth    int `json:"depth"`
				NumNodes int `json:"num_nodes"`
			}
			if json.UnmarshalString(data, &levelData) == nil && opt.OnTreeLevel != nil {
				if err := opt.OnTreeLevel(levelData.Depth, levelData.NumNodes); err != nil {
					return nil, fmt.Errorf("tree_level callback: %w", err)
				}
			}
		case "sufficiency_check":
			var suffData struct {
				Sufficient bool   `json:"sufficient"`
				Reason     string `json:"reason"`
			}
			if json.UnmarshalString(data, &suffData) == nil && opt.OnSufficiencyCheck != nil {
				if err := opt.OnSufficiencyCheck(suffData.Sufficient, suffData.Reason); err != nil {
					return nil, fmt.Errorf("sufficiency_check callback: %w", err)
				}
			}
		case "tree_search_complete":
			var treeDoneData struct {
				Collected int `json:"collected"`
				Depth     int `json:"depth"`
			}
			if json.UnmarshalString(data, &treeDoneData) == nil && opt.OnTreeSearchDone != nil {
				if err := opt.OnTreeSearchDone(treeDoneData.Collected, treeDoneData.Depth); err != nil {
					return nil, fmt.Errorf("tree_search_complete callback: %w", err)
				}
			}
		case "done":
			if json.UnmarshalString(data, result) != nil {
				// If parsing fails, try to extract what we can
			}
		case "error":
			var agentErr RetrievalAgentError
			if json.UnmarshalString(data, &agentErr) != nil {
				agentErr = RetrievalAgentError{Error: data}
			}
			if opt.OnError != nil {
				if callbackErr := opt.OnError(&agentErr); callbackErr != nil {
					return nil, callbackErr
				}
			}
			return nil, fmt.Errorf("retrieval agent: %s", agentErr.Error)
		}
	}

	return result, nil
}

// AnswerAgent performs a deprecated answer agent request.
// This is a backward-compatible wrapper around the /agents/answer endpoint.
// New code should use RetrievalAgent instead.
func (c *AntflyClient) AnswerAgent(ctx context.Context, req AnswerAgentRequest) (*AnswerAgentResult, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling answer agent request: %w", err)
	}

	acceptHeader := func(_ context.Context, httpReq *http.Request) error {
		if req.WithStreaming {
			httpReq.Header.Set("Accept", "text/event-stream")
		} else {
			httpReq.Header.Set("Accept", "application/json")
		}
		return nil
	}

	resp, err := c.client.AnswerAgentWithBody(ctx, "application/json", bytes.NewBuffer(reqBody), acceptHeader)
	if err != nil {
		return nil, fmt.Errorf("sending answer agent request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("answer agent request failed: %w", readErrorResponse(resp))
	}

	if !req.WithStreaming {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		var result AnswerAgentResult
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parsing answer agent result: %w", err)
		}
		return &result, nil
	}

	// Streaming mode: read SSE events, return the done payload
	var result AnswerAgentResult
	for eventType, data := range readSSEEvents(resp.Body) {
		switch eventType {
		case "done":
			_ = json.UnmarshalString(data, &result)
		case "error":
			var agentErr RetrievalAgentError
			if json.UnmarshalString(data, &agentErr) != nil {
				agentErr = RetrievalAgentError{Error: data}
			}
			return nil, fmt.Errorf("answer agent: %s", agentErr.Error)
		}
	}

	return &result, nil
}

