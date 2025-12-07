package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	antflyevalaf "github.com/antflydb/antfly-go/antfly-evalaf"
	"github.com/antflydb/antfly-go/evalaf/eval"
)

func main() {
	ctx := context.Background()

	// Example 1: Evaluate RAG with quick preset (no LLM calls)
	evaluateRAGQuick(ctx)

	fmt.Println("\n" + strings.Repeat("=", 80) + "\n")

	// Example 2: Evaluate Answer Agent classification
	evaluateAnswerAgentClassification(ctx)
}

func evaluateRAGQuick(ctx context.Context) {
	fmt.Println("Example 1: Quick RAG Evaluation (no LLM calls)")
	fmt.Println(strings.Repeat("=", 50))

	// Create Antfly client (for reference)
	_, _ = antflyevalaf.NewClient("http://localhost:3210")

	// Create dataset
	dataset := eval.NewJSONDatasetFromExamples("rag_test", []eval.Example{
		{
			Input: "What is machine learning?",
			Context: []any{
				"Machine learning is a subset of artificial intelligence.",
				"It involves training models on data to make predictions.",
			},
		},
		{
			Input: "What is deep learning?",
			Context: []any{
				"Deep learning is a type of machine learning.",
				"It uses neural networks with many layers.",
			},
		},
	})

	// Use quick preset (citations only, very fast)
	evaluators := antflyevalaf.QuickEvaluatorPreset()

	// Create target function
	// Note: This will fail if Antfly isn't running, but shows the pattern
	target := func(ctx context.Context, example eval.Example) (any, error) {
		// In real usage, this would call:
		// return client.CreateRAGTargetFunc([]string{"my_table"})(ctx, example)

		// For demo purposes, simulate a response with citations
		query := example.Input.(string)
		if query == "What is machine learning?" {
			return "Machine learning is a subset of AI [doc_id 0]. It trains models on data [doc_id 1].", nil
		}
		return "Deep learning uses neural networks [doc_id 1].", nil
	}

	// Run evaluation
	config := *eval.DefaultConfig()
	config.Execution.Parallel = false // Sequential for demo

	runner := eval.NewRunner(config, evaluators)
	report, err := runner.RunWithTarget(ctx, dataset, target)
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	// Print results
	report.Print()
}

func evaluateAnswerAgentClassification(ctx context.Context) {
	fmt.Println("Example 2: Answer Agent Classification Evaluation")
	fmt.Println(strings.Repeat("=", 50))

	// Create dataset with expected classifications
	dataset := eval.NewJSONDatasetFromExamples("agent_test", []eval.Example{
		{
			Input:     "What is the weather today?",
			Reference: "question",
		},
		{
			Input:     "papers about transformers",
			Reference: "search",
		},
		{
			Input:     "How does photosynthesis work?",
			Reference: "question",
		},
		{
			Input:     "latest research on quantum computing",
			Reference: "search",
		},
	})

	// Use Answer Agent preset (classification + confidence)
	evaluators := antflyevalaf.AnswerAgentEvaluatorPreset(nil, "") // nil = no LLM-as-judge

	// Create target function
	target := func(ctx context.Context, example eval.Example) (any, error) {
		// In real usage, this would call:
		// client := antfly.NewClient("http://localhost:3210")
		// return client.CreateAnswerAgentTargetFunc([]string{"my_table"})(ctx, example)

		// For demo purposes, simulate Answer Agent response
		query := example.Input.(string)

		// Simple heuristic: questions have "?" or start with question words
		isQuestion := false
		if len(query) > 0 && query[len(query)-1] == '?' {
			isQuestion = true
		}
		questionWords := []string{"what", "how", "why", "when", "where", "who"}
		for _, word := range questionWords {
			if len(query) >= len(word) && query[:len(word)] == word {
				isQuestion = true
				break
			}
		}

		routeType := "search"
		confidence := 0.75
		if isQuestion {
			routeType = "question"
			confidence = 0.85
		}

		return map[string]any{
			"route_type": routeType,
			"confidence": confidence,
			"answer":     "This is a simulated answer.",
		}, nil
	}

	// Run evaluation
	config := *eval.DefaultConfig()
	config.Execution.Parallel = false

	runner := eval.NewRunner(config, evaluators)
	report, err := runner.RunWithTarget(ctx, dataset, target)
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	// Print results
	report.Print()

	// Check classification accuracy
	if stats, ok := report.Summary.EvaluatorStats["classification"]; ok {
		fmt.Printf("\n📊 Classification Accuracy: %.1f%%\n", stats.PassRate*100)
	}
}
