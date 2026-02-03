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
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadSSEEvents(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   []struct{ event, data string }
	}{
		{
			name:  "single event",
			input: "event: test\ndata: hello\n\n",
			want:  []struct{ event, data string }{{"test", "hello"}},
		},
		{
			name:  "multiple events same type",
			input: "event: msg\ndata: one\ndata: two\ndata: three\n",
			want: []struct{ event, data string }{
				{"msg", "one"},
				{"msg", "two"},
				{"msg", "three"},
			},
		},
		{
			name:  "different event types",
			input: "event: classification\ndata: {\"type\":\"search\"}\nevent: hit\ndata: {\"id\":\"1\"}\nevent: done\ndata: {}\n",
			want: []struct{ event, data string }{
				{"classification", `{"type":"search"}`},
				{"hit", `{"id":"1"}`},
				{"done", "{}"},
			},
		},
		{
			name:  "data without event type",
			input: "data: orphan\n",
			want:  []struct{ event, data string }{{"", "orphan"}},
		},
		{
			name:  "event type persists",
			input: "event: answer\ndata: chunk1\ndata: chunk2\nevent: done\ndata: {}\n",
			want: []struct{ event, data string }{
				{"answer", "chunk1"},
				{"answer", "chunk2"},
				{"done", "{}"},
			},
		},
		{
			name:  "ignores non-sse lines",
			input: "comment line\nevent: test\ndata: value\nrandom\n",
			want:  []struct{ event, data string }{{"test", "value"}},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []struct{ event, data string }
			for event, data := range readSSEEvents(strings.NewReader(tt.input)) {
				got = append(got, struct{ event, data string }{event, data})
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %d events, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].event != tt.want[i].event {
					t.Errorf("event[%d].event = %q, want %q", i, got[i].event, tt.want[i].event)
				}
				if got[i].data != tt.want[i].data {
					t.Errorf("event[%d].data = %q, want %q", i, got[i].data, tt.want[i].data)
				}
			}
		})
	}
}

// chunkedReader splits reads at arbitrary boundaries to test partial line handling
type chunkedReader struct {
	data      string
	chunkSize int
	pos       int
}

func (r *chunkedReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	end := r.pos + r.chunkSize
	if end > len(r.data) {
		end = len(r.data)
	}
	n = copy(p, r.data[r.pos:end])
	r.pos = end
	return n, nil
}

func TestReadSSEEventsPartialLines(t *testing.T) {
	input := "event: classification\ndata: {\"query\":\"test\"}\nevent: hit\ndata: {\"id\":\"doc1\"}\nevent: done\ndata: {}\n"

	// Test with various chunk sizes to ensure partial line handling works
	for _, chunkSize := range []int{1, 2, 3, 5, 7, 13, 17, 64, len(input)} {
		t.Run(fmt.Sprintf("chunk_%d", chunkSize), func(t *testing.T) {
			reader := &chunkedReader{data: input, chunkSize: chunkSize}
			var events []struct{ event, data string }
			for event, data := range readSSEEvents(reader) {
				events = append(events, struct{ event, data string }{event, data})
			}

			if len(events) != 3 {
				t.Errorf("chunkSize=%d: got %d events, want 3", chunkSize, len(events))
				return
			}
			if events[0].event != "classification" || events[0].data != `{"query":"test"}` {
				t.Errorf("chunkSize=%d: event[0] = %+v", chunkSize, events[0])
			}
			if events[1].event != "hit" || events[1].data != `{"id":"doc1"}` {
				t.Errorf("chunkSize=%d: event[1] = %+v", chunkSize, events[1])
			}
			if events[2].event != "done" || events[2].data != "{}" {
				t.Errorf("chunkSize=%d: event[2] = %+v", chunkSize, events[2])
			}
		})
	}
}

func TestReadSSEEventsEarlyTermination(t *testing.T) {
	input := "event: a\ndata: 1\nevent: b\ndata: 2\nevent: c\ndata: 3\n"

	// Stop after first event
	count := 0
	for _, _ = range readSSEEvents(strings.NewReader(input)) {
		count++
		if count >= 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early termination: got %d events, want 1", count)
	}
}

