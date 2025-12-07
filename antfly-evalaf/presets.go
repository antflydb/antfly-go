package antflyevalaf

import (
	"github.com/antflydb/antfly-go/evalaf/eval"
	"github.com/antflydb/antfly-go/evalaf/genkit"
	"github.com/antflydb/antfly-go/evalaf/rag"
	genkitpkg "github.com/firebase/genkit/go/genkit"
)

// RAGEvaluatorPreset returns a preset of evaluators for RAG evaluation.
func RAGEvaluatorPreset(g *genkitpkg.Genkit, modelName string) []eval.Evaluator {
	if modelName == "" {
		modelName = "ollama/mistral"
	}

	// Use custom citation pattern to match LLM output format [docN] instead of [doc_id N]
	citationEval, _ := rag.NewCitationEvaluatorWithPattern("citations", `\[doc(\d+)\]`)
	coverageEval, _ := rag.NewCitationCoverageEvaluatorWithPattern("coverage", `\[doc(\d+)\]`)

	evaluators := []eval.Evaluator{
		// Citation quality
		citationEval,
		coverageEval,
	}

	// Add LLM-as-judge if Genkit is provided
	if g != nil {
		faithfulness, _ := genkit.NewFaithfulnessEvaluator(g, modelName)
		relevance, _ := genkit.NewRelevanceEvaluator(g, modelName)
		completeness, _ := genkit.NewCompletenessEvaluator(g, modelName)

		if faithfulness != nil {
			evaluators = append(evaluators, faithfulness)
		}
		if relevance != nil {
			evaluators = append(evaluators, relevance)
		}
		if completeness != nil {
			evaluators = append(evaluators, completeness)
		}
	}

	return evaluators
}

// AnswerAgentEvaluatorPreset returns a preset of evaluators for Answer Agent evaluation.
func AnswerAgentEvaluatorPreset(g *genkitpkg.Genkit, modelName string) []eval.Evaluator {
	if modelName == "" {
		modelName = "ollama/mistral"
	}

	evaluators := []eval.Evaluator{
		// Classification and confidence
		NewAnswerAgentClassificationEvaluator("classification"),
		NewAnswerAgentConfidenceEvaluator("confidence", 0.7),
	}

	// Add LLM-as-judge if Genkit is provided
	if g != nil {
		relevance, _ := genkit.NewRelevanceEvaluator(g, modelName)
		coherence, _ := genkit.NewCoherenceEvaluator(g, modelName)
		helpfulness, _ := genkit.NewHelpfulnessEvaluator(g, modelName)

		if relevance != nil {
			evaluators = append(evaluators, relevance)
		}
		if coherence != nil {
			evaluators = append(evaluators, coherence)
		}
		if helpfulness != nil {
			evaluators = append(evaluators, helpfulness)
		}
	}

	return evaluators
}

// ComprehensiveEvaluatorPreset returns a comprehensive set of evaluators for full evaluation.
func ComprehensiveEvaluatorPreset(g *genkitpkg.Genkit, modelName string) []eval.Evaluator {
	if modelName == "" {
		modelName = "ollama/mistral"
	}

	// Use custom citation pattern to match LLM output format [docN] instead of [doc_id N]
	citationEval, _ := rag.NewCitationEvaluatorWithPattern("citations", `\[doc(\d+)\]`)
	coverageEval, _ := rag.NewCitationCoverageEvaluatorWithPattern("coverage", `\[doc(\d+)\]`)

	evaluators := []eval.Evaluator{
		// Citations
		citationEval,
		coverageEval,

		// Classification
		NewAnswerAgentClassificationEvaluator("classification"),
		NewAnswerAgentConfidenceEvaluator("confidence", 0.7),
	}

	// Add LLM-as-judge if Genkit is provided
	if g != nil {
		faithfulness, _ := genkit.NewFaithfulnessEvaluator(g, modelName)
		relevance, _ := genkit.NewRelevanceEvaluator(g, modelName)
		completeness, _ := genkit.NewCompletenessEvaluator(g, modelName)
		coherence, _ := genkit.NewCoherenceEvaluator(g, modelName)
		helpfulness, _ := genkit.NewHelpfulnessEvaluator(g, modelName)
		safety, _ := genkit.NewSafetyEvaluator(g, modelName)

		if faithfulness != nil {
			evaluators = append(evaluators, faithfulness)
		}
		if relevance != nil {
			evaluators = append(evaluators, relevance)
		}
		if completeness != nil {
			evaluators = append(evaluators, completeness)
		}
		if coherence != nil {
			evaluators = append(evaluators, coherence)
		}
		if helpfulness != nil {
			evaluators = append(evaluators, helpfulness)
		}
		if safety != nil {
			evaluators = append(evaluators, safety)
		}
	}

	return evaluators
}

// QuickEvaluatorPreset returns a fast preset without LLM-as-judge.
// Useful for CI/CD or rapid iteration.
func QuickEvaluatorPreset() []eval.Evaluator {
	// Use custom citation pattern to match LLM output format [docN] instead of [doc_id N]
	citationEval, _ := rag.NewCitationEvaluatorWithPattern("citations", `\[doc(\d+)\]`)

	return []eval.Evaluator{
		citationEval,
		NewAnswerAgentClassificationEvaluator("classification"),
		NewAnswerAgentConfidenceEvaluator("confidence", 0.7),
	}
}
