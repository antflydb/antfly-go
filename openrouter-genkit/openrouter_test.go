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

package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

func TestOpenRouterPlugin(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Authorization header 'Bearer test-api-key', got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}

		// Read and verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var req chatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		if req.Model != "openai/gpt-4" {
			t.Errorf("expected model 'openai/gpt-4', got %s", req.Model)
		}

		// Send mock response
		resp := chatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "openai/gpt-4",
			Choices: []choice{
				{
					Index: 0,
					Message: &chatMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: &usage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Initialize plugin
	ctx := context.Background()
	plugin := &OpenRouter{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 30,
	}

	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	// Define model
	model := plugin.DefineModel(g, ModelDefinition{
		Name:  "openai/gpt-4",
		Label: "GPT-4",
	}, nil)

	// Test generation
	resp, err := model.Generate(ctx, &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("Hello!")},
			},
		},
	}, nil)

	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if len(resp.Message.Content) == 0 {
		t.Fatal("response has no content")
	}

	text := resp.Message.Content[0].Text
	if text != "Hello! How can I help you?" {
		t.Errorf("expected 'Hello! How can I help you?', got %s", text)
	}

	if resp.Usage == nil {
		t.Fatal("usage is nil")
	}

	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.Usage.InputTokens)
	}

	if resp.Usage.OutputTokens != 8 {
		t.Errorf("expected 8 output tokens, got %d", resp.Usage.OutputTokens)
	}
}

func TestConvertMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []*ai.Message
		wantLen  int
		wantErr  bool
	}{
		{
			name: "simple text message",
			messages: []*ai.Message{
				{
					Role:    ai.RoleUser,
					Content: []*ai.Part{ai.NewTextPart("Hello")},
				},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "system and user messages",
			messages: []*ai.Message{
				{
					Role:    ai.RoleSystem,
					Content: []*ai.Part{ai.NewTextPart("You are helpful")},
				},
				{
					Role:    ai.RoleUser,
					Content: []*ai.Part{ai.NewTextPart("Hello")},
				},
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "tool response",
			messages: []*ai.Message{
				{
					Role: ai.RoleTool,
					Content: []*ai.Part{
						ai.NewToolResponsePart(&ai.ToolResponse{
							Ref:    "call_123",
							Name:   "get_weather",
							Output: map[string]any{"temp": 72},
						}),
					},
				},
			},
			wantLen: 1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertMessages(tt.messages)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertMessages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("convertMessages() got %d messages, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestConvertTools(t *testing.T) {
	tools := []*ai.ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get the weather for a location",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{
						"type":        "string",
						"description": "The city and state",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	got, err := convertTools(tools)
	if err != nil {
		t.Fatalf("convertTools() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}

	if got[0].Type != "function" {
		t.Errorf("expected type 'function', got %s", got[0].Type)
	}

	if got[0].Function.Name != "get_weather" {
		t.Errorf("expected name 'get_weather', got %s", got[0].Function.Name)
	}

	if got[0].Function.Description != "Get the weather for a location" {
		t.Errorf("expected description 'Get the weather for a location', got %s", got[0].Function.Description)
	}
}

func TestToolCallResponse(t *testing.T) {
	// Create a mock server that returns a tool call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "openai/gpt-4",
			Choices: []choice{
				{
					Index: 0,
					Message: &chatMessage{
						Role: "assistant",
						ToolCalls: []toolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: toolCallFunction{
									Name:      "get_weather",
									Arguments: `{"location": "San Francisco, CA"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	plugin := &OpenRouter{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 30,
	}

	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	model := plugin.DefineModel(g, ModelDefinition{
		Name:  "openai/gpt-4",
		Label: "GPT-4",
	}, nil)

	resp, err := model.Generate(ctx, &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("What's the weather in SF?")},
			},
		},
		Tools: []*ai.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
				},
			},
		},
	}, nil)

	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if len(resp.Message.Content) == 0 {
		t.Fatal("response has no content")
	}

	part := resp.Message.Content[0]
	if !part.IsToolRequest() {
		t.Fatal("expected tool request part")
	}

	if part.ToolRequest.Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %s", part.ToolRequest.Name)
	}

	if part.ToolRequest.Ref != "call_123" {
		t.Errorf("expected tool ref 'call_123', got %s", part.ToolRequest.Ref)
	}
}

func TestStreamingResponse(t *testing.T) {
	// Create a mock server for streaming
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		chunks := []string{
			`data: {"id":"test-id","object":"chat.completion.chunk","created":1234567890,"model":"openai/gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"test-id","object":"chat.completion.chunk","created":1234567890,"model":"openai/gpt-4","choices":[{"index":0,"delta":{"content":" there!"},"finish_reason":null}]}`,
			`data: {"id":"test-id","object":"chat.completion.chunk","created":1234567890,"model":"openai/gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			"data: [DONE]",
		}

		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	ctx := context.Background()
	plugin := &OpenRouter{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 30,
	}

	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	model := plugin.DefineModel(g, ModelDefinition{
		Name:  "openai/gpt-4",
		Label: "GPT-4",
	}, nil)

	var chunks []string
	resp, err := model.Generate(ctx, &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("Hello!")},
			},
		},
	}, func(ctx context.Context, chunk *ai.ModelResponseChunk) error {
		for _, part := range chunk.Content {
			if part.IsText() {
				chunks = append(chunks, part.Text)
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}

	expectedChunks := []string{"Hello", " there!"}
	for i, expected := range expectedChunks {
		if i < len(chunks) && chunks[i] != expected {
			t.Errorf("chunk %d: expected %q, got %q", i, expected, chunks[i])
		}
	}
}

func TestModelLookup(t *testing.T) {
	ctx := context.Background()
	plugin := &OpenRouter{
		APIKey:  "test-api-key",
		BaseURL: "http://localhost:8080",
		Timeout: 30,
	}

	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	// Model should not exist before defining
	if IsDefinedModel(g, "test-model") {
		t.Error("model should not be defined before DefineModel")
	}

	plugin.DefineModel(g, ModelDefinition{Name: "test-model"}, nil)

	// Model should exist after defining
	if !IsDefinedModel(g, "test-model") {
		t.Error("model should be defined after DefineModel")
	}

	// Lookup should return the model
	model := Model(g, "test-model")
	if model == nil {
		t.Error("Model() should return non-nil after DefineModel")
	}
}

func TestFallbackModels(t *testing.T) {
	// Create a mock server that verifies the "models" array is sent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		// Verify "models" array is present instead of "model"
		if _, hasModel := req["model"]; hasModel {
			t.Error("expected 'models' array, but got 'model' string")
		}

		models, hasModels := req["models"].([]any)
		if !hasModels {
			t.Fatal("expected 'models' array in request")
		}

		if len(models) != 3 {
			t.Errorf("expected 3 models in array, got %d", len(models))
		}

		expectedModels := []string{"anthropic/claude-3-opus", "anthropic/claude-3-sonnet", "openai/gpt-4"}
		for i, expected := range expectedModels {
			if i < len(models) {
				if actual, ok := models[i].(string); !ok || actual != expected {
					t.Errorf("model[%d]: expected %q, got %v", i, expected, models[i])
				}
			}
		}

		// Send mock response
		resp := chatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "anthropic/claude-3-opus", // The model that was actually used
			Choices: []choice{
				{
					Index: 0,
					Message: &chatMessage{
						Role:    "assistant",
						Content: "Hello from Claude!",
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	plugin := &OpenRouter{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 30,
	}

	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	// Define model with fallbacks
	model := plugin.DefineModel(g, ModelDefinition{
		Name:      "anthropic/claude-3-opus",
		Fallbacks: []string{"anthropic/claude-3-sonnet", "openai/gpt-4"},
		Label:     "Claude with Fallbacks",
	}, nil)

	resp, err := model.Generate(ctx, &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("Hello!")},
			},
		},
	}, nil)

	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if len(resp.Message.Content) == 0 {
		t.Fatal("response has no content")
	}

	text := resp.Message.Content[0].Text
	if text != "Hello from Claude!" {
		t.Errorf("expected 'Hello from Claude!', got %s", text)
	}
}

func TestSingleModelNoFallback(t *testing.T) {
	// Create a mock server that verifies "model" string is sent (not "models" array)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		// Verify "model" string is present, not "models" array
		if _, hasModels := req["models"]; hasModels {
			t.Error("expected 'model' string, but got 'models' array")
		}

		model, hasModel := req["model"].(string)
		if !hasModel {
			t.Fatal("expected 'model' string in request")
		}

		if model != "openai/gpt-4" {
			t.Errorf("expected model 'openai/gpt-4', got %s", model)
		}

		// Send mock response
		resp := chatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "openai/gpt-4",
			Choices: []choice{
				{
					Index: 0,
					Message: &chatMessage{
						Role:    "assistant",
						Content: "Hello!",
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	plugin := &OpenRouter{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 30,
	}

	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	// Define model WITHOUT fallbacks
	model := plugin.DefineModel(g, ModelDefinition{
		Name:  "openai/gpt-4",
		Label: "GPT-4",
	}, nil)

	resp, err := model.Generate(ctx, &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("Hello!")},
			},
		},
	}, nil)

	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}
}
