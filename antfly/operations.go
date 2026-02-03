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

// RAG performs a RAG (Retrieval-Augmented Generation) query and streams the response
// Accepts a RAGRequest with one or more QueryRequests for single-table or multi-table RAG queries
// The callback function is called for each chunk of the streaming response
func (c *AntflyClient) RAG(ctx context.Context, ragReq RAGRequest, opts ...RAGOptions) (string, error) {
	// Merge options
	var opt RAGOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Marshal RAGRequest - QueryRequest.MarshalJSON handles the conversion automatically
	ragBody, err := json.Marshal(ragReq)
	if err != nil {
		return "", fmt.Errorf("marshalling RAG request: %w", err)
	}

	// Set Accept header based on streaming mode
	acceptHeader := func(_ context.Context, req *http.Request) error {
		if ragReq.WithStreaming {
			req.Header.Set("Accept", "text/event-stream")
		} else {
			req.Header.Set("Accept", "application/json")
		}
		return nil
	}

	resp, err := c.client.RagQueryWithBody(ctx, "application/json", bytes.NewBuffer(ragBody), acceptHeader)
	if err != nil {
		return "", fmt.Errorf("sending RAG request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("RAG request failed: %w", readErrorResponse(resp))
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

	// Read the SSE stream (RAG only uses data: lines, not event: lines)
	for _, data := range readSSEEvents(resp.Body) {
		if err := callback(data); err != nil {
			return "", fmt.Errorf("callback error: %w", err)
		}
	}

	return result.String(), nil
}

// ChatAgent performs a chat agent query with multi-turn conversation support.
// The agent maintains conversation context, uses tools (search, filter, clarification),
// and generates contextual answers.
// Supports streaming responses with granular callbacks for different event types.
func (c *AntflyClient) ChatAgent(ctx context.Context, req ChatAgentRequest, opts ...ChatAgentOptions) (*ChatAgentResult, error) {
	// Merge options
	var opt ChatAgentOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Marshal request - QueryRequest.MarshalJSON handles the conversion automatically
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling chat agent request: %w", err)
	}

	// Set Accept header based on streaming mode
	acceptHeader := func(_ context.Context, httpReq *http.Request) error {
		if req.WithStreaming {
			httpReq.Header.Set("Accept", "text/event-stream")
		} else {
			httpReq.Header.Set("Accept", "application/json")
		}
		return nil
	}

	resp, err := c.client.ChatAgentWithBody(ctx, "application/json", bytes.NewBuffer(reqBody), acceptHeader)
	if err != nil {
		return nil, fmt.Errorf("sending chat agent request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chat agent request failed: %w", readErrorResponse(resp))
	}

	// If streaming is disabled, read JSON response directly
	if !req.WithStreaming {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		var result ChatAgentResult
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parsing chat agent result: %w", err)
		}
		return &result, nil
	}

	// Build result from streaming events
	result := &ChatAgentResult{}
	var answerBuilder strings.Builder

	for eventType, data := range readSSEEvents(resp.Body) {
		switch eventType {
		case "classification":
			var classData ClassificationTransformationResult
			if json.UnmarshalString(data, &classData) == nil {
				result.ClassificationTransformation = classData
				if opt.OnClassification != nil {
					if err := opt.OnClassification(&classData); err != nil {
						return nil, fmt.Errorf("classification callback: %w", err)
					}
				}
			}
		case "hit":
			var hitData Hit
			if json.UnmarshalString(data, &hitData) == nil && opt.OnHit != nil {
				if err := opt.OnHit(&hitData); err != nil {
					return nil, fmt.Errorf("hit callback: %w", err)
				}
			}
		case "answer":
			var answerStr string
			if json.UnmarshalString(data, &answerStr) == nil {
				answerBuilder.WriteString(answerStr)
				if opt.OnAnswer != nil {
					if err := opt.OnAnswer(answerStr); err != nil {
						return nil, fmt.Errorf("answer callback: %w", err)
					}
				}
			}
		case "clarification_required":
			var clarification ClarificationRequest
			if json.UnmarshalString(data, &clarification) == nil {
				result.PendingClarification = clarification
				if opt.OnClarificationRequired != nil {
					if err := opt.OnClarificationRequired(&clarification); err != nil {
						return nil, fmt.Errorf("clarification callback: %w", err)
					}
				}
			}
		case "done":
			var doneData struct {
				AppliedFilters   []FilterSpec `json:"applied_filters"`
				AnswerConfidence float32      `json:"answer_confidence"`
			}
			if json.UnmarshalString(data, &doneData) == nil {
				result.AppliedFilters = doneData.AppliedFilters
				result.AnswerConfidence = doneData.AnswerConfidence
			}
		case "error":
			var chatErr ChatAgentError
			if json.UnmarshalString(data, &chatErr) != nil {
				chatErr = ChatAgentError{Error: data}
			}
			if opt.OnError != nil {
				if callbackErr := opt.OnError(&chatErr); callbackErr != nil {
					return nil, callbackErr
				}
			}
			return nil, fmt.Errorf("chat agent: %s", chatErr.Error)
		}
	}

	result.Answer = answerBuilder.String()
	return result, nil
}

