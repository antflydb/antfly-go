package docsaf

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// OpenAPIProcessor processes OpenAPI specification content using libopenapi.
// It extracts API info, paths, and schemas as separate document sections.
type OpenAPIProcessor struct{}

// CanProcess returns true for .yaml, .yml, and .json files.
// Note: The content will only be processed if it's a valid OpenAPI v3 specification.
func (op *OpenAPIProcessor) CanProcess(contentType, path string) bool {
	if strings.Contains(contentType, "application/x-yaml") ||
		strings.Contains(contentType, "application/yaml") ||
		strings.Contains(contentType, "text/yaml") ||
		strings.Contains(contentType, "application/json") {
		return true
	}
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".json")
}

// Process processes OpenAPI specification content and returns document sections.
// Returns an error if the content is not a valid OpenAPI v3 specification.
func (op *OpenAPIProcessor) Process(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error) {
	// Try to parse as OpenAPI
	doc, err := libopenapi.NewDocument(content)
	if err != nil {
		return nil, fmt.Errorf("not a valid OpenAPI document: %w", err)
	}

	var sections []DocumentSection

	v3Model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAPI v3 model: %w", err)
	}

	if v3Model != nil {
		sections = append(sections, op.extractV3Sections(&v3Model.Model, path, sourceURL, baseURL)...)
	}

	return sections, nil
}

// extractV3Sections extracts document sections from an OpenAPI v3 document.
func (op *OpenAPIProcessor) extractV3Sections(model *v3.Document, path, sourceURL, baseURL string) []DocumentSection {
	var sections []DocumentSection

	// Extract API info as a section
	if model.Info != nil {
		infoJSON, _ := json.MarshalIndent(model.Info, "", "  ")

		url := ""
		if baseURL != "" {
			url = baseURL + "/" + path + "#info"
		}

		metadata := map[string]any{
			"openapi_version": model.Version,
			"api_version":     model.Info.Version,
			"api_title":       model.Info.Title,
		}
		if sourceURL != "" {
			metadata["source_url"] = sourceURL
		}

		sections = append(sections, DocumentSection{
			ID:       generateID(path, "info"),
			FilePath: path,
			Title:    fmt.Sprintf("%s (Info)", model.Info.Title),
			Content:  string(infoJSON),
			Type:     "openapi_info",
			URL:      url,
			Metadata: metadata,
		})
	}

	// Extract paths as individual sections
	if model.Paths != nil && model.Paths.PathItems != nil {
		for pathPair := model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
			pathKey := pathPair.Key()
			pathItem := pathPair.Value()

			operations := extractOperations(pathItem)
			for method, operation := range operations {
				opJSON, _ := json.MarshalIndent(operation, "", "  ")
				operationID := operation.OperationId
				if operationID == "" {
					operationID = fmt.Sprintf("%s_%s", method, strings.ReplaceAll(pathKey, "/", "_"))
				}

				url := ""
				if baseURL != "" {
					slug := strings.ToLower(method) + "-" + pathKey
					url = baseURL + "/" + path + "#" + slug
				}

				metadata := map[string]any{
					"http_method":  method,
					"path":         pathKey,
					"operation_id": operationID,
					"summary":      operation.Summary,
					"description":  operation.Description,
					"tags":         operation.Tags,
				}
				if sourceURL != "" {
					metadata["source_url"] = sourceURL
				}

				sections = append(sections, DocumentSection{
					ID:       generateID(path, fmt.Sprintf("path_%s_%s", method, pathKey)),
					FilePath: path,
					Title:    fmt.Sprintf("%s %s", strings.ToUpper(method), pathKey),
					Content:  string(opJSON),
					Type:     "openapi_path",
					URL:      url,
					Metadata: metadata,
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

			url := ""
			if baseURL != "" {
				slug := "schema-" + strings.ToLower(schemaName)
				url = baseURL + "/" + path + "#" + slug
			}

			metadata := map[string]any{
				"schema_name": schemaName,
				"schema_type": schema.Type,
				"description": schema.Description,
			}
			if sourceURL != "" {
				metadata["source_url"] = sourceURL
			}

			sections = append(sections, DocumentSection{
				ID:       generateID(path, fmt.Sprintf("schema_%s", schemaName)),
				FilePath: path,
				Title:    fmt.Sprintf("Schema: %s", schemaName),
				Content:  string(schemaJSON),
				Type:     "openapi_schema",
				URL:      url,
				Metadata: metadata,
			})
		}
	}

	return sections
}

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
