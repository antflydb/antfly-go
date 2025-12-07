package antflyevalaf

import (
	"context"
	"fmt"
	"net/http"

	antfly "github.com/antflydb/antfly-go/antfly"
	"github.com/antflydb/antfly-go/evalaf/eval"
	"github.com/bytedance/sonic"
)

// Type aliases for the generated types from the Antfly SDK.
// These provide access to query hits with full score information.
type (
	RAGRequest   = antfly.RAGRequest
	RAGResult    = antfly.RAGResult
	QueryResult  = antfly.QueryResult
	QueryHit     = antfly.Hit
	QueryHits    = antfly.Hits
)

// Client wraps the Antfly SDK client for evalaf integration.
type Client struct {
	*antfly.AntflyClient
}

// NewClient creates a new Antfly client for evalaf.
func NewClient(baseURL string) (*Client, error) {
	if baseURL == "" {
		baseURL = "http://localhost:3210"
	}
	sdkClient, err := antfly.NewAntflyClient(baseURL, http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("creating antfly client: %w", err)
	}
	return &Client{AntflyClient: sdkClient}, nil
}

// CallRAG calls the Antfly RAG endpoint and returns the full result including query hits with scores.
func (c *Client) CallRAG(ctx context.Context, req RAGRequest) (*RAGResult, error) {
	// Call the SDK's RAG method which returns raw JSON
	rawJSON, err := c.AntflyClient.RAG(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the JSON into RAGResult to get full hit information
	var result RAGResult
	if err := sonic.UnmarshalString(rawJSON, &result); err != nil {
		return nil, fmt.Errorf("parsing RAG response: %w", err)
	}

	return &result, nil
}

// CallAnswerAgent calls the Antfly Answer Agent endpoint.
func (c *Client) CallAnswerAgent(ctx context.Context, req antfly.AnswerAgentRequest) (*antfly.AnswerAgentResult, error) {
	return c.AntflyClient.AnswerAgent(ctx, req)
}

// CreateRAGTargetFunc creates a target function for evaluating Antfly's RAG endpoint.
func (c *Client) CreateRAGTargetFunc(tables []string) eval.TargetFunc {
	return func(ctx context.Context, example eval.Example) (any, error) {
		query, ok := example.Input.(string)
		if !ok {
			return nil, fmt.Errorf("input must be a string")
		}

		resp, err := c.CallRAG(ctx, RAGRequest{
			Queries: []antfly.QueryRequest{{
				Table:          tables[0],
				SemanticSearch: query,
			}},
		})
		if err != nil {
			return nil, err
		}

		return resp.SummaryResult.Summary, nil
	}
}

// CreateAnswerAgentTargetFunc creates a target function for evaluating Answer Agent.
func (c *Client) CreateAnswerAgentTargetFunc(tables []string) eval.TargetFunc {
	return func(ctx context.Context, example eval.Example) (any, error) {
		query, ok := example.Input.(string)
		if !ok {
			return nil, fmt.Errorf("input must be a string")
		}

		resp, err := c.CallAnswerAgent(ctx, antfly.AnswerAgentRequest{
			Query: query,
			Queries: []antfly.QueryRequest{{
				Table: tables[0],
			}},
		})
		if err != nil {
			return nil, err
		}

		// Return structured response for classification evaluators
		return map[string]any{
			"route_type": resp.ClassificationTransformation.RouteType,
			"confidence": resp.ClassificationTransformation.Confidence,
			"answer":     resp.Answer,
		}, nil
	}
}

// CreateAnswerAgentAnswerTargetFunc creates a target function that returns just the answer.
func (c *Client) CreateAnswerAgentAnswerTargetFunc(tables []string) eval.TargetFunc {
	return func(ctx context.Context, example eval.Example) (any, error) {
		query, ok := example.Input.(string)
		if !ok {
			return nil, fmt.Errorf("input must be a string")
		}

		resp, err := c.CallAnswerAgent(ctx, antfly.AnswerAgentRequest{
			Query: query,
			Queries: []antfly.QueryRequest{{
				Table: tables[0],
			}},
		})
		if err != nil {
			return nil, err
		}

		return resp.Answer, nil
	}
}
