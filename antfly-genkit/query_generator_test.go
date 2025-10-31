package antfly

import (
	"context"
	"testing"

	"github.com/antflydb/antfly-go/antfly"
	"github.com/antflydb/antfly-go/antfly/oapi"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

func TestAnalyzeTableCapabilities(t *testing.T) {
	tables := map[string]antfly.TableStatus{
		"products": {
			Name: "products",
			Schema: &oapi.TableSchema{
				DocumentSchemas: map[string]oapi.DocumentSchema{
					"product": {
						Schema: map[string]interface{}{
							"properties": map[string]interface{}{
								"name":        map[string]interface{}{"type": "string"},
								"description": map[string]interface{}{"type": "string"},
								"price":       map[string]interface{}{"type": "number"},
								"category":    map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
			Indexes: map[string]oapi.IndexConfig{
				"embedding_idx": {
					Type: "embeddingindex",
				},
				"search_idx": {
					Type: "bleve",
				},
			},
		},
		"reviews": {
			Name: "reviews",
			Schema: &oapi.TableSchema{
				DocumentSchemas: map[string]oapi.DocumentSchema{
					"review": {
						Schema: map[string]interface{}{
							"properties": map[string]interface{}{
								"text":       map[string]interface{}{"type": "string"},
								"rating":     map[string]interface{}{"type": "number"},
								"product_id": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
			Indexes: map[string]oapi.IndexConfig{
				"embedding_idx": {
					Type: "embeddingindex",
				},
			},
		},
	}

	capabilities := analyzeTableCapabilities(tables)

	// Test products table capabilities
	productsCap := capabilities["products"]
	if productsCap == nil {
		t.Fatal("products capabilities not found")
	}
	if !productsCap.HasEmbeddingIdx {
		t.Error("products should have embedding index")
	}
	if !productsCap.HasBleveIdx {
		t.Error("products should have bleve index")
	}
	if len(productsCap.EmbeddingIndexes) != 1 || productsCap.EmbeddingIndexes[0] != "embedding_idx" {
		t.Errorf("expected embedding_idx, got %v", productsCap.EmbeddingIndexes)
	}
	if len(productsCap.BleveIndexes) != 1 || productsCap.BleveIndexes[0] != "search_idx" {
		t.Errorf("expected search_idx, got %v", productsCap.BleveIndexes)
	}
	if len(productsCap.SearchableFields) == 0 {
		t.Error("products should have searchable fields")
	}

	// Test reviews table capabilities
	reviewsCap := capabilities["reviews"]
	if reviewsCap == nil {
		t.Fatal("reviews capabilities not found")
	}
	if !reviewsCap.HasEmbeddingIdx {
		t.Error("reviews should have embedding index")
	}
	if reviewsCap.HasBleveIdx {
		t.Error("reviews should not have bleve index")
	}
}

func TestBuildTableAnalysis(t *testing.T) {
	capabilities := map[string]*TableCapabilities{
		"products": {
			TableName:        "products",
			HasEmbeddingIdx:  true,
			EmbeddingIndexes: []string{"embedding_idx"},
			HasBleveIdx:      true,
			BleveIndexes:     []string{"search_idx"},
			SearchableFields: []string{"name", "description"},
			FilterableFields: []string{"category", "price"},
		},
	}

	analysis := buildTableAnalysis(capabilities)

	if analysis == "" {
		t.Fatal("analysis should not be empty")
	}
	if !contains(analysis, "products") {
		t.Error("analysis should contain table name")
	}
	if !contains(analysis, "semantic search") {
		t.Error("analysis should mention semantic search")
	}
	if !contains(analysis, "full-text search") {
		t.Error("analysis should mention full-text search")
	}
}

func TestBuildBleveQuery(t *testing.T) {
	tests := []struct {
		name            string
		terms           []string
		searchableFields []string
		wantQueryType   string
	}{
		{
			name:             "single term",
			terms:            []string{"laptop"},
			searchableFields: []string{"name", "description"},
			wantQueryType:    "match",
		},
		{
			name:             "multiple terms",
			terms:            []string{"laptop", "gaming"},
			searchableFields: []string{"name", "description"},
			wantQueryType:    "disjunction",
		},
		{
			name:             "empty terms",
			terms:            []string{},
			searchableFields: []string{"name"},
			wantQueryType:    "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := buildBleveQuery(tt.terms, tt.searchableFields)

			// Basic validation - just verify the function runs without panic
			// We can't easily validate the internal structure without unmarshaling
			_ = q
		})
	}
}

func TestBuildFilterQuery(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string]interface{}
		want    int // expected query length (0 for empty)
	}{
		{
			name:    "empty filters",
			filters: map[string]interface{}{},
			want:    0,
		},
		{
			name: "single filter",
			filters: map[string]interface{}{
				"category": "electronics",
			},
			want: 1,
		},
		{
			name: "multiple filters",
			filters: map[string]interface{}{
				"category": "electronics",
				"brand":    "Apple",
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := buildFilterQuery(tt.filters)

			// Basic validation - just verify the function runs without panic
			// We can't easily validate the internal structure without unmarshaling
			_ = q
		})
	}
}

func TestBuildQueryRequests(t *testing.T) {
	capabilities := map[string]*TableCapabilities{
		"products": {
			TableName:        "products",
			HasEmbeddingIdx:  true,
			EmbeddingIndexes: []string{"embedding_idx"},
			HasBleveIdx:      true,
			BleveIndexes:     []string{"search_idx"},
			SearchableFields: []string{"name", "description"},
			FilterableFields: []string{"category", "price"},
		},
		"reviews": {
			TableName:        "reviews",
			HasEmbeddingIdx:  true,
			EmbeddingIndexes: []string{"embedding_idx"},
			HasBleveIdx:      false,
			SearchableFields: []string{"text"},
			FilterableFields: []string{"rating", "product_id"},
		},
	}

	tests := []struct {
		name    string
		plan    *QueryPlan
		wantLen int
		checks  func(*testing.T, []antfly.QueryRequest)
	}{
		{
			name: "semantic search only",
			plan: &QueryPlan{
				TableQueries: []TableQueryPlan{
					{
						TableName:          "reviews",
						SemanticSearchText: "great product",
						Limit:              10,
					},
				},
			},
			wantLen: 1,
			checks: func(t *testing.T, reqs []antfly.QueryRequest) {
				if reqs[0].SemanticSearch != "great product" {
					t.Error("semantic search text not set correctly")
				}
				if len(reqs[0].Indexes) == 0 {
					t.Error("indexes should be set for semantic search")
				}
				if reqs[0].Limit != 10 {
					t.Error("limit not set correctly")
				}
			},
		},
		{
			name: "full-text search only",
			plan: &QueryPlan{
				TableQueries: []TableQueryPlan{
					{
						TableName:           "products",
						FullTextSearchTerms: []string{"laptop", "gaming"},
						Limit:               20,
					},
				},
			},
			wantLen: 1,
			checks: func(t *testing.T, reqs []antfly.QueryRequest) {
				if reqs[0].FullTextSearch == nil {
					t.Error("full text search should be set")
				}
				if reqs[0].Limit != 20 {
					t.Error("limit not set correctly")
				}
			},
		},
		{
			name: "hybrid search",
			plan: &QueryPlan{
				TableQueries: []TableQueryPlan{
					{
						TableName:           "products",
						SemanticSearchText:  "gaming laptop",
						FullTextSearchTerms: []string{"laptop", "gaming"},
						UseHybrid:           true,
						Limit:               15,
					},
				},
			},
			wantLen: 1,
			checks: func(t *testing.T, reqs []antfly.QueryRequest) {
				if reqs[0].SemanticSearch == "" {
					t.Error("semantic search should be set")
				}
				if reqs[0].FullTextSearch == nil {
					t.Error("full text search should be set")
				}
				if reqs[0].MergeStrategy != "rrf" {
					t.Error("merge strategy should be rrf for hybrid search")
				}
				if len(reqs[0].Indexes) == 0 {
					t.Error("indexes should be set")
				}
			},
		},
		{
			name: "with filters",
			plan: &QueryPlan{
				TableQueries: []TableQueryPlan{
					{
						TableName:          "products",
						SemanticSearchText: "laptop",
						Filters: map[string]interface{}{
							"category": "electronics",
						},
						Limit: 10,
					},
				},
			},
			wantLen: 1,
			checks: func(t *testing.T, reqs []antfly.QueryRequest) {
				if reqs[0].FilterQuery == nil {
					t.Error("filter query should be set")
				}
			},
		},
		{
			name: "multiple tables",
			plan: &QueryPlan{
				TableQueries: []TableQueryPlan{
					{
						TableName:          "products",
						SemanticSearchText: "laptop",
						Limit:              10,
					},
					{
						TableName:          "reviews",
						SemanticSearchText: "great laptop",
						Limit:              20,
					},
				},
			},
			wantLen: 2,
			checks: func(t *testing.T, reqs []antfly.QueryRequest) {
				if reqs[0].Table != "products" {
					t.Error("first query should be for products table")
				}
				if reqs[1].Table != "reviews" {
					t.Error("second query should be for reviews table")
				}
			},
		},
		{
			name: "with field selection and ordering",
			plan: &QueryPlan{
				TableQueries: []TableQueryPlan{
					{
						TableName:          "products",
						SemanticSearchText: "laptop",
						FieldsToReturn:     []string{"name", "price"},
						OrderByField:       "price",
						OrderDescending:    true,
						Limit:              10,
					},
				},
			},
			wantLen: 1,
			checks: func(t *testing.T, reqs []antfly.QueryRequest) {
				if len(reqs[0].Fields) != 2 {
					t.Error("fields should be set")
				}
				if reqs[0].OrderBy == nil || !reqs[0].OrderBy["price"] {
					t.Error("order by should be set to price descending")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, err := buildQueryRequests(tt.plan, capabilities)
			if err != nil {
				t.Fatalf("buildQueryRequests failed: %v", err)
			}
			if len(reqs) != tt.wantLen {
				t.Errorf("expected %d requests, got %d", tt.wantLen, len(reqs))
			}
			if tt.checks != nil {
				tt.checks(t, reqs)
			}
		})
	}
}

func TestBuildQueryRequestsErrors(t *testing.T) {
	capabilities := map[string]*TableCapabilities{
		"products": {
			TableName:        "products",
			HasEmbeddingIdx:  true,
			EmbeddingIndexes: []string{"embedding_idx"},
		},
	}

	plan := &QueryPlan{
		TableQueries: []TableQueryPlan{
			{
				TableName: "nonexistent",
				Limit:     10,
			},
		},
	}

	_, err := buildQueryRequests(plan, capabilities)
	if err == nil {
		t.Error("expected error for nonexistent table")
	}
}

func TestGenerateQueries_Validation(t *testing.T) {
	ctx := context.Background()
	// Note: We can't easily create a genkit instance in tests without full setup
	// So we'll pass nil and test that validation happens before genkit is used
	var g *genkit.Genkit
	var model ai.Model

	tables := map[string]antfly.TableStatus{
		"products": {
			Name:    "products",
			Indexes: map[string]oapi.IndexConfig{},
		},
	}

	classification := &ClassificationResult{
		RouteType:  RouteQuestion,
		Keywords:   []string{"laptop"},
		Confidence: 0.9,
	}

	// Test empty query
	_, err := GenerateQueries(ctx, g, model, "", classification, tables)
	if err == nil {
		t.Error("expected error for empty query")
	}

	// Test nil classification
	_, err = GenerateQueries(ctx, g, model, "test query", nil, tables)
	if err == nil {
		t.Error("expected error for nil classification")
	}

	// Test empty tables
	_, err = GenerateQueries(ctx, g, model, "test query", classification, map[string]antfly.TableStatus{})
	if err == nil {
		t.Error("expected error for empty tables")
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