// AnswerAgent performs an answer agent query with classification, query generation, and answer generation.
// The agent classifies the query, generates appropriate searches, executes them, and generates answers.
// Supports streaming responses with granular callbacks for different event types.
func (c *AntflyClient) AnswerAgent(ctx context.Context, req AnswerAgentRequest, opts ...AnswerAgentOptions) (*AnswerAgentResult, error) {
	// Merge options
	var opt AnswerAgentOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Marshal request - AnswerAgentRequest.MarshalJSON handles the conversion automatically
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling answer agent request: %w", err)
	}

	// Set Accept header based on streaming mode
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

	// If streaming is disabled, read JSON response directly
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

	// Build result from streaming events
	result := &AnswerAgentResult{}
	var answerBuilder strings.Builder

	for eventType, data := range readSSEEvents(resp.Body) {
		switch eventType {
		case "classification":
			var classData ClassificationTransformationResult
			if json.UnmarshalString(data, &classData) == nil {
				result.ClassificationTransformation = classData
				if opt.OnClassification != nil {
					if err := opt.OnClassification(&classData); err != nil {
						return nil, fmt.Errorf("classification callback: %w", err)
					}
				}
			}
		case "reasoning":
			var reasoningStr string
			if json.UnmarshalString(data, &reasoningStr) == nil && opt.OnReasoning != nil {
				if err := opt.OnReasoning(reasoningStr); err != nil {
					return nil, fmt.Errorf("reasoning callback: %w", err)
				}
			}
		case "hit":
			var hitData Hit
			if json.UnmarshalString(data, &hitData) == nil && opt.OnHit != nil {
				if err := opt.OnHit(&hitData); err != nil {
					return nil, fmt.Errorf("hit callback: %w", err)
				}
			}
		case "answer":
			var answerStr string
			if json.UnmarshalString(data, &answerStr) == nil {
				answerBuilder.WriteString(answerStr)
				if opt.OnAnswer != nil {
					if err := opt.OnAnswer(answerStr); err != nil {
						return nil, fmt.Errorf("answer callback: %w", err)
					}
				}
			}
		case "followup_question":
			var followupStr string
			if json.UnmarshalString(data, &followupStr) == nil {
				result.FollowupQuestions = append(result.FollowupQuestions, followupStr)
				if opt.OnFollowupQuestion != nil {
					if err := opt.OnFollowupQuestion(followupStr); err != nil {
						return nil, fmt.Errorf("followup callback: %w", err)
					}
				}
			}
		case "error":
			var agentErr AnswerAgentError
			if json.UnmarshalString(data, &agentErr) != nil {
				agentErr = AnswerAgentError{Error: data}
			}
			if opt.OnError != nil {
				if callbackErr := opt.OnError(&agentErr); callbackErr != nil {
					return nil, callbackErr
				}
			}
			if agentErr.Table != "" {
				return nil, fmt.Errorf("answer agent on table %s (status %d): %s", agentErr.Table, agentErr.Status, agentErr.Error)
			}
			return nil, fmt.Errorf("answer agent: %s", agentErr.Error)
		}
	}

	result.Answer = answerBuilder.String()
	return result, nil
}
