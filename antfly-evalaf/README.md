# antfly - Antfly-Specific Evaluators and Utilities

The `antfly` package provides ready-to-use evaluators, client helpers, and presets specifically designed for evaluating Antfly's RAG and Answer Agent systems.

## Features

- Pre-configured evaluators for Antfly's citation format
- Answer Agent evaluators (classification, confidence)
- Client helpers for calling Antfly endpoints
- Preset evaluator combinations for common use cases
- Target functions for easy integration

## Quick Start

### Evaluating Antfly RAG

```go
package main

import (
    "context"
    "github.com/antflydb/antfly-go/evalaf/antfly"
    "github.com/antflydb/antfly-go/evalaf/eval"
)

func main() {
    ctx := context.Background()

    // Create Antfly client
    client := antfly.NewClient("http://localhost:3210")

    // Create dataset
    dataset := eval.NewJSONDatasetFromExamples("rag_test", []eval.Example{
        {
            Input: "What is machine learning?",
            Context: []any{
                "Machine learning is a subset of AI",
                "It involves training models on data",
            },
        },
    })

    // Use RAG preset evaluators
    evaluators := antfly.RAGEvaluatorPreset(nil, "") // nil = no LLM-as-judge

    // Create target function that calls Antfly RAG
    target := client.CreateRAGTargetFunc([]string{"my_table"})

    // Run evaluation
    runner := eval.NewRunner(eval.DefaultConfig(), evaluators)
    report, _ := runner.RunWithTarget(ctx, dataset, target)

    report.Print()
}
```

### Evaluating Answer Agent

```go
package main

import (
    "context"
    "github.com/antflydb/antfly-go/evalaf/antfly"
    "github.com/antflydb/antfly-go/evalaf/eval"
)

func main() {
    ctx := context.Background()

    // Create client
    client := antfly.NewClient("http://localhost:3210")

    // Create dataset with expected classifications
    dataset := eval.NewJSONDatasetFromExamples("agent_test", []eval.Example{
        {
            Input:     "What is the weather?",
            Reference: "question", // Expected classification
        },
        {
            Input:     "papers about AI",
            Reference: "search",
        },
    })

    // Use Answer Agent preset
    evaluators := antfly.AnswerAgentEvaluatorPreset(nil, "")

    // Create target function
    target := client.CreateAnswerAgentTargetFunc([]string{"my_table"})

    // Run evaluation
    runner := eval.NewRunner(eval.DefaultConfig(), evaluators)
    report, _ := runner.RunWithTarget(ctx, dataset, target)

    report.Print()
}
```

## Client Helpers

### NewClient

Creates a client for calling Antfly endpoints:

```go
client := antfly.NewClient("http://localhost:3210")
```

### CallRAG

Call the RAG endpoint:

```go
resp, err := client.CallRAG(ctx, antfly.RAGRequest{
    Query:  "What is machine learning?",
    Tables: []string{"docs"},
})
fmt.Println(resp.Answer)
```

### CallAnswerAgent

Call the Answer Agent endpoint:

```go
resp, err := client.CallAnswerAgent(ctx, antfly.AnswerAgentRequest{
    Query:  "What is the weather?",
    Tables: []string{"weather_data"},
})
fmt.Printf("Classification: %s (%.2f confidence)\n", resp.RouteType, resp.Confidence)
fmt.Println(resp.Answer)
```

### Target Functions

Pre-built target functions for evaluation:

```go
// For RAG evaluation (returns answer string)
ragTarget := client.CreateRAGTargetFunc([]string{"my_table"})

// For Answer Agent evaluation (returns structured response)
agentTarget := client.CreateAnswerAgentTargetFunc([]string{"my_table"})

// For Answer Agent answer evaluation (returns answer string only)
answerTarget := client.CreateAnswerAgentAnswerTargetFunc([]string{"my_table"})
```

## Evaluators

### Citation Evaluators

Pre-configured for Antfly's `[doc_id X]` format:

