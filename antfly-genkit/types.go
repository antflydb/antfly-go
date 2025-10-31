package antfly

// ClassificationResult represents the result of classifying a user query
type ClassificationResult struct {
	// RouteType indicates whether this is a "question" or "search" query
	RouteType string `json:"route_type"`

	// Keywords extracted from the query for search optimization
	Keywords []string `json:"keywords"`

	// Confidence score from 0.0 to 1.0 indicating classification confidence
	Confidence float32 `json:"confidence"`
}

// RouteType constants
const (
	RouteQuestion = "question"
	RouteSearch   = "search"
)
