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

	// Transform types for MongoDB-style atomic updates
	Transform       = oapi.Transform
	TransformOp     = oapi.TransformOp
	TransformOpType = oapi.TransformOpType

	// Key scan types
	ScanKeysRequest = oapi.ScanKeysRequest
	LookupKeyParams = oapi.LookupKeyParams

	// AI Agent types
	ClassificationTransformationResult = oapi.ClassificationTransformationResult
	RouteType                          = oapi.RouteType
	QueryStrategy                      = oapi.QueryStrategy
	SemanticQueryMode                  = oapi.SemanticQueryMode
	ClassificationStepConfig           = oapi.ClassificationStepConfig
	GenerationStepConfig               = oapi.GenerationStepConfig
	FollowupStepConfig                 = oapi.FollowupStepConfig
	ConfidenceStepConfig               = oapi.ConfidenceStepConfig
	RetryConfig                        = oapi.RetryConfig
	ChainLink                          = oapi.ChainLink
	ChainCondition                     = oapi.ChainCondition

	// Chat/Agent types (used by retrieval agent)
	ChatMessage          = oapi.ChatMessage
	ChatMessageRole      = oapi.ChatMessageRole
	ChatToolCall         = oapi.ChatToolCall
	ChatToolResult       = oapi.ChatToolResult
	ChatToolName         = oapi.ChatToolName
	ChatToolsConfig      = oapi.ChatToolsConfig
	ClarificationRequest = oapi.ClarificationRequest
	FilterSpec           = oapi.FilterSpec
	FilterSpecOperator   = oapi.FilterSpecOperator

	// Retrieval Agent types
	RetrievalAgentRequest   = oapi.RetrievalAgentRequest
	RetrievalAgentResult    = oapi.RetrievalAgentResult
	RetrievalAgentState     = oapi.RetrievalAgentState
	RetrievalAgentSteps     = oapi.RetrievalAgentSteps

	// Answer Agent types (deprecated, use Retrieval Agent instead)
	AnswerAgentRequest = oapi.AnswerAgentRequest
	AnswerAgentResult  = oapi.AnswerAgentResult
	AnswerAgentSteps   = oapi.AnswerAgentSteps
	RetrievalQueryRequest   = oapi.RetrievalQueryRequest
	RetrievalReasoningStep  = oapi.RetrievalReasoningStep
	RetrievalStrategy = oapi.RetrievalStrategy
	TreeSearchConfig        = oapi.TreeSearchConfig
	QueryHit                = oapi.QueryHit

	// Evaluation types
	EvalConfig    = oapi.EvalConfig
	EvalOptions   = oapi.EvalOptions
	EvalResult    = oapi.EvalResult
	EvalSummary   = oapi.EvalSummary
	EvaluatorName = oapi.EvaluatorName
	GroundTruth   = oapi.GroundTruth

	// Join types
	JoinClause    = oapi.JoinClause
	JoinCondition = oapi.JoinCondition
	JoinFilters   = oapi.JoinFilters
	JoinOperator  = oapi.JoinOperator
	JoinResult    = oapi.JoinResult
	JoinStrategy  = oapi.JoinStrategy
	JoinType      = oapi.JoinType

	// Graph index types
	GraphIndexV0Config      = oapi.GraphIndexV0Config
	GraphIndexV0Stats       = oapi.GraphIndexV0Stats
	EdgeTypeConfig          = oapi.EdgeTypeConfig
	EdgeTypeConfigTopology  = oapi.EdgeTypeConfigTopology
	EdgeDirection           = oapi.EdgeDirection
	Edge                    = oapi.Edge
	EdgesResponse           = oapi.EdgesResponse

	// Graph query types
	GraphQuery        = oapi.GraphQuery
	GraphQueryParams  = oapi.GraphQueryParams
	GraphQueryResult  = oapi.GraphQueryResult
	GraphQueryType    = oapi.GraphQueryType
	GraphNodeSelector = oapi.GraphNodeSelector
	GraphResultNode   = oapi.GraphResultNode

	// Graph pattern types
	PatternEdgeStep = oapi.PatternEdgeStep
	PatternMatch    = oapi.PatternMatch
	PatternStep     = oapi.PatternStep

	// Graph traversal types
	TraverseResponse = oapi.TraverseResponse

	// Path types
	Path                = oapi.Path
	PathEdge            = oapi.PathEdge
	PathFindRequest     = oapi.PathFindRequest
	PathFindResult      = oapi.PathFindResult
	PathFindWeightMode  = oapi.PathFindWeightMode
	PathWeightMode      = oapi.PathWeightMode
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

	// ChatMessageRole values
	ChatMessageRoleUser      = oapi.ChatMessageRoleUser
	ChatMessageRoleAssistant = oapi.ChatMessageRoleAssistant
	ChatMessageRoleSystem    = oapi.ChatMessageRoleSystem
	ChatMessageRoleTool      = oapi.ChatMessageRoleTool

	// ChatToolName values
	ChatToolNameAddFilter        = oapi.ChatToolNameAddFilter
	ChatToolNameAskClarification = oapi.ChatToolNameAskClarification
	ChatToolNameFetch            = oapi.ChatToolNameFetch
	ChatToolNameSearch           = oapi.ChatToolNameSearch
	ChatToolNameWebsearch        = oapi.ChatToolNameWebsearch

	// FilterSpecOperator values
	FilterSpecOperatorEq       = oapi.FilterSpecOperatorEq
	FilterSpecOperatorNe       = oapi.FilterSpecOperatorNe
	FilterSpecOperatorGt       = oapi.FilterSpecOperatorGt
	FilterSpecOperatorGte      = oapi.FilterSpecOperatorGte
	FilterSpecOperatorLt       = oapi.FilterSpecOperatorLt
	FilterSpecOperatorLte      = oapi.FilterSpecOperatorLte
	FilterSpecOperatorContains = oapi.FilterSpecOperatorContains
	FilterSpecOperatorPrefix   = oapi.FilterSpecOperatorPrefix
	FilterSpecOperatorRange    = oapi.FilterSpecOperatorRange
	FilterSpecOperatorIn       = oapi.FilterSpecOperatorIn

	// RetrievalAgentState values
	RetrievalAgentStateToolCalling           = oapi.RetrievalAgentStateToolCalling
	RetrievalAgentStateComplete              = oapi.RetrievalAgentStateComplete
	RetrievalAgentStateAwaitingClarification = oapi.RetrievalAgentStateAwaitingClarification

	// RetrievalStrategy values
	RetrievalStrategySemantic = oapi.RetrievalStrategySemantic
	RetrievalStrategyBm25     = oapi.RetrievalStrategyBm25
	RetrievalStrategyTree     = oapi.RetrievalStrategyTree
	RetrievalStrategyGraph    = oapi.RetrievalStrategyGraph
	RetrievalStrategyMetadata = oapi.RetrievalStrategyMetadata
	RetrievalStrategyHybrid   = oapi.RetrievalStrategyHybrid

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

	// JoinOperator values
	JoinOperatorEq  = oapi.JoinOperatorEq
	JoinOperatorNeq = oapi.JoinOperatorNeq
	JoinOperatorLt  = oapi.JoinOperatorLt
	JoinOperatorLte = oapi.JoinOperatorLte
	JoinOperatorGt  = oapi.JoinOperatorGt
	JoinOperatorGte = oapi.JoinOperatorGte

	// JoinStrategy values
	JoinStrategyBroadcast   = oapi.JoinStrategyBroadcast
	JoinStrategyIndexLookup = oapi.JoinStrategyIndexLookup
	JoinStrategyShuffle     = oapi.JoinStrategyShuffle

	// JoinType values
	JoinTypeInner = oapi.JoinTypeInner
	JoinTypeLeft  = oapi.JoinTypeLeft
	JoinTypeRight = oapi.JoinTypeRight

	// TransformOpType values for MongoDB-style operators
	TransformOpTypeSet         = oapi.TransformOpTypeSet
	TransformOpTypeUnset       = oapi.TransformOpTypeUnset
	TransformOpTypeInc         = oapi.TransformOpTypeInc
	TransformOpTypeMul         = oapi.TransformOpTypeMul
	TransformOpTypeMin         = oapi.TransformOpTypeMin
	TransformOpTypeMax         = oapi.TransformOpTypeMax
	TransformOpTypePush        = oapi.TransformOpTypePush
	TransformOpTypePull        = oapi.TransformOpTypePull
	TransformOpTypeAddToSet    = oapi.TransformOpTypeAddToSet
	TransformOpTypePop         = oapi.TransformOpTypePop
	TransformOpTypeRename      = oapi.TransformOpTypeRename
	TransformOpTypeCurrentDate = oapi.TransformOpTypeCurrentDate

	// IndexType graph value
	IndexTypeGraphV0 = oapi.IndexTypeGraphV0

	// EdgeDirection values
	EdgeDirectionBoth = oapi.EdgeDirectionBoth
	EdgeDirectionIn   = oapi.EdgeDirectionIn
	EdgeDirectionOut  = oapi.EdgeDirectionOut

	// EdgeTypeConfigTopology values
	EdgeTypeConfigTopologyGraph = oapi.EdgeTypeConfigTopologyGraph
	EdgeTypeConfigTopologyTree  = oapi.EdgeTypeConfigTopologyTree

	// GraphQueryType values
	GraphQueryTypeKShortestPaths = oapi.GraphQueryTypeKShortestPaths
	GraphQueryTypeNeighbors      = oapi.GraphQueryTypeNeighbors
	GraphQueryTypePattern        = oapi.GraphQueryTypePattern
	GraphQueryTypeShortestPath   = oapi.GraphQueryTypeShortestPath
	GraphQueryTypeTraverse       = oapi.GraphQueryTypeTraverse

	// PathFindWeightMode values
	PathFindWeightModeMaxWeight = oapi.PathFindWeightModeMaxWeight
	PathFindWeightModeMinHops   = oapi.PathFindWeightModeMinHops
	PathFindWeightModeMinWeight = oapi.PathFindWeightModeMinWeight

	// PathWeightMode values
	PathWeightModeMaxWeight = oapi.PathWeightModeMaxWeight
	PathWeightModeMinHops   = oapi.PathWeightModeMinHops
	PathWeightModeMinWeight = oapi.PathWeightModeMinWeight
)