```go
citations := antfly.NewCitationEvaluator("citations")
coverage := antfly.NewCitationCoverageEvaluator("coverage")
```

### Answer Agent Evaluators

Configured for Answer Agent's response format:

```go
classification := antfly.NewAnswerAgentClassificationEvaluator("classification")
confidence := antfly.NewAnswerAgentConfidenceEvaluator("confidence", 0.7)
```

## Evaluator Presets

### RAGEvaluatorPreset

Comprehensive RAG evaluation:

```go
import (
    genkitpkg "github.com/firebase/genkit/go/genkit"
    "github.com/firebase/genkit/go/plugins/ollama"
)

// Initialize Genkit (optional, for LLM-as-judge)
g, _ := genkitpkg.Init(ctx,
    genkitpkg.WithPlugins(&ollama.Ollama{}),
)

evaluators := antfly.RAGEvaluatorPreset(g, "ollama/mistral")
// Returns: citations, coverage, faithfulness, relevance, completeness
```

Without LLM-as-judge:

```go
evaluators := antfly.RAGEvaluatorPreset(nil, "")
// Returns: citations, coverage (fast evaluation)
```

### AnswerAgentEvaluatorPreset

Answer Agent evaluation:

```go
evaluators := antfly.AnswerAgentEvaluatorPreset(g, "ollama/mistral")
// Returns: classification, confidence, relevance, coherence, helpfulness
```

### ComprehensiveEvaluatorPreset

Full evaluation suite:

```go
evaluators := antfly.ComprehensiveEvaluatorPreset(g, "ollama/mistral")
// Returns: citations, coverage, classification, confidence,
//          faithfulness, relevance, completeness, coherence,
//          helpfulness, safety
```

### QuickEvaluatorPreset

Fast evaluation without LLM-as-judge (for CI/CD):

```go
evaluators := antfly.QuickEvaluatorPreset()
// Returns: citations, classification, confidence
// No LLM calls, very fast
```

## Complete Example

### Comprehensive Antfly Evaluation

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/antflydb/antfly-go/evalaf/antfly"
    "github.com/antflydb/antfly-go/evalaf/eval"
    genkitpkg "github.com/firebase/genkit/go/genkit"
    "github.com/firebase/genkit/go/plugins/ollama"
)

