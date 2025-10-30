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
package oapi

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/kaptinlin/jsonschema"
)

// ValidationError represents a single validation error from document schema validation
type ValidationError struct {
	Field   string
	Message string
}

// ValidationResult contains the results of document validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// Validate validates a document against the DocumentSchema.
// It compiles the JSON schema and validates the document structure, types, and constraints.
// Returns a ValidationResult containing the validation status and any errors.
//
// Example usage:
//
//	schema := oapi.DocumentSchema{
//	    Schema: map[string]interface{}{
//	        "type": "object",
//	        "properties": map[string]interface{}{
//	            "name": map[string]interface{}{"type": "string"},
//	            "age":  map[string]interface{}{"type": "number"},
//	        },
//	        "required": []string{"name"},
//	    },
//	}
//
//	doc := map[string]interface{}{
//	    "name": "Alice",
//	    "age":  30,
//	}
//
//	result, err := schema.Validate(doc)
//	if err != nil {
//	    // Handle compilation error
//	}
//	if !result.Valid {
//	    // Handle validation errors
//	    for _, e := range result.Errors {
//	        fmt.Printf("%s: %s\n", e.Field, e.Message)
//	    }
//	}
func (d *DocumentSchema) Validate(document any) (*ValidationResult, error) {
	// If schema is empty, consider document valid
	if len(d.Schema) == 0 {
		return &ValidationResult{Valid: true}, nil
	}

	// Create a new compiler with sonic JSON encoder/decoder for consistency
	compiler := jsonschema.NewCompiler()
	compiler.WithDecoderJSON(sonic.Unmarshal)
	compiler.WithEncoderJSON(sonic.Marshal)

	// Marshal the schema to bytes for compilation
	schemaBytes, err := sonic.Marshal(d.Schema)
	if err != nil {
		return nil, fmt.Errorf("marshalling schema: %w", err)
	}

	// Compile the schema
	compiledSchema, err := compiler.Compile(schemaBytes)
	if err != nil {
		return nil, fmt.Errorf("compiling schema: %w", err)
	}

	// Convert document to map for validation
	var docMap map[string]any
	switch v := document.(type) {
	case map[string]any:
		docMap = v
	default:
		// If not already a map, marshal and unmarshal to convert
		docBytes, err := sonic.Marshal(document)
		if err != nil {
			return nil, fmt.Errorf("marshalling document: %w", err)
		}
		if err := sonic.Unmarshal(docBytes, &docMap); err != nil {
			return nil, fmt.Errorf("unmarshalling document to map: %w", err)
		}
	}

	// Validate the document
	result := compiledSchema.ValidateMap(docMap)

	// Build validation result
	validationResult := &ValidationResult{
		Valid: result.IsValid(),
	}

	if !result.IsValid() {
		validationResult.Errors = make([]ValidationError, 0, len(result.Errors))
		for field, e := range result.Errors {
			validationResult.Errors = append(validationResult.Errors, ValidationError{
				Field:   field,
				Message: e.Message,
			})
		}
	}

	return validationResult, nil
}
