// Copyright 2025 The Antfly Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package openrouter provides a Genkit plugin for OpenRouter.
// OpenRouter provides a unified API to access various LLM providers.
package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
)

const (
	provider   = "openrouter"
	apiBaseURL = "https://openrouter.ai/api/v1"
)

var roleMapping = map[ai.Role]string{
	ai.RoleUser:   "user",
	ai.RoleModel:  "assistant",
	ai.RoleSystem: "system",
	ai.RoleTool:   "tool",
}

// OpenRouter provides configuration options for the plugin.
type OpenRouter struct {
	// APIKey is the OpenRouter API key. If empty, reads from OPENROUTER_API_KEY env var.
	APIKey string
	// BaseURL is the OpenRouter API base URL. Defaults to https://openrouter.ai/api/v1
	BaseURL string
	// Timeout is the request timeout in seconds. Defaults to 120.
	Timeout int
	// SiteName is an optional site name for OpenRouter analytics.
	SiteName string
	// SiteURL is an optional site URL for OpenRouter analytics.
	SiteURL string

	mu      sync.Mutex
	initted bool
}

// Name returns the provider name.
func (o *OpenRouter) Name() string {
	return provider
}

// Init initializes the plugin.
func (o *OpenRouter) Init(ctx context.Context) []api.Action {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.initted {
		panic("openrouter.Init already called")
	}
	if o.APIKey == "" {
		o.APIKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if o.APIKey == "" {
		panic("openrouter: need APIKey or OPENROUTER_API_KEY environment variable")
	}
	if o.BaseURL == "" {
		o.BaseURL = apiBaseURL
	}
	if o.Timeout == 0 {
		o.Timeout = 120
	}
	o.initted = true
	return []api.Action{}
}

// ModelDefinition represents a model configuration.
type ModelDefinition struct {
	// Name is the OpenRouter model ID (e.g., "openai/gpt-4", "anthropic/claude-3-opus").
	// This is the primary model that will be used.
	Name string
	// Fallbacks is an optional list of fallback model IDs.
	// If the primary model is unavailable, OpenRouter will try these in order.
	// When fallbacks are specified, the "models" parameter is used instead of "model".
	Fallbacks []string
	// Label is an optional human-readable label.
	Label string
}

// DefineModel registers a model with Genkit.
func (o *OpenRouter) DefineModel(g *genkit.Genkit, model ModelDefinition, opts *ai.ModelOptions) ai.Model {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.initted {
		panic("openrouter.Init not called")
	}

	var modelOpts ai.ModelOptions
	if opts != nil {
		modelOpts = *opts
	} else {
		modelOpts = ai.ModelOptions{
			Label: model.Label,
			Supports: &ai.ModelSupports{
				Multiturn:  true,
				SystemRole: true,
				Media:      true, // Most models on OpenRouter support media
				Tools:      true, // Most models on OpenRouter support tools
			},
			Versions: []string{},
		}
	}
	if modelOpts.Label == "" {
		modelOpts.Label = "OpenRouter - " + model.Name
	}

	meta := &ai.ModelOptions{
		Label:    modelOpts.Label,
		Supports: modelOpts.Supports,
		Versions: modelOpts.Versions,
	}

	gen := &generator{
		model:    model,
		apiKey:   o.APIKey,
		baseURL:  o.BaseURL,
		timeout:  o.Timeout,
		siteName: o.SiteName,
		siteURL:  o.SiteURL,
	}

	return genkit.DefineModel(g, api.NewName(provider, model.Name), meta, gen.generate)
}

// IsDefinedModel reports whether a model is defined.
func IsDefinedModel(g *genkit.Genkit, name string) bool {
	return genkit.LookupModel(g, api.NewName(provider, name)) != nil
}

// Model returns the [ai.Model] with the given name.
func Model(g *genkit.Genkit, name string) ai.Model {
	return genkit.LookupModel(g, api.NewName(provider, name))
}

// generator handles API requests.
type generator struct {
	model    ModelDefinition
	apiKey   string
	baseURL  string
	timeout  int
	siteName string
	siteURL  string
}

// OpenRouter API types (OpenAI-compatible)
type chatRequest struct {
	// Model is the single model to use. Mutually exclusive with Models.
	Model string `json:"model,omitempty"`
	// Models is an array of models for fallback support.
	// OpenRouter will try each model in order until one succeeds.
	// Mutually exclusive with Model.
	Models           []string        `json:"models,omitempty"`
	Messages         []chatMessage   `json:"messages"`
	Stream           bool            `json:"stream,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	Tools            []chatTool      `json:"tools,omitempty"`
	ToolChoice       any             `json:"tool_choice,omitempty"`
	ResponseFormat   *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content"` // string or []contentPart
	Name       string         `json:"name,omitempty"`
	ToolCalls  []toolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type toolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolCallFunction `json:"function"`
}

type toolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type responseFormat struct {
	Type string `json:"type"` // "text" or "json_object"
}

type chatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
	Usage   *usage   `json:"usage,omitempty"`
}

