package antflyevalaf

import (
	"github.com/antflydb/antfly-go/evalaf/agent"
)

// NewAnswerAgentClassificationEvaluator creates a classification evaluator for Antfly's Answer Agent.
// The Answer Agent classifies queries as "question" or "search".
func NewAnswerAgentClassificationEvaluator(name string) *agent.ClassificationEvaluator {
	if name == "" {
		name = "answer_agent_classification"
	}
	return agent.NewClassificationEvaluator(name, []string{"question", "search"})
}

// NewAnswerAgentConfidenceEvaluator creates a confidence evaluator for Answer Agent.
// Default threshold is 0.7 (70% confidence).
func NewAnswerAgentConfidenceEvaluator(name string, minConfidence float64) *agent.ConfidenceEvaluator {
	if name == "" {
		name = "answer_agent_confidence"
	}
	if minConfidence <= 0 {
		minConfidence = 0.7
	}
	return agent.NewConfidenceEvaluator(name, minConfidence)
}

// AnswerAgentResponse represents the structured response from Antfly's Answer Agent.
type AnswerAgentResponse struct {
	RouteType         string   `json:"route_type"`                    // "question" or "search"
	ImprovedQuery     string   `json:"improved_query"`                // Improved version of query
	SemanticQuery     string   `json:"semantic_query"`                // Query optimized for semantic search
	Confidence        float64  `json:"confidence"`                    // Confidence score (0-1)
	Answer            string   `json:"answer,omitempty"`              // Generated answer (for questions)
	Reasoning         string   `json:"reasoning,omitempty"`           // Reasoning (if enabled)
	FollowUpQuestions []string `json:"follow_up_questions,omitempty"` // Follow-up questions
}