func TestChatAgentStreaming(t *testing.T) {
	sseResponse := `event: classification
data: {"route_type":"search","semantic_query":"test query"}
event: hit
data: {"id":"doc1","score":0.95}
event: answer
data: "Hello "
event: answer
data: "World"
event: done
data: {"answer_confidence":0.9,"applied_filters":[]}
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agents/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("expected Accept: text/event-stream, got %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseResponse))
	}))
	defer server.Close()

	client, err := NewAntflyClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewAntflyClient: %v", err)
	}

	var classificationReceived bool
	var hitsReceived int
	var answerChunks []string

	result, err := client.ChatAgent(context.Background(), ChatAgentRequest{
		Messages:      []ChatMessage{{Role: "user", Content: "test"}},
		WithStreaming: true,
	}, ChatAgentOptions{
		OnClassification: func(result *ClassificationTransformationResult) error {
			classificationReceived = true
			if result.RouteType != "search" {
				t.Errorf("got route_type %q, want %q", result.RouteType, "search")
			}
			return nil
		},
		OnHit: func(hit *Hit) error {
			hitsReceived++
			return nil
		},
		OnAnswer: func(chunk string) error {
			answerChunks = append(answerChunks, chunk)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ChatAgent: %v", err)
	}

	if !classificationReceived {
		t.Error("classification callback was not called")
	}
	if hitsReceived != 1 {
		t.Errorf("got %d hits, want 1", hitsReceived)
	}
	if len(answerChunks) != 2 {
		t.Errorf("got %d answer chunks, want 2", len(answerChunks))
	}
	if result.Answer != "Hello World" {
		t.Errorf("got answer %q, want %q", result.Answer, "Hello World")
	}
	if result.AnswerConfidence != 0.9 {
		t.Errorf("got answer_confidence %v, want 0.9", result.AnswerConfidence)
	}
}

func TestChatAgentNonStreaming(t *testing.T) {
	jsonResponse := `{"answer":"Test answer","answer_confidence":0.85,"classification_transformation":{"route_type":"search"}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json, got %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonResponse))
	}))
	defer server.Close()

	client, err := NewAntflyClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewAntflyClient: %v", err)
	}

	result, err := client.ChatAgent(context.Background(), ChatAgentRequest{
		Messages:      []ChatMessage{{Role: "user", Content: "test"}},
		WithStreaming: false,
	})
	if err != nil {
		t.Fatalf("ChatAgent: %v", err)
	}

	if result.Answer != "Test answer" {
		t.Errorf("got answer %q, want %q", result.Answer, "Test answer")
	}
	if result.AnswerConfidence != 0.85 {
		t.Errorf("got answer_confidence %v, want 0.85", result.AnswerConfidence)
	}
}

