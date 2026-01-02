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

import "github.com/antflydb/antfly-go/antfly/oapi"

// Re-export commonly used types from oapi package
type (
	// Table and Index types
	CreateTableRequest = oapi.CreateTableRequest
	TableStatus        = oapi.TableStatus
	TableSchema        = oapi.TableSchema
	IndexConfig        = oapi.IndexConfig
	IndexStatus        = oapi.IndexStatus
	IndexType          = oapi.IndexType

	// Index config types
	EmbeddingIndexConfig = oapi.EmbeddingIndexConfig
	BleveIndexV2Config   = oapi.BleveIndexV2Config
	BleveIndexV2Stats    = oapi.BleveIndexV2Stats

	EmbedderProvider         = oapi.EmbedderProvider
	GeneratorProvider        = oapi.GeneratorProvider
	EmbedderConfig           = oapi.EmbedderConfig
	GeneratorConfig          = oapi.GeneratorConfig
	OllamaEmbedderConfig     = oapi.OllamaEmbedderConfig
	OpenAIEmbedderConfig     = oapi.OpenAIEmbedderConfig
	GoogleEmbedderConfig     = oapi.GoogleEmbedderConfig
	BedrockEmbedderConfig    = oapi.BedrockEmbedderConfig
	VertexEmbedderConfig     = oapi.VertexEmbedderConfig
	TermiteEmbedderConfig    = oapi.TermiteEmbedderConfig
	OllamaGeneratorConfig    = oapi.OllamaGeneratorConfig
	OpenAIGeneratorConfig    = oapi.OpenAIGeneratorConfig
	GoogleGeneratorConfig    = oapi.GoogleGeneratorConfig
	BedrockGeneratorConfig   = oapi.BedrockGeneratorConfig
	VertexGeneratorConfig    = oapi.VertexGeneratorConfig
	AnthropicGeneratorConfig = oapi.AnthropicGeneratorConfig
	TermiteGeneratorConfig   = oapi.TermiteGeneratorConfig
	RerankerConfig           = oapi.RerankerConfig
	OllamaRerankerConfig     = oapi.OllamaRerankerConfig
	TermiteRerankerConfig    = oapi.TermiteRerankerConfig
	RerankerProvider         = oapi.RerankerProvider
	Pruner                   = oapi.Pruner

	// Chunker config types
	ChunkerProvider      = oapi.ChunkerProvider
	ChunkerConfig        = oapi.ChunkerConfig
	TermiteChunkerConfig = oapi.TermiteChunkerConfig
	AntflyChunkerConfig  = oapi.AntflyChunkerConfig

	// Query response types
	QueryResponses     = oapi.QueryResponses
	QueryResult        = oapi.QueryResult
	Hits               = oapi.QueryHits
	Hit                = oapi.QueryHit
	AggregationRequest = oapi.AggregationRequest
	AggregationOption  = oapi.AggregationBucket
	AggregationResult  = oapi.AggregationResult

	// RAG response types
	RAGResult      = oapi.RAGResult
	GenerateResult = oapi.GenerateResult

	// Other types
	AntflyType     = oapi.AntflyType
	MergeStrategy  = oapi.MergeStrategy
	DocumentSchema = oapi.DocumentSchema

	// Validation types
	ValidationError  = oapi.ValidationError
	ValidationResult = oapi.ValidationResult

	// LinearMerge types
	LinearMergePageStatus = oapi.LinearMergePageStatus
	LinearMergeRequest    = oapi.LinearMergeRequest
	LinearMergeResult     = oapi.LinearMergeResult
	FailedOperation       = oapi.FailedOperation
	KeyRange              = oapi.KeyRange
	SyncLevel             = oapi.SyncLevel

	// Key scan types
	ScanKeysRequest = oapi.ScanKeysRequest
	LookupKeyParams = oapi.LookupKeyParams

	// AI Agent types
	AnswerAgentResult                  = oapi.AnswerAgentResult
	ClassificationTransformationResult = oapi.ClassificationTransformationResult
	RouteType                          = oapi.RouteType
	QueryStrategy                      = oapi.QueryStrategy
	SemanticQueryMode                  = oapi.SemanticQueryMode
	AnswerAgentSteps                   = oapi.AnswerAgentSteps
	ClassificationStepConfig           = oapi.ClassificationStepConfig
	AnswerStepConfig                   = oapi.AnswerStepConfig
	FollowupStepConfig                 = oapi.FollowupStepConfig
	ConfidenceStepConfig               = oapi.ConfidenceStepConfig
	RetryConfig                        = oapi.RetryConfig
	ChainLink                          = oapi.ChainLink
	ChainCondition                     = oapi.ChainCondition

	// Evaluation types
	EvalConfig    = oapi.EvalConfig
	EvalOptions   = oapi.EvalOptions
	EvalResult    = oapi.EvalResult
	EvalSummary   = oapi.EvalSummary
	EvaluatorName = oapi.EvaluatorName
	GroundTruth   = oapi.GroundTruth
)