func main() {
    ctx := context.Background()

    // Initialize Genkit for LLM-as-judge
    g, err := genkitpkg.Init(ctx,
        genkitpkg.WithPlugins(&ollama.Ollama{}),
    )
    if err != nil {
        log.Fatalf("Failed to init Genkit: %v", err)
    }

    // Create Antfly client
    client := antfly.NewClient("http://localhost:3210")

    // Load dataset
    dataset, err := eval.NewJSONDataset("antfly_eval", "testdata/antfly_rag.json")
    if err != nil {
        log.Fatalf("Failed to load dataset: %v", err)
    }

    // Use comprehensive evaluators
    evaluators := antfly.ComprehensiveEvaluatorPreset(g, "ollama/mistral")

    // Create target function
    target := client.CreateRAGTargetFunc([]string{"my_docs"})

    // Configure evaluation
    config := *eval.DefaultConfig()
    config.Execution.Parallel = true
    config.Execution.MaxConcurrency = 5

    // Run evaluation
    runner := eval.NewRunner(config, evaluators)
    report, err := runner.RunWithTarget(ctx, dataset, target)
    if err != nil {
        log.Fatalf("Evaluation failed: %v", err)
    }

    // Print results
    report.Print()

    // Save results
    if err := report.SaveToFile("antfly_eval_results.json", "json", true); err != nil {
        log.Printf("Failed to save results: %v", err)
    }

    // Check if pass rate is acceptable
    if report.Summary.PassRate < 0.8 {
        fmt.Printf("⚠️  Warning: Pass rate %.1f%% is below 80%%\n", report.Summary.PassRate*100)
    } else {
        fmt.Printf("✅ Pass rate %.1f%% meets quality bar\n", report.Summary.PassRate*100)
    }
}
```

### CI/CD Integration

```go
// Fast evaluation for CI/CD
func runCIEvaluation() error {
    ctx := context.Background()

    client := antfly.NewClient("http://localhost:3210")
    dataset, _ := eval.NewJSONDataset("ci", "testdata/ci_dataset.json")

    // Use quick preset (no LLM calls)
    evaluators := antfly.QuickEvaluatorPreset()

    target := client.CreateRAGTargetFunc([]string{"docs"})

    runner := eval.NewRunner(eval.DefaultConfig(), evaluators)
    report, err := runner.RunWithTarget(ctx, dataset, target)
    if err != nil {
        return err
    }

    // Fail CI if pass rate is too low
    if report.Summary.PassRate < 0.95 {
        return fmt.Errorf("evaluation failed: pass rate %.1f%% < 95%%", report.Summary.PassRate*100)
    }

    return nil
}
```

## Answer Agent Response Format

The Answer Agent returns structured responses:

```go
type AnswerAgentResponse struct {
    RouteType       string   // "question" or "search"
    ImprovedQuery   string   // Improved version of query
    SemanticQuery   string   // Query optimized for semantic search
    Confidence      float64  // Confidence score (0-1)
    Answer          string   // Generated answer (for questions)
    Reasoning       string   // Reasoning (if enabled)
    FollowUpQuestions []string // Follow-up questions
}
```

## Dataset Format

### RAG Evaluation Dataset

```json
[
  {
    "input": "What is machine learning?",
    "context": [
      "Machine learning is a subset of AI",
      "It involves training models on data"
    ],
    "metadata": {
      "domain": "ai"
    }
  }
]
```

### Answer Agent Classification Dataset

```json
[
  {
    "input": "What is the weather?",
    "reference": "question",
    "metadata": {
      "expected_confidence": 0.9
    }
  },
  {
    "input": "papers about transformers",
    "reference": "search",
    "metadata": {
      "expected_confidence": 0.85
    }
  }
]
```

## Integration with searchaf

Since `antfly` is a generic package, it can be used in searchaf:

```go
// In searchaf
import "github.com/antflydb/anteval/antfly"

func evaluateCustomerPrompt(customerID string) (*eval.Report, error) {
    // Create client pointing to customer's Antfly instance
    client := antfly.NewClient(getCustomerAntflyURL(customerID))

    // Load customer-specific dataset
    dataset := loadCustomerDataset(customerID)

    // Use appropriate preset
    evaluators := antfly.RAGEvaluatorPreset(g, "ollama/mistral")

    // Evaluate
    target := client.CreateRAGTargetFunc(getCustomerTables(customerID))
    runner := eval.NewRunner(eval.DefaultConfig(), evaluators)

    return runner.RunWithTarget(ctx, dataset, target)
}
```

## Best Practices

1. **Development**: Use `ComprehensiveEvaluatorPreset` with local Ollama
2. **CI/CD**: Use `QuickEvaluatorPreset` for fast feedback
3. **Production Monitoring**: Use `RAGEvaluatorPreset` or `AnswerAgentEvaluatorPreset`
4. **A/B Testing**: Use presets to compare prompt variations
5. **Custom Needs**: Mix preset evaluators with custom ones

## Performance

- **QuickEvaluatorPreset**: ~10ms per example (no LLM calls)
- **RAGEvaluatorPreset** (without LLM): ~50ms per example
- **RAGEvaluatorPreset** (with LLM): ~2-5s per example (depends on model)
- **ComprehensiveEvaluatorPreset**: ~5-10s per example (6 LLM calls)

## See Also

- [eval package](../eval/README.md) - Core evaluation library
- [rag package](../rag/README.md) - RAG evaluators
- [agent package](../agent/README.md) - Agent evaluators
- [genkit package](../genkit/README.md) - LLM-as-judge
- [Antfly Documentation](../../docs/) - Antfly API documentation