func TestChatAgentStreamingError(t *testing.T) {
	sseResponse := `event: classification
data: {"route_type":"search"}
event: error
data: {"error":"something went wrong"}
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseResponse))
	}))
	defer server.Close()

	client, err := NewAntflyClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewAntflyClient: %v", err)
	}

	var errorReceived bool
	_, err = client.ChatAgent(context.Background(), ChatAgentRequest{
		Messages:      []ChatMessage{{Role: "user", Content: "test"}},
		WithStreaming: true,
	}, ChatAgentOptions{
		OnError: func(e *ChatAgentError) error {
			errorReceived = true
			if e.Error != "something went wrong" {
				t.Errorf("got error %q, want %q", e.Error, "something went wrong")
			}
			return nil
		},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errorReceived {
		t.Error("error callback was not called")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("error message should contain 'something went wrong', got %q", err.Error())
	}
}

func TestAnswerAgentStreaming(t *testing.T) {
	sseResponse := `event: classification
data: {"route_type":"search","semantic_query":"transformed query"}
event: hit
data: {"id":"doc1","score":0.9}
event: hit
data: {"id":"doc2","score":0.8}
event: answer
data: "Part one "
event: answer
data: "Part two"
event: followup_question
data: "Would you like to know more?"
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agents/answer" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseResponse))
	}))
	defer server.Close()

	client, err := NewAntflyClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewAntflyClient: %v", err)
	}

	var classificationReceived bool
	var hitsReceived []string
	var answerChunks []string
	var followups []string

	result, err := client.AnswerAgent(context.Background(), AnswerAgentRequest{
		Query:         "test query",
		WithStreaming: true,
	}, AnswerAgentOptions{
		OnClassification: func(result *ClassificationTransformationResult) error {
			classificationReceived = true
			if result.SemanticQuery != "transformed query" {
				t.Errorf("got semantic_query %q, want %q", result.SemanticQuery, "transformed query")
			}
			return nil
		},
		OnHit: func(hit *Hit) error {
			hitsReceived = append(hitsReceived, hit.ID)
			return nil
		},
		OnAnswer: func(chunk string) error {
			answerChunks = append(answerChunks, chunk)
			return nil
		},
		OnFollowupQuestion: func(question string) error {
			followups = append(followups, question)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("AnswerAgent: %v", err)
	}

	if !classificationReceived {
		t.Error("classification callback was not called")
	}
	if len(hitsReceived) != 2 {
		t.Errorf("got %d hits, want 2", len(hitsReceived))
	}
	if result.Answer != "Part one Part two" {
		t.Errorf("got answer %q, want %q", result.Answer, "Part one Part two")
	}
	if len(followups) != 1 || followups[0] != "Would you like to know more?" {
		t.Errorf("got followups %v, want [\"Would you like to know more?\"]", followups)
	}
	if len(result.FollowupQuestions) != 1 {
		t.Errorf("got %d followup questions in result, want 1", len(result.FollowupQuestions))
	}
}

func TestAnswerAgentNonStreaming(t *testing.T) {
	jsonResponse := `{"answer":"Complete answer","classification_transformation":{"route_type":"search","semantic_query":"test"},"followup_questions":["Q1","Q2"]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonResponse))
	}))
	defer server.Close()

	client, err := NewAntflyClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewAntflyClient: %v", err)
	}

	result, err := client.AnswerAgent(context.Background(), AnswerAgentRequest{
		Query:         "test query",
		WithStreaming: false,
	})
	if err != nil {
		t.Fatalf("AnswerAgent: %v", err)
	}

	if result.Answer != "Complete answer" {
		t.Errorf("got answer %q, want %q", result.Answer, "Complete answer")
	}
	if len(result.FollowupQuestions) != 2 {
		t.Errorf("got %d followup questions, want 2", len(result.FollowupQuestions))
	}
}

func TestAnswerAgentStreamingError(t *testing.T) {
	sseResponse := `event: error
data: {"error":"query failed","status":500,"table":"docs"}
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseResponse))
	}))
	defer server.Close()

	client, err := NewAntflyClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewAntflyClient: %v", err)
	}

	var errorReceived *AnswerAgentError
	_, err = client.AnswerAgent(context.Background(), AnswerAgentRequest{
		Query:         "test query",
		WithStreaming: true,
	}, AnswerAgentOptions{
		OnError: func(e *AnswerAgentError) error {
			errorReceived = e
			return nil
		},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errorReceived == nil {
		t.Fatal("error callback was not called")
	}
	if errorReceived.Table != "docs" {
		t.Errorf("got table %q, want %q", errorReceived.Table, "docs")
	}
	if errorReceived.Status != 500 {
		t.Errorf("got status %d, want 500", errorReceived.Status)
	}
	if !strings.Contains(err.Error(), "docs") {
		t.Errorf("error message should contain table name, got %q", err.Error())
	}
}

func TestAnswerAgentHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	client, err := NewAntflyClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewAntflyClient: %v", err)
	}

	_, err = client.AnswerAgent(context.Background(), AnswerAgentRequest{
		Query:         "test query",
		WithStreaming: true,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message should contain status code, got %q", err.Error())
	}
}
