package antfly

import (
	"context"
	"fmt"
	"strings"

	"github.com/antflydb/antfly-go/antfly"
	"github.com/antflydb/antfly-go/antfly/query"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// QueryPlan represents the LLM's structured output for query generation
type QueryPlan struct {
	// TableQueries contains one entry per table to query
	TableQueries []TableQueryPlan `json:"table_queries"`
}

// TableQueryPlan represents the query plan for a single table
type TableQueryPlan struct {
	// TableName is the name of the table to query
	TableName string `json:"table_name"`

	// SemanticSearchText is the text to use for semantic/vector search (if applicable)
	SemanticSearchText string `json:"semantic_search_text,omitempty"`

	// FullTextSearchTerms are the terms/phrases to search for using BM25 full-text search
	FullTextSearchTerms []string `json:"full_text_search_terms,omitempty"`

	// Filters are structured field:value pairs to filter results
	Filters map[string]interface{} `json:"filters,omitempty"`

	// UseHybrid indicates whether to use hybrid search (semantic + full-text)
	UseHybrid bool `json:"use_hybrid"`

	// Limit is the maximum number of results to return
	Limit int `json:"limit"`

	// FieldsToReturn specifies which fields to include in results (empty = all fields)
	FieldsToReturn []string `json:"fields_to_return,omitempty"`

	// OrderByField specifies the field to order by (optional)
	OrderByField string `json:"order_by_field,omitempty"`

	// OrderDescending indicates whether to order descending (true) or ascending (false)
	OrderDescending bool `json:"order_descending,omitempty"`
}

// TableCapabilities represents what query types a table supports
type TableCapabilities struct {
	TableName        string
	HasEmbeddingIdx  bool
	EmbeddingIndexes []string
	HasBleveIdx      bool
	BleveIndexes     []string
	SearchableFields []string
	FilterableFields []string
}

// GenerateQueries uses an LLM to generate appropriate Antfly queries based on the user's query,
// classification result, and available table schemas.
func GenerateQueries(
	ctx context.Context,
	g *genkit.Genkit,
	model ai.Model,
	userQuery string,
	classification *ClassificationResult,
	tables map[string]antfly.TableStatus,
) ([]antfly.QueryRequest, error) {
	if userQuery == "" {
		return nil, fmt.Errorf("user query cannot be empty")
	}
	if classification == nil {
		return nil, fmt.Errorf("classification cannot be nil")
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("no tables provided")
	}

	// Analyze table capabilities
	capabilities := analyzeTableCapabilities(tables)

	// Build table analysis string for prompt
	tableAnalysis := buildTableAnalysis(capabilities)

	// Build the prompt text
	systemPrompt := buildSystemPrompt()
	userPromptText := buildUserPromptText(userQuery, classification, tableAnalysis)

	// Build generation options
	genOpts := []ai.GenerateOption{
		ai.WithModel(model),
		ai.WithSystem(systemPrompt),
		ai.WithPrompt("%s", userPromptText),
	}

	// Execute generation to get structured query plan
	plan, _, err := genkit.GenerateData[QueryPlan](ctx, g, genOpts...)
	if err != nil {
		return nil, fmt.Errorf("generating query plan: %w", err)
	}

	// Convert QueryPlan to QueryRequests
	queries, err := buildQueryRequests(plan, capabilities)
	if err != nil {
		return nil, fmt.Errorf("building query requests: %w", err)
	}

	return queries, nil
}

// analyzeTableCapabilities examines each table's indexes and schema to determine capabilities
func analyzeTableCapabilities(tables map[string]antfly.TableStatus) map[string]*TableCapabilities {
	capabilities := make(map[string]*TableCapabilities)

	for tableName, tableStatus := range tables {
		cap := &TableCapabilities{
			TableName:        tableName,
			EmbeddingIndexes: []string{},
			BleveIndexes:     []string{},
			SearchableFields: []string{},
			FilterableFields: []string{},
		}

		// Analyze indexes
		for indexName, indexConfig := range tableStatus.Indexes {
			switch indexConfig.Type {
			case "embeddingindex":
				cap.HasEmbeddingIdx = true
				cap.EmbeddingIndexes = append(cap.EmbeddingIndexes, indexName)
			case "bleve":
				cap.HasBleveIdx = true
				cap.BleveIndexes = append(cap.BleveIndexes, indexName)
			}
		}

		// Analyze schema to extract searchable/filterable fields
		if tableStatus.Schema != nil {
			for typeName, docSchema := range tableStatus.Schema.DocumentSchemas {
				if docSchema.Schema != nil {
					// Extract properties from JSON schema
					if props, ok := docSchema.Schema["properties"].(map[string]interface{}); ok {
						for fieldName := range props {
							// All fields are potentially filterable
							cap.FilterableFields = append(cap.FilterableFields, fieldName)
							// Text fields are searchable
							cap.SearchableFields = append(cap.SearchableFields, fieldName)
						}
					}
				}
				_ = typeName // Use typeName if needed for more sophisticated analysis
			}
		}

		capabilities[tableName] = cap
	}

	return capabilities
}

// buildTableAnalysis creates a human-readable analysis of table capabilities for the LLM
func buildTableAnalysis(capabilities map[string]*TableCapabilities) string {
	var sb strings.Builder

	for _, cap := range capabilities {
		sb.WriteString(fmt.Sprintf("Table: %s\n", cap.TableName))

		if cap.HasEmbeddingIdx {
			sb.WriteString(fmt.Sprintf("  - Supports semantic search (embedding indexes: %s)\n",
				strings.Join(cap.EmbeddingIndexes, ", ")))
		}

		if cap.HasBleveIdx {
			sb.WriteString(fmt.Sprintf("  - Supports full-text search (bleve indexes: %s)\n",
				strings.Join(cap.BleveIndexes, ", ")))
		}

		if len(cap.SearchableFields) > 0 {
			sb.WriteString(fmt.Sprintf("  - Searchable fields: %s\n",
				strings.Join(cap.SearchableFields, ", ")))
		}

		if len(cap.FilterableFields) > 0 {
			sb.WriteString(fmt.Sprintf("  - Filterable fields: %s\n",
				strings.Join(cap.FilterableFields, ", ")))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// buildSystemPrompt creates the system prompt for the query generation LLM
func buildSystemPrompt() string {
	return `You are an expert database query optimizer for Antfly, a hybrid vector and full-text search system.

Your task is to analyze a user's query and generate optimized queries for the available tables.

Query Types:
- SEMANTIC SEARCH: Use vector embeddings for semantic similarity (requires embedding indexes)
- FULL-TEXT SEARCH: Use BM25 for keyword matching (requires bleve indexes)
- FILTERS: Use structured field:value filters for precise filtering
- HYBRID: Combine semantic and full-text with Reciprocal Rank Fusion (RRF)

Classification Types:
- "question": User wants a specific answer. Prioritize precision. Use filters + semantic search.
- "search": User wants to explore/discover. Prioritize recall. Use hybrid search.

Guidelines:
1. Only query tables that are relevant to the user's query
2. Use semantic search when available for natural language queries
3. Use full-text search for keyword-heavy queries
4. Use hybrid search (semantic + full-text) for "search" classification when both indexes available
5. Extract entities from the query to create filters (e.g., dates, IDs, categories)
6. For "question" classification: prefer smaller limits (5-10), use filters for precision
7. For "search" classification: prefer larger limits (20-50), use hybrid for recall
8. Return fields relevant to the query (or all fields if unsure)
9. Order results by relevance (let the search algorithm handle it) unless a specific field is mentioned`
}

// buildUserPromptText creates the user prompt text for the query generation LLM
func buildUserPromptText(userQuery string, classification *ClassificationResult, tableAnalysis string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("USER QUERY: \"%s\"\n\n", userQuery))
	sb.WriteString("CLASSIFICATION:\n")
	sb.WriteString(fmt.Sprintf("- Route Type: %s\n", classification.RouteType))
	sb.WriteString(fmt.Sprintf("- Keywords: %s\n", strings.Join(classification.Keywords, ", ")))
	sb.WriteString(fmt.Sprintf("- Confidence: %.2f\n\n", classification.Confidence))
	sb.WriteString("AVAILABLE TABLES AND CAPABILITIES:\n")
	sb.WriteString(tableAnalysis)
	sb.WriteString("\nTASK:\n")
	sb.WriteString("Generate an optimal query plan for this user query. For each relevant table, specify:\n")
	sb.WriteString("1. Whether to use semantic search (and what text to search for)\n")
	sb.WriteString("2. Whether to use full-text search (and what terms to search for)\n")
	sb.WriteString("3. Any filters to apply (extracted from the query)\n")
	sb.WriteString("4. Whether to use hybrid search (combine semantic + full-text)\n")
	sb.WriteString("5. Appropriate limit for results\n")
	sb.WriteString("6. Fields to return (leave empty for all fields)\n")
	sb.WriteString("7. Any ordering preferences\n\n")

	if classification.RouteType == RouteQuestion {
		sb.WriteString("This is a QUESTION query - prioritize precision and use filters where possible.\n\n")
	} else if classification.RouteType == RouteSearch {
		sb.WriteString("This is a SEARCH query - prioritize recall and use hybrid search where available.\n\n")
	}

	sb.WriteString("Provide your response as a structured QueryPlan.")

	return sb.String()
}

// buildQueryRequests converts the LLM's QueryPlan into actual Antfly QueryRequests
func buildQueryRequests(plan *QueryPlan, capabilities map[string]*TableCapabilities) ([]antfly.QueryRequest, error) {
	var requests []antfly.QueryRequest

	for _, tablePlan := range plan.TableQueries {
		cap, ok := capabilities[tablePlan.TableName]
		if !ok {
			return nil, fmt.Errorf("unknown table in plan: %s", tablePlan.TableName)
		}

		req := antfly.QueryRequest{
			Table: tablePlan.TableName,
			Limit: tablePlan.Limit,
		}

		// Set default limit if not specified
		if req.Limit == 0 {
			req.Limit = 10
		}

		// Add semantic search if requested and available
		if tablePlan.SemanticSearchText != "" && cap.HasEmbeddingIdx {
			req.SemanticSearch = tablePlan.SemanticSearchText
			req.Indexes = cap.EmbeddingIndexes
		}

		// Add full-text search if requested and available
		if len(tablePlan.FullTextSearchTerms) > 0 && cap.HasBleveIdx {
			bleveQuery := buildBleveQuery(tablePlan.FullTextSearchTerms, cap.SearchableFields)
			req.FullTextSearch = &bleveQuery
		}

		// Add filters if requested
		if len(tablePlan.Filters) > 0 {
			filterQuery := buildFilterQuery(tablePlan.Filters)
			req.FilterQuery = &filterQuery
		}

		// Set merge strategy for hybrid queries
		if tablePlan.UseHybrid && req.SemanticSearch != "" && req.FullTextSearch != nil {
			req.MergeStrategy = "rrf"
		}

		// Set fields to return
		if len(tablePlan.FieldsToReturn) > 0 {
			req.Fields = tablePlan.FieldsToReturn
		}

		// Set ordering if specified
		if tablePlan.OrderByField != "" {
			req.OrderBy = map[string]bool{
				tablePlan.OrderByField: tablePlan.OrderDescending,
			}
		}

		requests = append(requests, req)
	}

	return requests, nil
}

// buildBleveQuery constructs a Bleve query from search terms
func buildBleveQuery(terms []string, searchableFields []string) query.Query {
	if len(terms) == 0 {
		return query.Query{}
	}

	// If single term, use a simple match query
	if len(terms) == 1 {
		matchQuery := query.MatchQuery{
			Match: terms[0],
		}
		return matchQuery.ToQuery()
	}

	// Multiple terms: use disjunction query (OR)
	var queries []query.Query
	for _, term := range terms {
		matchQuery := query.MatchQuery{
			Match: term,
		}
		queries = append(queries, matchQuery.ToQuery())
	}

	disjunctionQuery := query.DisjunctionQuery{
		Disjuncts: queries,
	}

	return disjunctionQuery.ToQuery()
}

// buildFilterQuery constructs a Bleve filter query from field:value pairs
func buildFilterQuery(filters map[string]interface{}) query.Query {
	if len(filters) == 0 {
		return query.Query{}
	}

	var queries []query.Query
	for field, value := range filters {
		// Use term query for exact matching
		termQuery := query.TermQuery{
			Term:  fmt.Sprintf("%v", value),
			Field: field,
		}
		queries = append(queries, termQuery.ToQuery())
	}

	// If single filter, return it directly
	if len(queries) == 1 {
		return queries[0]
	}

	// Multiple filters: use conjunction query (AND)
	conjunctionQuery := query.ConjunctionQuery{
		Conjuncts: queries,
	}

	return conjunctionQuery.ToQuery()
}
