package docsaf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// OpenAPIProcessor processes OpenAPI specification files using libopenapi.
// It extracts API info, paths, and schemas as separate document sections.
type OpenAPIProcessor struct{}

// CanProcess returns true for .yaml, .yml, and .json files.
// Note: The file will only be processed if it's a valid OpenAPI v3 specification.
func (op *OpenAPIProcessor) CanProcess(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".json")
}

// ProcessFile processes an OpenAPI specification file and returns document sections.
// Returns an error if the file is not a valid OpenAPI v3 specification.
func (op *OpenAPIProcessor) ProcessFile(filePath, baseDir string) ([]DocumentSection, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	relPath, _ := filepath.Rel(baseDir, filePath)

	// Try to parse as OpenAPI
	doc, err := libopenapi.NewDocument(content)
	if err != nil {
		// Not a valid OpenAPI doc, skip
		return nil, fmt.Errorf("not a valid OpenAPI document: %w", err)
	}

	var sections []DocumentSection

	// Parse v3 document
	v3Model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAPI v3 model: %w", err)
	}

	if v3Model != nil {
		sections = append(sections, op.extractV3Sections(&v3Model.Model, relPath)...)
	}

	return sections, nil
}

// extractV3Sections extracts document sections from an OpenAPI v3 document.
// It creates separate sections for:
// - API info (openapi_info)
// - Each path operation (openapi_path)
// - Each component schema (openapi_schema)
func (op *OpenAPIProcessor) extractV3Sections(model *v3.Document, relPath string) []DocumentSection {
	var sections []DocumentSection

	// Extract API info as a section
	if model.Info != nil {
		infoJSON, _ := json.MarshalIndent(model.Info, "", "  ")
		sections = append(sections, DocumentSection{
			ID:       generateID(relPath, "info"),
			FilePath: relPath,
			Title:    fmt.Sprintf("%s (Info)", model.Info.Title),
			Content:  string(infoJSON),
			Type:     "openapi_info",
			Metadata: map[string]any{
				"openapi_version": model.Version,
				"api_version":     model.Info.Version,
				"api_title":       model.Info.Title,
			},
			CreatedAt: time.Now(),
		})
	}

	// Extract paths as individual sections
	if model.Paths != nil && model.Paths.PathItems != nil {
		for pathPair := model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
			pathKey := pathPair.Key()
			pathItem := pathPair.Value()

			// Extract each operation (GET, POST, etc.)
			operations := extractOperations(pathItem)
			for method, operation := range operations {
				opJSON, _ := json.MarshalIndent(operation, "", "  ")
				operationID := operation.OperationId
				if operationID == "" {
					operationID = fmt.Sprintf("%s_%s", method, strings.ReplaceAll(pathKey, "/", "_"))
				}

				sections = append(sections, DocumentSection{
					ID:       generateID(relPath, fmt.Sprintf("path_%s_%s", method, pathKey)),
					FilePath: relPath,
					Title:    fmt.Sprintf("%s %s", strings.ToUpper(method), pathKey),
					Content:  string(opJSON),
					Type:     "openapi_path",
					Metadata: map[string]any{
						"http_method":  method,
						"path":         pathKey,
						"operation_id": operationID,
						"summary":      operation.Summary,
						"description":  operation.Description,
						"tags":         operation.Tags,
					},
					CreatedAt: time.Now(),
				})
			}
		}
	}

	// Extract schemas as individual sections
	if model.Components != nil && model.Components.Schemas != nil {
		for schemaPair := model.Components.Schemas.First(); schemaPair != nil; schemaPair = schemaPair.Next() {
			schemaName := schemaPair.Key()
			schemaProxy := schemaPair.Value()
			schema := schemaProxy.Schema()

			schemaJSON, _ := json.MarshalIndent(schema, "", "  ")

			sections = append(sections, DocumentSection{
				ID:       generateID(relPath, fmt.Sprintf("schema_%s", schemaName)),
				FilePath: relPath,
				Title:    fmt.Sprintf("Schema: %s", schemaName),
				Content:  string(schemaJSON),
				Type:     "openapi_schema",
				Metadata: map[string]any{
					"schema_name": schemaName,
					"schema_type": schema.Type,
					"description": schema.Description,
				},
				CreatedAt: time.Now(),
			})
		}
	}

	return sections
}

// extractOperations extracts all HTTP operations from a path item.
// Returns a map of method name (lowercase) to operation.
func extractOperations(pathItem *v3.PathItem) map[string]*v3.Operation {
	ops := make(map[string]*v3.Operation)
	if pathItem.Get != nil {
		ops["get"] = pathItem.Get
	}
	if pathItem.Post != nil {
		ops["post"] = pathItem.Post
	}
	if pathItem.Put != nil {
		ops["put"] = pathItem.Put
	}
	if pathItem.Delete != nil {
		ops["delete"] = pathItem.Delete
	}
	if pathItem.Patch != nil {
		ops["patch"] = pathItem.Patch
	}
	if pathItem.Options != nil {
		ops["options"] = pathItem.Options
	}
	if pathItem.Head != nil {
		ops["head"] = pathItem.Head
	}
	return ops
}