// ChunkingModel is just a string - use "fixed" or any ONNX model directory name
// No predefined constants needed since any model name is valid

// Constants from oapi
const (
	// IndexType values
	IndexTypeFullTextV0 = oapi.IndexTypeFullTextV0
	IndexTypeAknnV0     = oapi.IndexTypeAknnV0

	// Provider values
	EmbedderProviderOllama     = oapi.EmbedderProviderOllama
	EmbedderProviderOpenai     = oapi.EmbedderProviderOpenai
	EmbedderProviderGemini     = oapi.EmbedderProviderGemini
	EmbedderProviderBedrock    = oapi.EmbedderProviderBedrock
	EmbedderProviderVertex     = oapi.EmbedderProviderVertex
	EmbedderProviderTermite    = oapi.EmbedderProviderTermite
	EmbedderProviderMock       = oapi.EmbedderProviderMock
	GeneratorProviderOllama    = oapi.GeneratorProviderOllama
	GeneratorProviderOpenai    = oapi.GeneratorProviderOpenai
	GeneratorProviderGemini    = oapi.GeneratorProviderGemini
	GeneratorProviderBedrock   = oapi.GeneratorProviderBedrock
	GeneratorProviderVertex    = oapi.GeneratorProviderVertex
	GeneratorProviderAnthropic = oapi.GeneratorProviderAnthropic
	GeneratorProviderTermite   = oapi.GeneratorProviderTermite
	GeneratorProviderMock      = oapi.GeneratorProviderMock
	RerankerProviderOllama     = oapi.RerankerProviderOllama
	RerankerProviderTermite    = oapi.RerankerProviderTermite

	// MergeStrategy values
	MergeStrategyRrf      = oapi.MergeStrategyRrf
	MergeStrategyFailover = oapi.MergeStrategyFailover

	// LinearMergePageStatus values
	LinearMergePageStatusSuccess = oapi.LinearMergePageStatusSuccess
	LinearMergePageStatusPartial = oapi.LinearMergePageStatusPartial
	LinearMergePageStatusError   = oapi.LinearMergePageStatusError

	// SyncLevel values
	SyncLevelPropose  = oapi.SyncLevelPropose
	SyncLevelWrite    = oapi.SyncLevelWrite
	SyncLevelFullText = oapi.SyncLevelFullText
	SyncLevelAknn     = oapi.SyncLevelAknn

	// RouteType values
	RouteTypeQuestion = oapi.RouteTypeQuestion
	RouteTypeSearch   = oapi.RouteTypeSearch

	// QueryStrategy values
	QueryStrategySimple    = oapi.QueryStrategySimple
	QueryStrategyDecompose = oapi.QueryStrategyDecompose
	QueryStrategyStepBack  = oapi.QueryStrategyStepBack
	QueryStrategyHyde      = oapi.QueryStrategyHyde

	// SemanticQueryMode values
	SemanticQueryModeRewrite      = oapi.SemanticQueryModeRewrite
	SemanticQueryModeHypothetical = oapi.SemanticQueryModeHypothetical

	// ChainCondition values
	ChainConditionAlways      = oapi.ChainConditionAlways
	ChainConditionOnError     = oapi.ChainConditionOnError
	ChainConditionOnTimeout   = oapi.ChainConditionOnTimeout
	ChainConditionOnRateLimit = oapi.ChainConditionOnRateLimit

	// EvaluatorName values
	EvaluatorNameCitationQuality = oapi.EvaluatorNameCitationQuality
	EvaluatorNameCoherence       = oapi.EvaluatorNameCoherence
	EvaluatorNameCompleteness    = oapi.EvaluatorNameCompleteness
	EvaluatorNameCorrectness     = oapi.EvaluatorNameCorrectness
	EvaluatorNameFaithfulness    = oapi.EvaluatorNameFaithfulness
	EvaluatorNameHelpfulness     = oapi.EvaluatorNameHelpfulness
	EvaluatorNameMap             = oapi.EvaluatorNameMap
	EvaluatorNameMrr             = oapi.EvaluatorNameMrr
	EvaluatorNameNdcg            = oapi.EvaluatorNameNdcg
	EvaluatorNamePrecision       = oapi.EvaluatorNamePrecision
	EvaluatorNameRecall          = oapi.EvaluatorNameRecall
	EvaluatorNameRelevance       = oapi.EvaluatorNameRelevance
	EvaluatorNameSafety          = oapi.EvaluatorNameSafety
)