type choice struct {
	Index        int          `json:"index"`
	Message      *chatMessage `json:"message,omitempty"`
	Delta        *chatMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
}

// generate handles the chat completion request.
func (g *generator) generate(ctx context.Context, input *ai.ModelRequest, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	stream := cb != nil

	// Convert messages
	messages, err := convertMessages(input.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// Build request
	req := chatRequest{
		Messages: messages,
		Stream:   stream,
	}

	// Use "models" array if fallbacks are specified, otherwise use single "model"
	if len(g.model.Fallbacks) > 0 {
		// Combine primary model with fallbacks
		models := make([]string, 0, 1+len(g.model.Fallbacks))
		models = append(models, g.model.Name)
		models = append(models, g.model.Fallbacks...)
		req.Models = models
	} else {
		req.Model = g.model.Name
	}

	// Add generation config
	if input.Config != nil {
		// Try to convert config to GenerationCommonConfig
		if cfg, ok := input.Config.(*ai.GenerationCommonConfig); ok && cfg != nil {
			if cfg.Temperature != 0 {
				temp := cfg.Temperature
				req.Temperature = &temp
			}
			if cfg.TopP != 0 {
				topP := cfg.TopP
				req.TopP = &topP
			}
			if cfg.MaxOutputTokens != 0 {
				maxTokens := cfg.MaxOutputTokens
				req.MaxTokens = &maxTokens
			}
			if len(cfg.StopSequences) > 0 {
				req.Stop = cfg.StopSequences
			}
		} else if cfg, ok := input.Config.(ai.GenerationCommonConfig); ok {
			if cfg.Temperature != 0 {
				temp := cfg.Temperature
				req.Temperature = &temp
			}
			if cfg.TopP != 0 {
				topP := cfg.TopP
				req.TopP = &topP
			}
			if cfg.MaxOutputTokens != 0 {
				maxTokens := cfg.MaxOutputTokens
				req.MaxTokens = &maxTokens
			}
			if len(cfg.StopSequences) > 0 {
				req.Stop = cfg.StopSequences
			}
		} else if cfgMap, ok := input.Config.(map[string]any); ok {
			// Handle config passed as a map
			if temp, ok := cfgMap["temperature"].(float64); ok && temp != 0 {
				req.Temperature = &temp
			}
			if topP, ok := cfgMap["topP"].(float64); ok && topP != 0 {
				req.TopP = &topP
			}
			if maxTokens, ok := cfgMap["maxOutputTokens"].(float64); ok && maxTokens != 0 {
				mt := int(maxTokens)
				req.MaxTokens = &mt
			}
			if stop, ok := cfgMap["stopSequences"].([]string); ok && len(stop) > 0 {
				req.Stop = stop
			}
		}
	}

	// Add tools
	if len(input.Tools) > 0 {
		tools, err := convertTools(input.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tools: %w", err)
		}
		req.Tools = tools
	}

	// Marshal request
	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"/chat/completions", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	if g.siteName != "" {
		httpReq.Header.Set("X-Title", g.siteName)
	}
	if g.siteURL != "" {
		httpReq.Header.Set("HTTP-Referer", g.siteURL)
	}

	client := &http.Client{Timeout: time.Duration(g.timeout) * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if stream {
		return g.handleStreamingResponse(ctx, input, resp, cb)
	}
	return g.handleNonStreamingResponse(input, resp)
}

func (g *generator) handleNonStreamingResponse(input *ai.ModelRequest, resp *http.Response) (*ai.ModelResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return translateResponse(&chatResp, input)
}

func (g *generator) handleStreamingResponse(ctx context.Context, input *ai.ModelRequest, resp *http.Response, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenRouter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chunks []*ai.ModelResponseChunk
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line == "data: [DONE]" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		var streamResp streamChunk
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed chunks
		}

		chunk, err := translateStreamChunk(&streamResp)
		if err != nil {
			continue
		}
		if chunk != nil {
			chunks = append(chunks, chunk)
			if err := cb(ctx, chunk); err != nil {
				return nil, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	// Build final response
	finalResponse := &ai.ModelResponse{
		Request:      input,
		FinishReason: ai.FinishReason("stop"),
		Message: &ai.Message{
			Role: ai.RoleModel,
		},
	}
	for _, chunk := range chunks {
		finalResponse.Message.Content = append(finalResponse.Message.Content, chunk.Content...)
	}

	return finalResponse, nil
}

func convertMessages(messages []*ai.Message) ([]chatMessage, error) {
	result := make([]chatMessage, 0, len(messages))

	for _, msg := range messages {
		converted, err := convertMessage(msg)
		if err != nil {
			return nil, err
		}
		result = append(result, converted...)
	}

	return result, nil
}

func convertMessage(msg *ai.Message) ([]chatMessage, error) {
	role := roleMapping[msg.Role]
	if role == "" {
		role = "user"
	}

	// Check if this is a tool response
	if msg.Role == ai.RoleTool {
		var messages []chatMessage
		for _, part := range msg.Content {
			if part.IsToolResponse() {
				outputJSON, err := json.Marshal(part.ToolResponse.Output)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool response: %w", err)
				}
				messages = append(messages, chatMessage{
					Role:       "tool",
					Content:    string(outputJSON),
					ToolCallID: part.ToolResponse.Ref,
				})
			}
		}
		return messages, nil
	}

	// Check for tool calls (from model)
	var toolCalls []toolCall
	var textParts []contentPart
	var hasMedia bool

	for _, part := range msg.Content {
		if part.IsToolRequest() {
			args, err := json.Marshal(part.ToolRequest.Input)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool request args: %w", err)
			}
			toolCalls = append(toolCalls, toolCall{
				ID:   part.ToolRequest.Ref,
				Type: "function",
				Function: toolCallFunction{
					Name:      part.ToolRequest.Name,
					Arguments: string(args),
				},
			})
		} else if part.IsText() {
			textParts = append(textParts, contentPart{
				Type: "text",
				Text: part.Text,
			})
		} else if part.IsMedia() {
			hasMedia = true
			// Handle media (images)
			// In Genkit, media content is stored in Text field as URL or data URI
			// ContentType contains the MIME type
			mediaURL := part.Text
			if mediaURL == "" && part.ContentType != "" {
				// Fallback: use content type as placeholder
				mediaURL = part.ContentType
			}
			textParts = append(textParts, contentPart{
				Type: "image_url",
				ImageURL: &imageURL{
					URL: mediaURL,
				},
			})
		}
	}

	chatMsg := chatMessage{
		Role: role,
	}

	if len(toolCalls) > 0 {
		chatMsg.ToolCalls = toolCalls
	}

	// Set content
	if hasMedia || len(textParts) > 1 {
		chatMsg.Content = textParts
	} else if len(textParts) == 1 && textParts[0].Type == "text" {
		chatMsg.Content = textParts[0].Text
	} else if len(textParts) == 0 && len(toolCalls) == 0 {
		chatMsg.Content = ""
	}

	return []chatMessage{chatMsg}, nil
}

func convertTools(tools []*ai.ToolDefinition) ([]chatTool, error) {
	result := make([]chatTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, chatTool{
			Type: "function",
			Function: toolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return result, nil
}

func translateResponse(resp *chatResponse, input *ai.ModelRequest) (*ai.ModelResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	choice := resp.Choices[0]
	modelResp := &ai.ModelResponse{
		Request:      input,
		FinishReason: ai.FinishReason(choice.FinishReason),
		Message: &ai.Message{
			Role: ai.RoleModel,
		},
	}

	if choice.Message != nil {
		// Handle tool calls
		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				var args any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = tc.Function.Arguments
				}
				toolReq := &ai.ToolRequest{
					Ref:   tc.ID,
					Name:  tc.Function.Name,
					Input: args,
				}
				modelResp.Message.Content = append(modelResp.Message.Content, ai.NewToolRequestPart(toolReq))
			}
		}

		// Handle text content
		if content, ok := choice.Message.Content.(string); ok && content != "" {
			modelResp.Message.Content = append(modelResp.Message.Content, ai.NewTextPart(content))
		}
	}

	// Add usage info
	if resp.Usage != nil {
		modelResp.Usage = &ai.GenerationUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}

	return modelResp, nil
}

func translateStreamChunk(chunk *streamChunk) (*ai.ModelResponseChunk, error) {
	if len(chunk.Choices) == 0 {
		return nil, nil
	}

	choice := chunk.Choices[0]
	if choice.Delta == nil {
		return nil, nil
	}

	result := &ai.ModelResponseChunk{}

	// Handle tool calls in stream
	if len(choice.Delta.ToolCalls) > 0 {
		for _, tc := range choice.Delta.ToolCalls {
			var args any
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = tc.Function.Arguments
				}
			}
			toolReq := &ai.ToolRequest{
				Ref:   tc.ID,
				Name:  tc.Function.Name,
				Input: args,
			}
			result.Content = append(result.Content, ai.NewToolRequestPart(toolReq))
		}
	}

	// Handle text content
	if content, ok := choice.Delta.Content.(string); ok && content != "" {
		result.Content = append(result.Content, ai.NewTextPart(content))
	}

	if len(result.Content) == 0 {
		return nil, nil
	}

	return result, nil
}
